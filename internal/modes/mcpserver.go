package modes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sam-hartman-mistral/annas-mcp-server/internal/anna"
	"github.com/sam-hartman-mistral/annas-mcp-server/internal/logger"
	"github.com/sam-hartman-mistral/annas-mcp-server/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// downloadTracker tracks recent downloads to prevent duplicates
var (
	downloadTracker   = make(map[string]time.Time)
	downloadTrackerMu sync.RWMutex
	downloadCooldown  = 30 * time.Second // Prevent same hash download within 30 seconds
)

// fileSizeRegex is precompiled for extracting file size from error messages
var fileSizeRegex = regexp.MustCompile(`(\d+)\s+bytes\s+\(([\d.]+)\s+MB\)`)

func init() {
	// Start background goroutine to clean up old entries from downloadTracker
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cleanupDownloadTracker()
		}
	}()
}

// cleanupDownloadTracker removes entries older than the cooldown period
func cleanupDownloadTracker() {
	downloadTrackerMu.Lock()
	defer downloadTrackerMu.Unlock()

	cutoff := time.Now().Add(-downloadCooldown * 2) // Keep entries for 2x cooldown
	for key, timestamp := range downloadTracker {
		if timestamp.Before(cutoff) {
			delete(downloadTracker, key)
		}
	}
}

// Tool definitions - single source of truth for all tool metadata
const (
	// Tool names
	ToolNameSearch   = "search"
	ToolNameDownload = "download"

	// Tool descriptions
	SearchToolDescription = "Search for books on Anna's Archive. Returns a list of books with metadata including title, authors, format (epub, mobi, pdf, etc.), language, size, and MD5 hash. Results are sorted by format preference (EPUB first by default, as EPUBs are best for Kindle: small file size, reflowable text, adjustable fonts). Use the hash from search results to download a specific book."

	DownloadToolDescription = "Download a book and send it to a Kindle email. The book is downloaded from Anna's Archive, saved locally as a backup (if ANNAS_DOWNLOAD_PATH is set), and then emailed to the specified Kindle email address. If no kindle_email is provided, uses the default configured KINDLE_EMAIL. Requires ANNAS_SECRET_KEY for API access and email configuration (SMTP settings) for Kindle delivery. If email is not configured, falls back to local download only. Note: Kindle email only accepts PDF, EPUB, DOC, DOCX, HTML, RTF, and TXT formats - MOBI files will be rejected."

	// Parameter descriptions
	SearchTermDesc       = "Search term - can be book title, author name, or any keywords"
	SearchFormatDesc     = "Optional: Preferred format (epub, pdf, mobi). Defaults to 'epub' for Kindle compatibility. EPUBs are recommended as they are small (0.5-5MB), reflowable, and work best on Kindle devices."
	DownloadHashDesc     = "MD5 hash of the book to download - get this from the search results"
	DownloadTitleDesc    = "Book title - used for the filename and email subject. Get this from search results."
	DownloadFormatDesc   = "Book format (epub, mobi, pdf, azw3, etc.) - get this from search results. The actual format will be detected from the downloaded file, but this helps with initial filename."
	DownloadKindleDesc   = "Optional: Kindle email address to send the book to. If not specified, uses the default KINDLE_EMAIL from server configuration."
)

// addToolsToServer adds the standard tools to an MCP server instance
func addToolsToServer(server *mcp.Server) {
	server.AddTools(
		mcp.NewServerTool(ToolNameSearch, SearchToolDescription, SearchTool, mcp.Input(
			mcp.Property("term", mcp.Description(SearchTermDesc)),
			mcp.Property("format", mcp.Description(SearchFormatDesc)),
		)),
		mcp.NewServerTool(ToolNameDownload, DownloadToolDescription, DownloadTool, mcp.Input(
			mcp.Property("hash", mcp.Description(DownloadHashDesc)),
			mcp.Property("title", mcp.Description(DownloadTitleDesc)),
			mcp.Property("format", mcp.Description(DownloadFormatDesc)),
			mcp.Property("kindle_email", mcp.Description(DownloadKindleDesc)),
		)),
	)
}

// getToolsListJSON returns the tools list in JSON-RPC format for the tools/list handler
func getToolsListJSON() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        ToolNameSearch,
			"description": SearchToolDescription,
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"term": map[string]interface{}{
						"type":        "string",
						"description": SearchTermDesc,
					},
					"format": map[string]interface{}{
						"type":        "string",
						"description": SearchFormatDesc,
					},
				},
				"required": []string{"term"},
			},
		},
		{
			"name":        ToolNameDownload,
			"description": DownloadToolDescription,
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"hash": map[string]interface{}{
						"type":        "string",
						"description": DownloadHashDesc,
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": DownloadTitleDesc,
					},
					"format": map[string]interface{}{
						"type":        "string",
						"description": DownloadFormatDesc,
					},
					"kindle_email": map[string]interface{}{
						"type":        "string",
						"description": DownloadKindleDesc,
					},
				},
				"required": []string{"hash", "title", "format"},
			},
		},
	}
}

// checkEmailFallback checks if an email error should trigger a fallback to local download.
// Returns (shouldFallback, fallbackReason).
func checkEmailFallback(err error) (bool, string) {
	if err == nil {
		return false, ""
	}

	errStr := err.Error()

	if errStr == "email configuration incomplete: SMTP_HOST, SMTP_USER, SMTP_PASSWORD, and FROM_EMAIL must be set" {
		return true, "Email not configured"
	}

	if strings.Contains(errStr, "file too large") || strings.Contains(errStr, "size limit") || strings.Contains(errStr, "exceeded") {
		// Extract file size from error message using precompiled regex
		fileSizeInfo := ""
		matches := fileSizeRegex.FindStringSubmatch(errStr)
		if len(matches) >= 3 {
			fileSizeInfo = fmt.Sprintf("File size: %s bytes (%s MB)", matches[1], matches[2])
		} else if strings.Contains(errStr, "bytes (") {
			// Fallback: try simpler pattern
			parts := strings.Split(errStr, "bytes (")
			if len(parts) > 1 {
				sizePart := strings.Split(parts[1], ")")[0]
				fileSizeInfo = fmt.Sprintf("File size: %s", sizePart)
			}
		}

		if fileSizeInfo != "" {
			return true, fmt.Sprintf("File too large for email - %s. Gmail has a 25MB attachment limit (18MB recommended for email).", fileSizeInfo)
		}
		return true, "File too large for email (>18MB). Gmail has a 25MB attachment limit."
	}

	if strings.Contains(errStr, "broken pipe") {
		return true, "SMTP connection failed (likely due to file size)"
	}

	return false, ""
}

func SearchTool(ctx context.Context, cc *mcp.ServerSession, params *mcp.CallToolParamsFor[SearchParams]) (*mcp.CallToolResultFor[any], error) {
	l := logger.GetLogger()

	preferredFormat := params.Arguments.PreferredFormat
	if preferredFormat == "" {
		preferredFormat = "epub" // Default to EPUB for Kindle compatibility
	}

	l.Info("Search command called",
		zap.String("searchTerm", params.Arguments.SearchTerm),
		zap.String("preferredFormat", preferredFormat),
	)

	books, err := anna.FindBookWithFormat(params.Arguments.SearchTerm, preferredFormat)
	if err != nil {
		l.Error("Search command failed",
			zap.String("searchTerm", params.Arguments.SearchTerm),
			zap.Error(err),
		)
		return nil, err
	}

	// Limit results to prevent huge responses that might timeout
	maxResults := 30
	if len(books) > maxResults {
		books = books[:maxResults]
		l.Info("Limited search results", zap.Int("totalFound", len(books)), zap.Int("returned", maxResults))
	}

	bookList := ""
	for i, book := range books {
		bookList += book.String()
		if i < len(books)-1 {
			bookList += "\n\n"
		}
	}
	
	if len(books) == 0 {
		bookList = "No books found for your search term."
	}

	l.Info("Search command completed successfully",
		zap.String("searchTerm", params.Arguments.SearchTerm),
		zap.Int("resultsCount", len(books)),
	)

	// Wrap books array in a dictionary for Le Chat compatibility
	// Le Chat expects structuredContent to be a dict, not a list
	structuredContent := map[string]interface{}{
		"items": books,
	}

	return &mcp.CallToolResultFor[any]{
		Content:           []mcp.Content{&mcp.TextContent{Text: bookList}},
		StructuredContent: structuredContent,
	}, nil
}

func DownloadTool(ctx context.Context, cc *mcp.ServerSession, params *mcp.CallToolParamsFor[DownloadParams]) (*mcp.CallToolResultFor[any], error) {
	l := logger.GetLogger()

	hash := params.Arguments.BookHash
	
	// Determine which Kindle email to use: provided or default (need this first for duplicate check)
	kindleEmail := params.Arguments.KindleEmail
	if kindleEmail == "" {
		env, err := GetEnv()
		if err == nil {
			kindleEmail = env.KindleEmail
		}
	}

	// Create a composite key for duplicate tracking: hash:kindle_email
	// This allows the same book to be sent to different Kindles
	trackerKey := hash + ":" + kindleEmail

	// Check if this hash+kindle was recently downloaded to prevent duplicates
	downloadTrackerMu.RLock()
	lastDownload, recentlyDownloaded := downloadTracker[trackerKey]
	downloadTrackerMu.RUnlock()
	
	if recentlyDownloaded {
		timeSinceLastDownload := time.Since(lastDownload)
		if timeSinceLastDownload < downloadCooldown {
			l.Info("Download request ignored - same book recently sent to this Kindle",
				zap.String("bookHash", hash),
				zap.String("kindleEmail", kindleEmail),
				zap.String("title", params.Arguments.Title),
				zap.Duration("timeSinceLastDownload", timeSinceLastDownload),
				zap.Duration("cooldown", downloadCooldown),
			)
			return &mcp.CallToolResultFor[any]{
				Content: []mcp.Content{&mcp.TextContent{
					Text: "Download skipped - this book was recently sent to this Kindle. Please wait a moment before trying again.",
				}},
			}, nil
		}
	}
	
	// Record this download attempt
	downloadTrackerMu.Lock()
	downloadTracker[trackerKey] = time.Now()
	downloadTrackerMu.Unlock()

	l.Info("Download command called",
		zap.String("bookHash", hash),
		zap.String("title", params.Arguments.Title),
		zap.String("format", params.Arguments.Format),
		zap.String("kindleEmail", kindleEmail),
	)

	env, err := GetEnv()
	if err != nil {
		l.Error("Failed to get environment variables", zap.Error(err))
		return nil, err
	}
	secretKey := env.SecretKey
	downloadPath := env.DownloadPath

	title := params.Arguments.Title
	format := params.Arguments.Format
	book := &anna.Book{
		Hash:   params.Arguments.BookHash,
		Title:  title,
		Format: format,
	}

	// Try to email to Kindle first, fall back to regular download if email not configured or file too large
	err = book.EmailToKindle(secretKey, env.SMTPHost, env.SMTPPort, env.SMTPUser, env.SMTPPassword, env.FromEmail, kindleEmail)
	if err != nil {
		shouldFallback, fallbackReason := checkEmailFallback(err)
		if shouldFallback {
			l.Info("Falling back to regular download",
				zap.String("reason", fallbackReason),
				zap.String("original_error", err.Error()),
			)
			err = book.Download(secretKey, downloadPath)
			if err != nil {
				l.Error("Download command failed",
					zap.String("bookHash", params.Arguments.BookHash),
					zap.String("downloadPath", downloadPath),
					zap.Error(err),
				)
				return nil, err
			}
			l.Info("Download command completed successfully",
				zap.String("bookHash", params.Arguments.BookHash),
				zap.String("downloadPath", downloadPath),
			)
			return &mcp.CallToolResultFor[any]{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Book downloaded successfully to path: %s\n\n%s\n\nThe file was saved locally instead of emailing to Kindle.", downloadPath, fallbackReason),
				}},
			}, nil
		}
		// Email failed for another reason that we can't recover from
		l.Error("Failed to email book to Kindle",
			zap.String("bookHash", params.Arguments.BookHash),
			zap.Error(err),
		)
		return nil, err
	}

	l.Info("Book sent to Kindle successfully",
		zap.String("bookHash", params.Arguments.BookHash),
		zap.String("kindleEmail", kindleEmail),
	)

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{
			Text: "Book sent to Kindle successfully at: " + kindleEmail,
		}},
	}, nil
}

func StartMCPServer() {
	l := logger.GetLogger()
	defer l.Sync()

	serverVersion := version.GetVersion()
	l.Info("Starting MCP server",
		zap.String("name", "annas-mcp"),
		zap.String("version", serverVersion),
	)

	server := mcp.NewServer("annas-mcp", serverVersion, nil)
	addToolsToServer(server)

	l.Info("MCP server started successfully")

	if err := server.Run(context.Background(), mcp.NewStdioTransport()); err != nil {
		l.Fatal("MCP server failed", zap.Error(err))
	}
}

// StartMCPHTTPServer starts an HTTP server that exposes MCP tools via HTTP endpoints
func StartMCPHTTPServer(port string) {
	l := logger.GetLogger()
	defer l.Sync()

	if port == "" {
		port = "8080"
	}

	serverVersion := version.GetVersion()
	l.Info("Starting MCP HTTP server",
		zap.String("name", "annas-mcp"),
		zap.String("version", serverVersion),
		zap.String("port", port),
	)

	// Create MCP server instance for tool definitions
	mcpServer := mcp.NewServer("annas-mcp", serverVersion, nil)
	addToolsToServer(mcpServer)

	mux := http.NewServeMux()

	// Root endpoint - some clients check this first
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodGet || r.Method == http.MethodOptions {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name":    "annas-mcp",
				"version": serverVersion,
				"mcp":     "/mcp",
			})
			return
		}
		// For POST, redirect to /mcp
		if r.Method == http.MethodPost {
			r.URL.Path = "/mcp"
			mux.ServeHTTP(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// MCP protocol endpoint (JSON-RPC 2.0)
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		// Handle OPTIONS for CORS preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Handle GET for discovery
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name":     "annas-mcp",
				"version":  serverVersion,
				"protocol": "mcp",
			})
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var jsonRPCReq struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      interface{}     `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params,omitempty"`
		}

		// Log the raw request for debugging
		bodyBytes, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		l.Info("MCP request received",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("body", string(bodyBytes)),
		)

		if err := json.NewDecoder(strings.NewReader(string(bodyBytes))).Decode(&jsonRPCReq); err != nil {
			l.Error("Failed to decode JSON-RPC request", zap.Error(err), zap.String("body", string(bodyBytes)))
			http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
			return
		}

		l.Info("Parsed JSON-RPC request",
			zap.String("jsonrpc", jsonRPCReq.JSONRPC),
			zap.String("method", jsonRPCReq.Method),
			zap.Any("id", jsonRPCReq.ID),
		)

		if jsonRPCReq.JSONRPC != "2.0" {
			sendJSONRPCError(w, jsonRPCReq.ID, -32600, "Invalid Request", "jsonrpc must be '2.0'")
			return
		}

		var jsonRPCResp interface{}

		switch jsonRPCReq.Method {
		case "initialize":
			var params struct {
				ProtocolVersion string                 `json:"protocolVersion"`
				Capabilities    map[string]interface{} `json:"capabilities"`
				ClientInfo      map[string]interface{} `json:"clientInfo,omitempty"`
			}
			if err := json.Unmarshal(jsonRPCReq.Params, &params); err != nil {
				sendJSONRPCError(w, jsonRPCReq.ID, -32602, "Invalid params", err.Error())
				return
			}

			l.Info("Initialize request",
				zap.String("protocolVersion", params.ProtocolVersion),
				zap.Any("capabilities", params.Capabilities),
			)

			// Return the protocol version that was requested, or default to 2024-11-05
			protocolVersion := params.ProtocolVersion
			if protocolVersion == "" || protocolVersion == "1.0" {
				protocolVersion = "2024-11-05"
			}

			jsonRPCResp = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      jsonRPCReq.ID,
				"result": map[string]interface{}{
					"protocolVersion": protocolVersion,
					"capabilities": map[string]interface{}{
						"tools": map[string]interface{}{},
					},
					"serverInfo": map[string]interface{}{
						"name":    "annas-mcp",
						"version": serverVersion,
					},
				},
			}

			l.Info("Initialize response", zap.Any("response", jsonRPCResp))

		case "ping":
			// Respond to MCP ping (JSON-RPC) with empty result per spec
			jsonRPCResp = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      jsonRPCReq.ID,
				"result":  map[string]interface{}{},
			}

		case "tools/list":
			jsonRPCResp = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      jsonRPCReq.ID,
				"result": map[string]interface{}{
					"tools": getToolsListJSON(),
				},
			}

		case "tools/call":
			var params struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			if err := json.Unmarshal(jsonRPCReq.Params, &params); err != nil {
				sendJSONRPCError(w, jsonRPCReq.ID, -32602, "Invalid params", err.Error())
				return
			}

			ctx := context.Background()
			var result *mcp.CallToolResultFor[any]
			var callErr error

			switch params.Name {
			case "search":
				// Handle both "term" and "query" parameter names (Le Chat uses "query")
				var term string
				var ok bool
				if term, ok = params.Arguments["term"].(string); !ok {
					if term, ok = params.Arguments["query"].(string); !ok {
						sendJSONRPCError(w, jsonRPCReq.ID, -32602, "Invalid params", "term or query must be a string")
						return
					}
				}
				format, _ := params.Arguments["format"].(string) // Optional
				searchParams := &mcp.CallToolParamsFor[SearchParams]{
					Arguments: SearchParams{
						SearchTerm:      term,
						PreferredFormat: format,
					},
				}
				result, callErr = SearchTool(ctx, nil, searchParams)

			case "download":
				hash, _ := params.Arguments["hash"].(string)
				title, _ := params.Arguments["title"].(string)
				format, _ := params.Arguments["format"].(string)
				kindleEmail, _ := params.Arguments["kindle_email"].(string) // Optional
				if hash == "" || title == "" || format == "" {
					sendJSONRPCError(w, jsonRPCReq.ID, -32602, "Invalid params", "hash, title, and format are required")
					return
				}
				downloadParams := &mcp.CallToolParamsFor[DownloadParams]{
					Arguments: DownloadParams{
						BookHash:    hash,
						Title:       title,
						Format:      format,
						KindleEmail: kindleEmail,
					},
				}
				result, callErr = DownloadTool(ctx, nil, downloadParams)

			default:
				sendJSONRPCError(w, jsonRPCReq.ID, -32601, "Method not found", "Unknown tool: "+params.Name)
				return
			}

			if callErr != nil {
				l.Error("Tool execution failed", 
					zap.String("tool", params.Name),
					zap.Error(callErr),
				)
				sendJSONRPCError(w, jsonRPCReq.ID, -32000, "Tool execution error", callErr.Error())
				return
			}

			// Convert result to JSON-RPC format
			content := []map[string]interface{}{}
			for _, c := range result.Content {
				if textContent, ok := c.(*mcp.TextContent); ok {
					content = append(content, map[string]interface{}{
						"type": "text",
						"text": textContent.Text,
					})
				}
			}

			// Include StructuredContent if available (e.g., Book structs from search)
			resultMap := map[string]interface{}{
				"content": content,
			}
			if result.StructuredContent != nil {
				resultMap["structuredContent"] = result.StructuredContent
			}

			l.Info("Tool execution successful",
				zap.String("tool", params.Name),
				zap.Int("contentItems", len(content)),
			)

			jsonRPCResp = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      jsonRPCReq.ID,
				"result":  resultMap,
			}

		default:
			sendJSONRPCError(w, jsonRPCReq.ID, -32601, "Method not found", "Unknown method: "+jsonRPCReq.Method)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResp)
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"name":    "annas-mcp",
			"version": serverVersion,
		})
	})

	// Ping endpoint for MCP validation/handshake checks
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"name":   "annas-mcp",
		})
	})

	// CORS middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		mux.ServeHTTP(w, r)
	})

	addr := ":" + port
	l.Info("MCP HTTP server listening", zap.String("address", addr))

	if err := http.ListenAndServe(addr, handler); err != nil {
		l.Fatal("MCP HTTP server failed", zap.Error(err))
		os.Exit(1)
	}
}

// sendJSONRPCError sends a JSON-RPC 2.0 error response
func sendJSONRPCError(w http.ResponseWriter, id interface{}, code int, message, data string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
			"data":    data,
		},
	})
}
