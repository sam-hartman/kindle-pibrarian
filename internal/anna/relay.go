package anna

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/sam-hartman/kindle-pibrarian/internal/logger"
	"github.com/sam-hartman/kindle-pibrarian/internal/relay"
	"go.uber.org/zap"
)

// JSON-RPC envelopes for the Pi's annas-mcp HTTP server.
type jsonRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Method  string         `json:"method"`
	Params  toolCallParams `json:"params"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  *struct {
		StructuredContent *struct {
			Items []*Book `json:"items"`
		} `json:"structuredContent"`
		// Some MCP responses also include a "content" array with a
		// human-readable text field; we ignore it here.
		IsError bool `json:"isError"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`

	// errText is the joined text content of an isError result, filled in by
	// decodeJSONRPC. It is raw; run it through relayErrorDetail before showing it.
	errText string
}

// callRelayTool POSTs a JSON-RPC tools/call envelope to the Pi's
// annas-mcp via the relay and returns the parsed response.
func callRelayTool(toolName string, args map[string]interface{}) (*jsonRPCResponse, error) {
	envelope := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: toolCallParams{
			Name:      toolName,
			Arguments: args,
		},
	}

	req, err := relay.NewRequest("POST", relay.TargetPiAnnasMCP, "/mcp", envelope)
	if err != nil {
		return nil, err
	}
	// MCP HTTP transport requires Accept: application/json, text/event-stream.
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := relay.Client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("relay request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read relay response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("relay returned status %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	parsed, err := decodeJSONRPC(body)
	if err != nil {
		return nil, fmt.Errorf("decode relay response: %w (body=%s)", err, truncate(string(body), 500))
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("relay tool error: %s", parsed.Error.Message)
	}
	if parsed.Result != nil && parsed.Result.IsError {
		if detail := relayErrorDetail(parsed.errText); detail != "" {
			return nil, fmt.Errorf("relay tool failed: %s", detail)
		}
		return nil, fmt.Errorf("relay tool returned isError=true")
	}
	return parsed, nil
}

// decodeJSONRPC handles both plain JSON and SSE-framed JSON
// (`event: message\ndata: {...}\n\n`).
func decodeJSONRPC(body []byte) (*jsonRPCResponse, error) {
	trimmed := body
	// Crude SSE strip: find the first `data:` line and take its value.
	for i := 0; i < len(body)-5; i++ {
		if body[i] == 'd' && string(body[i:i+5]) == "data:" {
			rest := body[i+5:]
			// Skip leading space.
			j := 0
			for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t') {
				j++
			}
			rest = rest[j:]
			// Cut at first newline-newline or end.
			end := len(rest)
			for k := 0; k < len(rest)-1; k++ {
				if rest[k] == '\n' {
					end = k
					break
				}
			}
			trimmed = rest[:end]
			break
		}
	}

	var parsed jsonRPCResponse
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return nil, err
	}
	var content struct {
		Result *struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if json.Unmarshal(trimmed, &content) == nil && content.Result != nil {
		var parts []string
		for _, c := range content.Result.Content {
			if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
				parts = append(parts, strings.TrimSpace(c.Text))
			}
		}
		parsed.errText = strings.Join(parts, " ")
	}
	return &parsed, nil
}

const maxRelayErrorChars = 300

var (
	// The relay's error text can embed a request URL (Go's http client errors
	// include the full URL, and Anna's download URL carries the API key as a
	// query parameter), so URLs are removed wholesale.
	relayURLRe = regexp.MustCompile(`(?i)\b(?:https?|wss?)://\S+`)
	// key=..., token: ..., password "...", Bearer ..., etc.
	relaySecretRe = regexp.MustCompile(`(?i)\b(api[_-]?key|secret[_-]?key|key|token|secret|passcode|passwd|password|pass|authorization)\b(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s&,;]+)`)
	relayBearerRe = regexp.MustCompile(`(?i)\bbearer\s+\S+`)
	// Long opaque strings (keys, tokens, app passwords). A bare 32-char hex md5
	// is a book id, not a secret, and stays readable.
	relayOpaqueRe = regexp.MustCompile(`[A-Za-z0-9_\-]{24,}`)
	relayMD5Re    = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)
	relaySorryRe  = regexp.MustCompile(`^Sorry\s*[—–-]+\s*I couldn't send that book to the Kindle\.?\s*`)
)

// relayErrorDetail turns the relay's error text into something safe and short to
// show a reader: secrets and URLs removed, the duplicate "Sorry" prefix dropped,
// whitespace collapsed, truncated.
func relayErrorDetail(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	text = relaySorryRe.ReplaceAllString(text, "")
	text = relayURLRe.ReplaceAllString(text, "[url]")
	text = relayBearerRe.ReplaceAllString(text, "Bearer [redacted]")
	text = relaySecretRe.ReplaceAllString(text, "$1=[redacted]")
	text = relayOpaqueRe.ReplaceAllStringFunc(text, func(m string) string {
		if relayMD5Re.MatchString(m) {
			return m
		}
		return "[redacted]"
	})
	text = strings.TrimSpace(text)
	if r := []rune(text); len(r) > maxRelayErrorChars {
		text = string(r[:maxRelayErrorChars]) + "..."
	}
	return text
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// findBookViaRelay calls the pi-annas-mcp `search` tool through the relay.
func findBookViaRelay(query, preferredFormat string) ([]*Book, error) {
	l := logger.GetLogger()
	l.Info("Searching via Pi relay",
		zap.String("query", query),
		zap.String("format", preferredFormat),
	)

	args := map[string]interface{}{"term": query}
	if preferredFormat != "" {
		args["format"] = preferredFormat
	}
	resp, err := callRelayTool("search", args)
	if err != nil {
		return nil, err
	}
	if resp.Result == nil || resp.Result.StructuredContent == nil {
		return nil, fmt.Errorf("relay response missing structuredContent")
	}
	items := resp.Result.StructuredContent.Items
	l.Info("Relay search complete", zap.Int("results", len(items)))
	return items, nil
}

// downloadViaRelay calls the pi-annas-mcp `download` tool through the
// relay. When kindleEmail is non-empty the Pi-side server handles SMTP
// directly; otherwise it just downloads the file on the Pi (we discard
// the file on this side, since the Fly app has no local storage).
func downloadViaRelay(hash, title, format, author, kindleEmail string) error {
	l := logger.GetLogger()
	args := map[string]interface{}{
		"hash":   hash,
		"title":  title,
		"format": format,
	}
	if author != "" {
		args["author"] = author
	}
	if kindleEmail != "" {
		args["kindle_email"] = kindleEmail
	}
	l.Info("Downloading via Pi relay",
		zap.String("hash", hash),
		zap.String("title", title),
		zap.Bool("send_to_kindle", kindleEmail != ""),
	)
	if _, err := callRelayTool("download", args); err != nil {
		return err
	}
	return nil
}
