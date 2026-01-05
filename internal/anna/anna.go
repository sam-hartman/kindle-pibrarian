package anna

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"time"

	"github.com/PuerkitoBio/goquery"
	colly "github.com/gocolly/colly/v2"
	"github.com/iosifache/annas-mcp/internal/logger"
	"go.uber.org/zap"
)

const (
	AnnasSearchEndpoint   = "https://annas-archive.se/search?q=%s"
	AnnasDownloadEndpoint = "https://annas-archive.se/dyn/api/fast_download.json?md5=%s&key=%s"
)

// sanitizeFilename creates a safe filename by replacing problematic characters
func sanitizeFilename(name string) string {
	// Replace problematic characters with underscores
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "<", "_")
	name = strings.ReplaceAll(name, ">", "_")
	name = strings.ReplaceAll(name, "|", "_")
	// Replace multiple spaces with single space, then replace spaces with underscores
	name = strings.Join(strings.Fields(name), " ")
	name = strings.ReplaceAll(name, " ", "_")
	// Remove leading/trailing underscores and dots
	name = strings.Trim(name, "._")
	// Limit length to avoid filesystem issues
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

// detectFileFormat detects the actual file format from Content-Type header and file content
func detectFileFormat(contentType string, fileData []byte) (format string, mimeType string) {
	// First, try to detect from Content-Type header
	if contentType != "" {
		contentType = strings.ToLower(contentType)
		if strings.Contains(contentType, "pdf") {
			return "pdf", "application/pdf"
		}
		if strings.Contains(contentType, "epub") {
			return "epub", "application/epub+zip"
		}
		if strings.Contains(contentType, "mobi") || strings.Contains(contentType, "mobipocket") {
			return "mobi", "application/x-mobipocket-ebook"
		}
		if strings.Contains(contentType, "azw") {
			return "azw3", "application/vnd.amazon.ebook"
		}
	}

	// If Content-Type doesn't help, detect from file magic bytes
	if len(fileData) >= 4 {
		// PDF files start with "%PDF"
		if len(fileData) >= 4 && string(fileData[0:4]) == "%PDF" {
			return "pdf", "application/pdf"
		}

		// EPUB files are ZIP archives, start with "PK" (ZIP magic bytes)
		if len(fileData) >= 2 && string(fileData[0:2]) == "PK" {
			// Check if it's actually an EPUB by looking for mimetype file
			// For simplicity, if it's a ZIP and we expected EPUB, assume EPUB
			// Otherwise, we could check the ZIP contents
			return "epub", "application/epub+zip"
		}

		// MOBI files can start with various headers
		// Check for MOBI magic bytes: "BOOKMOBI" or "MOBI"
		if len(fileData) >= 8 {
			header := string(fileData[0:8])
			if header == "BOOKMOBI" || header == "MOBI    " {
				return "mobi", "application/x-mobipocket-ebook"
			}
		}
		// Also check for MOBI at offset 60 (some MOBI files have different headers)
		if len(fileData) >= 68 {
			if string(fileData[60:68]) == "BOOKMOBI" {
				return "mobi", "application/x-mobipocket-ebook"
			}
		}

		// AZW files are similar to MOBI
		if len(fileData) >= 8 {
			header := string(fileData[0:8])
			if header == "ITZEBX01" || header == "ITZEBX02" {
				return "azw3", "application/vnd.amazon.ebook"
			}
		}
	}

	// Default fallback
	return "unknown", "application/octet-stream"
}

func extractMetaInformation(meta string) (language, format, size string) {
	if meta == "" {
		return "", "", ""
	}
	
	// Try splitting by comma first (common format: "English, epub, 2.5 MB")
	parts := strings.Split(meta, ", ")
	if len(parts) >= 2 {
		// First part is often language
		potentialLang := strings.TrimSpace(parts[0])
		// Check if it looks like a language name
		languageKeywords := map[string]string{
			"english": "English", "spanish": "Spanish", "french": "French", 
			"german": "German", "italian": "Italian", "portuguese": "Portuguese",
			"russian": "Russian", "chinese": "Chinese", "japanese": "Japanese",
			"korean": "Korean", "arabic": "Arabic", "dutch": "Dutch",
			"polish": "Polish", "turkish": "Turkish", "en": "English",
			"es": "Spanish", "fr": "French", "de": "German", "it": "Italian",
			"pt": "Portuguese", "ru": "Russian", "zh": "Chinese", "ja": "Japanese",
		}
		langLower := strings.ToLower(potentialLang)
		if mappedLang, ok := languageKeywords[langLower]; ok {
			language = mappedLang
		} else if len(potentialLang) > 1 && len(potentialLang) < 20 {
			// Might be a language name even if not in our map
			language = potentialLang
		}
		
		// Look for format in the parts
		for _, part := range parts {
			partLower := strings.ToLower(strings.TrimSpace(part))
			if partLower == "epub" || partLower == "pdf" || partLower == "mobi" || 
			   partLower == "azw" || partLower == "azw3" || partLower == "zip" {
				format = partLower
				break
			}
		}
		
		// Look for size (contains MB, KB, GB, or bytes)
		for _, part := range parts {
			partLower := strings.ToLower(strings.TrimSpace(part))
			if strings.Contains(partLower, "mb") || strings.Contains(partLower, "kb") || 
			   strings.Contains(partLower, "gb") || strings.Contains(partLower, "bytes") ||
			   strings.Contains(partLower, "byte") {
				size = strings.TrimSpace(part)
				break
			}
		}
	}
	
	// If format not found in comma-separated parts, search the whole string
	if format == "" {
		metaLower := strings.ToLower(meta)
		if strings.Contains(metaLower, "epub") {
			format = "epub"
		} else if strings.Contains(metaLower, "pdf") {
			format = "pdf"
		} else if strings.Contains(metaLower, "mobi") {
			format = "mobi"
		} else if strings.Contains(metaLower, "azw3") {
			format = "azw3"
		} else if strings.Contains(metaLower, "azw") {
			format = "azw"
		}
	}
	
	// If size not found, search the whole string for size patterns
	if size == "" {
		// Look for patterns like "2.5 MB", "500 KB", "1.2 GB", etc.
		words := strings.Fields(meta)
		for i, word := range words {
			wordLower := strings.ToLower(word)
			if strings.Contains(wordLower, "mb") || strings.Contains(wordLower, "kb") ||
				strings.Contains(wordLower, "gb") || strings.Contains(wordLower, "bytes") {
				// Try to get the number before it
				if i > 0 {
					size = strings.TrimSpace(words[i-1] + " " + word)
				} else {
					size = strings.TrimSpace(word)
				}
				break
			}
		}
	}

	return language, format, size
}

func FindBook(query string) ([]*Book, error) {
	l := logger.GetLogger()

	c := colly.NewCollector(
		colly.Async(true),
	)

	// Configure HTTP client with TLS support
	// Using more compatible TLS settings
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         "annas-archive.se", // Updated to new domain
		},
		DisableCompression: false,
		ForceAttemptHTTP2:  false, // Use HTTP/1.1
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
	c.SetClient(client)

	// Set user agent to avoid being blocked
	c.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	bookList := make([]*colly.HTMLElement, 0)

	// Look for book entries - Anna's Archive uses specific structures
	c.OnHTML("a[href^='/md5/']", func(e *colly.HTMLElement) {
		bookList = append(bookList, e)
		l.Debug("Found book link", zap.String("href", e.Attr("href")))
	})
	
	// Also try to find book cards/items directly
	c.OnHTML("[class*='book'], [class*='item'], [class*='result']", func(e *colly.HTMLElement) {
		link := e.DOM.Find("a[href^='/md5/']").First()
		if link.Length() > 0 {
			href, _ := link.Attr("href")
			if href != "" {
				// Create a temporary element for this book
				tempE := &colly.HTMLElement{
					DOM:      link,
					Request:  e.Request,
					Response: e.Response,
				}
				bookList = append(bookList, tempE)
			}
		}
	})

	c.OnRequest(func(r *colly.Request) {
		l.Info("Visiting URL", zap.String("url", r.URL.String()))
		r.Headers.Set("User-Agent", c.UserAgent)
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.5")
	})

	c.OnError(func(r *colly.Response, err error) {
		l.Error("Request failed", zap.String("url", r.Request.URL.String()), zap.Error(err))
	})

	c.OnResponse(func(r *colly.Response) {
		l.Info("Received response", zap.String("url", r.Request.URL.String()), zap.Int("status", r.StatusCode), zap.Int("size", len(r.Body)))
	})

	fullURL := fmt.Sprintf(AnnasSearchEndpoint, url.QueryEscape(query))
	l.Info("Starting search", zap.String("query", query), zap.String("url", fullURL))
	c.Visit(fullURL)
	c.Wait()
	l.Info("Search completed", zap.Int("linksFound", len(bookList)))

	bookListParsed := make([]*Book, 0)
	seenHashes := make(map[string]bool)

	for _, e := range bookList {
		link := e.Attr("href")
		hash := strings.TrimPrefix(link, "/md5/")
		
		// Skip duplicates
		if seenHashes[hash] {
			continue
		}
		seenHashes[hash] = true

		// Navigate up the DOM to find the book container
		// Anna's Archive typically has the book info in a parent container
		container := e.DOM.Closest("div, article, section, li")
		if container.Length() == 0 {
			container = e.DOM.Parent()
		}

		// Extract title - try multiple strategies
		title := ""
		// Strategy 1: Look for heading tags (h1-h4) in the container
		for i := 1; i <= 4; i++ {
			title = strings.TrimSpace(container.Find(fmt.Sprintf("h%d", i)).First().Text())
			if title != "" {
				break
			}
		}
		// Strategy 2: Look for title attribute or data-title
		if title == "" {
			title, _ = container.Attr("data-title")
			title = strings.TrimSpace(title)
		}
		// Strategy 3: Get text from link itself
		if title == "" {
			title = strings.TrimSpace(e.Text)
		}
		// Strategy 4: Look for any text that seems like a title (first non-empty text node)
		if title == "" || len(title) < 3 {
			allText := strings.TrimSpace(container.Text())
			if len(allText) > 0 {
				// Take first reasonable chunk
				lines := strings.Split(allText, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if len(line) > 5 && len(line) < 200 && !strings.HasPrefix(line, "http") {
						title = line
						break
					}
				}
			}
		}
		
		// Clean up title - remove file paths and extract just the book title
		if title != "" {
			// Remove common file path patterns
			title = strings.TrimSpace(title)
			
			// Remove leading path components (e.g., "lgli/", "upload/", etc.)
			// Try multiple times to remove nested paths
			for i := 0; i < 5; i++ {
				if idx := strings.LastIndex(title, "/"); idx >= 0 && idx < len(title)-1 {
					potentialTitle := strings.TrimSpace(title[idx+1:])
					// If the part after "/" looks like a title, use it
					if len(potentialTitle) > 5 && !strings.HasPrefix(potentialTitle, "!!") {
						title = potentialTitle
					} else {
						break
					}
				} else {
					break
				}
			}
			
			// Remove backslashes (Windows paths) and replace with spaces
			title = strings.ReplaceAll(title, "\\", " ")
			
			// Remove common path prefixes that might remain
			pathPrefixes := []string{"lgli/", "upload/", "nexusstc/", "!!1", "!!"}
			for _, prefix := range pathPrefixes {
				if strings.HasPrefix(title, prefix) {
					title = strings.TrimPrefix(title, prefix)
					title = strings.TrimSpace(title)
				}
			}
			
			// Remove file extensions from the end if they're part of the title
			extensions := []string{".epub", ".mobi", ".pdf", ".azw3", ".zip", ".nodrm"}
			for _, ext := range extensions {
				if strings.HasSuffix(strings.ToLower(title), ext) {
					title = strings.TrimSuffix(title, ext)
					title = strings.TrimSuffix(title, strings.ToUpper(ext))
					break
				}
			}
			
			// Remove underscores and replace with spaces
			title = strings.ReplaceAll(title, "_", " ")
			
			// Clean up multiple spaces and special characters
			title = strings.Join(strings.Fields(title), " ")
			
			// Take only the first line if there are multiple lines
			if idx := strings.Index(title, "\n"); idx > 0 {
				title = strings.TrimSpace(title[:idx])
			}
			
			// Final cleanup: remove any remaining path-like patterns at the start
			title = strings.TrimSpace(title)
			if strings.Contains(title, ":") && strings.Index(title, ":") < 3 {
				// Looks like a drive letter (e.g., "R:")
				if parts := strings.SplitN(title, ":", 2); len(parts) == 2 {
					title = strings.TrimSpace(parts[1])
				}
			}
		}

		// Extract metadata - look for text containing format/language/size info
		var metaText, authorsText, publisherText string
		containerText := strings.ToLower(container.Text())
		
		// Look for format indicators in the container text
		if strings.Contains(containerText, "epub") || strings.Contains(containerText, "pdf") || 
		   strings.Contains(containerText, "mobi") || strings.Contains(containerText, "azw") {
			// Find the div/span that contains this info
			container.Find("div, span").Each(func(i int, s *goquery.Selection) {
				text := strings.ToLower(strings.TrimSpace(s.Text()))
				// Look for metadata strings that contain format, language, or size info
				if (strings.Contains(text, "epub") || strings.Contains(text, "pdf") || 
					strings.Contains(text, "mobi") || strings.Contains(text, "azw") ||
					strings.Contains(text, "english") || strings.Contains(text, "spanish") ||
					strings.Contains(text, "french") || strings.Contains(text, "german") ||
					strings.Contains(text, "mb") || strings.Contains(text, "kb") ||
					strings.Contains(text, "gb") || strings.Contains(text, "bytes")) &&
					len(text) < 500 {
					// Prefer shorter, more specific metadata strings
					if metaText == "" || (len(text) < len(metaText) && strings.Contains(text, ",")) {
						metaText = strings.TrimSpace(s.Text())
					}
				}
			})
		}
		
		// Also look for language indicators more broadly
		languageKeywords := []string{"english", "spanish", "french", "german", "italian", "portuguese", 
			"russian", "chinese", "japanese", "korean", "arabic", "dutch", "polish", "turkish"}
		if metaText == "" {
			for _, lang := range languageKeywords {
				if strings.Contains(containerText, lang) {
					container.Find("div, span").Each(func(i int, s *goquery.Selection) {
						text := strings.ToLower(strings.TrimSpace(s.Text()))
						if strings.Contains(text, lang) && len(text) < 200 {
							metaText = strings.TrimSpace(s.Text())
						}
					})
					if metaText != "" {
						break
					}
				}
			}
		}
		
		// Try to extract authors - look for text that might be author names
		// Authors are often in a separate div or after the title
		container.Find("div, span, p").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			// Heuristic: reasonable length, not a URL, not size info, not the title
			if text != "" && len(text) > 2 && len(text) < 300 && 
			   !strings.HasPrefix(text, "http") && 
			   !strings.Contains(strings.ToLower(text), "mb") && 
			   !strings.Contains(strings.ToLower(text), "kb") &&
			   text != title &&
			   !strings.Contains(strings.ToLower(text), "download") &&
			   !strings.Contains(strings.ToLower(text), "view") {
				// Check if it looks like an author name (has capital letters, not all caps)
				hasUpper := false
				allUpper := true
				for _, r := range text {
					if r >= 'A' && r <= 'Z' {
						hasUpper = true
					}
					if r >= 'a' && r <= 'z' {
						allUpper = false
					}
				}
				if hasUpper && !allUpper && authorsText == "" {
					authorsText = text
				}
			}
		})

		// Parse metadata
		language, format, size := extractMetaInformation(metaText)
		
		// If format not found in meta, try to extract from URL or other sources
		if format == "" {
			// Check if title or other text contains format info
			lowerTitle := strings.ToLower(title)
			if strings.Contains(lowerTitle, ".epub") || strings.Contains(metaText, "epub") {
				format = "epub"
			} else if strings.Contains(lowerTitle, ".pdf") || strings.Contains(metaText, "pdf") {
				format = "pdf"
			} else if strings.Contains(lowerTitle, ".mobi") || strings.Contains(metaText, "mobi") {
				format = "mobi"
			}
		}

		// Clean up format (remove leading characters if present)
		formatTrimmed := strings.TrimSpace(format)
		if len(formatTrimmed) > 0 {
			// Remove common prefixes
			formatTrimmed = strings.TrimPrefix(formatTrimmed, ".")
			formatTrimmed = strings.TrimPrefix(formatTrimmed, "-")
			formatTrimmed = strings.TrimPrefix(formatTrimmed, ":")
			// Remove first character if it's not alphanumeric
			if len(formatTrimmed) > 1 && !((formatTrimmed[0] >= 'a' && formatTrimmed[0] <= 'z') || 
				(formatTrimmed[0] >= 'A' && formatTrimmed[0] <= 'Z') || 
				(formatTrimmed[0] >= '0' && formatTrimmed[0] <= '9')) {
				formatTrimmed = formatTrimmed[1:]
			}
		}

		book := &Book{
			Language:  strings.TrimSpace(language),
			Format:    strings.TrimSpace(formatTrimmed),
			Size:      strings.TrimSpace(size),
			Title:     title,
			Publisher: strings.TrimSpace(publisherText),
			Authors:   strings.TrimSpace(authorsText),
			URL:       e.Request.AbsoluteURL(link),
			Hash:      hash,
		}

		// Only add if we have at least a hash
		if hash != "" {
			bookListParsed = append(bookListParsed, book)
		}
	}

	return bookListParsed, nil
}

func (b *Book) Download(secretKey, folderPath string) error {
	l := logger.GetLogger()
	l.Info("Download function called",
		zap.String("hash", b.Hash),
		zap.String("title", b.Title),
		zap.String("format", b.Format),
		zap.String("folderPath", folderPath),
	)
	
	apiURL := fmt.Sprintf(AnnasDownloadEndpoint, b.Hash, secretKey)

	resp, err := http.Get(apiURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var apiResp fastDownloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("failed to decode API response: %w", err)
	}
	
	// Check HTTP status code first
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		if apiResp.Error != "" {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, apiResp.Error)
		}
		return fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}
	
	if apiResp.DownloadURL == "" {
		if apiResp.Error != "" {
			return fmt.Errorf("Anna's Archive API error: %s (this usually means the book hash is invalid or the book doesn't exist)", apiResp.Error)
		}
		return errors.New("failed to get download URL from API")
	}

	downloadResp, err := http.Get(apiResp.DownloadURL)
	if err != nil {
		return err
	}
	defer downloadResp.Body.Close()

	if downloadResp.StatusCode != http.StatusOK {
		return errors.New("failed to download file")
	}

	// Read the file into memory
	fileData, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Detect actual file format from Content-Type and file content
	contentType := downloadResp.Header.Get("Content-Type")
	actualFormat, _ := detectFileFormat(contentType, fileData)
	
	// Use detected format if available, otherwise fall back to search result format
	format := actualFormat
	if format == "unknown" {
		format = b.Format
	}

	filename := sanitizeFilename(b.Title) + "." + format
	filePath := filepath.Join(folderPath, filename)
	
	// Check if file already exists to avoid duplicates
	if _, err := os.Stat(filePath); err == nil {
		logger.GetLogger().Info("File already exists, skipping save",
			zap.String("path", filePath),
		)
	} else {
		if err := os.WriteFile(filePath, fileData, 0644); err != nil {
			// Log but don't fail if we can't save to disk
			logger.GetLogger().Warn("Failed to save file to disk", zap.Error(err))
		}
	}

	return nil
}

// EmailToKindle sends the book file to the Kindle email address
func (b *Book) EmailToKindle(secretKey, smtpHost, smtpPort, smtpUser, smtpPassword, fromEmail, kindleEmail string) error {
	l := logger.GetLogger()
	
	l.Info("EmailToKindle function called",
		zap.String("hash", b.Hash),
		zap.String("title", b.Title),
		zap.String("format", b.Format),
	)

	// Check if email is configured
	if smtpHost == "" || smtpUser == "" || smtpPassword == "" || fromEmail == "" {
		return errors.New("email configuration incomplete: SMTP_HOST, SMTP_USER, SMTP_PASSWORD, and FROM_EMAIL must be set")
	}

	// Download the file first
	apiURL := fmt.Sprintf(AnnasDownloadEndpoint, b.Hash, secretKey)
	l.Info("Fetching download URL from Anna's Archive API",
		zap.String("hash", b.Hash),
	)
	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %w", err)
	}
	defer resp.Body.Close()

	var apiResp fastDownloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("failed to decode API response: %w", err)
	}
	
	// Check HTTP status code first
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		if apiResp.Error != "" {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, apiResp.Error)
		}
		return fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}
	
	if apiResp.DownloadURL == "" {
		if apiResp.Error != "" {
			return fmt.Errorf("Anna's Archive API error: %s (this usually means the book hash is invalid or the book doesn't exist)", apiResp.Error)
		}
		return errors.New("failed to get download URL from API")
	}

	downloadResp, err := http.Get(apiResp.DownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer downloadResp.Body.Close()

	if downloadResp.StatusCode != http.StatusOK {
		return errors.New("failed to download file")
	}

	// Read file data
	fileData, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Detect actual file format from Content-Type and file content
	contentType := downloadResp.Header.Get("Content-Type")
	actualFormat, detectedMimeType := detectFileFormat(contentType, fileData)
	
	// Save to disk as backup (optional, uses ANNAS_DOWNLOAD_PATH if set)
	// This allows you to have a local copy even when emailing
	if downloadPath := os.Getenv("ANNAS_DOWNLOAD_PATH"); downloadPath != "" {
		format := actualFormat
		if format == "unknown" {
			format = b.Format
		}
		filename := sanitizeFilename(b.Title) + "." + format
		filePath := filepath.Join(downloadPath, filename)
		
		l.Info("Checking if file already exists before saving",
			zap.String("path", filePath),
			zap.String("hash", b.Hash),
		)
		
		// Check if file already exists to avoid duplicates
		if _, err := os.Stat(filePath); err == nil {
			l.Info("File already exists, skipping save to prevent duplicate",
				zap.String("path", filePath),
				zap.String("hash", b.Hash),
			)
		} else {
			l.Info("File does not exist, saving to disk",
				zap.String("path", filePath),
				zap.String("hash", b.Hash),
				zap.Int64("size_bytes", int64(len(fileData))),
			)
			if err := os.WriteFile(filePath, fileData, 0644); err != nil {
				// Log but don't fail if we can't save to disk
				l.Warn("Failed to save file to disk as backup", 
					zap.String("path", filePath),
					zap.Error(err),
				)
			} else {
				l.Info("File saved to disk as backup successfully",
					zap.String("path", filePath),
					zap.String("hash", b.Hash),
				)
			}
		}
	} else {
		l.Info("ANNAS_DOWNLOAD_PATH not set, skipping local save",
			zap.String("hash", b.Hash),
		)
	}
	
	// Use detected format if available, otherwise fall back to search result format
	format := actualFormat
	if format == "unknown" {
		format = b.Format
		// Determine MIME type based on format if detection failed
		switch strings.ToLower(b.Format) {
		case "pdf":
			detectedMimeType = "application/pdf"
		case "epub":
			detectedMimeType = "application/epub+zip"
		case "mobi":
			detectedMimeType = "application/x-mobipocket-ebook"
		case "azw", "azw3":
			detectedMimeType = "application/vnd.amazon.ebook"
		default:
			detectedMimeType = "application/octet-stream"
		}
	}

	// Kindle email service doesn't accept MOBI files - only PDF, EPUB, DOC, DOCX, HTML, RTF, TXT
	// If we detect MOBI, we should warn and potentially convert or skip email
	if format == "mobi" {
		l.Warn("MOBI format detected - Kindle email service doesn't accept MOBI files",
			zap.String("title", b.Title),
			zap.String("format", format),
		)
		// We'll still try to send it, but it will likely be rejected by Amazon
		// In the future, we could add MOBI to EPUB conversion here
	}

	filename := sanitizeFilename(b.Title) + "." + format
	mimeType := detectedMimeType

	// Log format detection for debugging
	if actualFormat != "unknown" && actualFormat != strings.ToLower(b.Format) {
		l.Info("File format mismatch detected",
			zap.String("expected_format", b.Format),
			zap.String("actual_format", actualFormat),
			zap.String("content_type", contentType),
		)
	}

	// Create email message
	var emailBody bytes.Buffer
	emailBody.WriteString(fmt.Sprintf("From: %s\r\n", fromEmail))
	emailBody.WriteString(fmt.Sprintf("To: %s\r\n", kindleEmail))
	emailBody.WriteString(fmt.Sprintf("Subject: %s\r\n", filename))
	emailBody.WriteString("MIME-Version: 1.0\r\n")
	emailBody.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=boundary123\r\n\r\n"))
	
	// Email body
	emailBody.WriteString("--boundary123\r\n")
	emailBody.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	emailBody.WriteString(fmt.Sprintf("Book: %s\r\n", b.Title))
	emailBody.WriteString("\r\n")
	
	// Attachment
	emailBody.WriteString("--boundary123\r\n")
	emailBody.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", mimeType, filename))
	emailBody.WriteString("Content-Transfer-Encoding: base64\r\n")
	emailBody.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", filename))
	
	// Encode file as base64
	encoded := base64.StdEncoding.EncodeToString(fileData)
	// Split into lines of 76 characters (RFC 2045)
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		emailBody.WriteString(encoded[i:end] + "\r\n")
	}
	
	emailBody.WriteString("\r\n--boundary123--\r\n")

	// Send email via SMTP
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	auth := smtp.PlainAuth("", smtpUser, smtpPassword, smtpHost)

	l.Info("Sending book to Kindle",
		zap.String("title", b.Title),
		zap.String("format", b.Format),
		zap.String("kindle_email", kindleEmail),
	)

	err = smtp.SendMail(addr, auth, fromEmail, []string{kindleEmail}, emailBody.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	l.Info("Book sent to Kindle successfully",
		zap.String("title", b.Title),
		zap.String("kindle_email", kindleEmail),
	)

	return nil
}

func (b *Book) String() string {
	return fmt.Sprintf("Title: %s\nAuthors: %s\nPublisher: %s\nLanguage: %s\nFormat: %s\nSize: %s\nURL: %s\nHash: %s",
		b.Title, b.Authors, b.Publisher, b.Language, b.Format, b.Size, b.URL, b.Hash)
}

func (b *Book) ToJSON() (string, error) {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}
