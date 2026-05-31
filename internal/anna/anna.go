package anna

import (
	"bytes"
	"encoding/base64"
	"fmt"
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
	"github.com/sam-hartman/kindle-pibrarian/internal/logger"
	"github.com/sam-hartman/kindle-pibrarian/internal/relay"
	"go.uber.org/zap"
)

// Anna's Archive base URLs are configurable and tried in order; see domains.go
// (ANNAS_BASE_URLS env override). The old hardcoded annas-archive.li endpoints
// were removed because that domain is now parked.

// Supported formats and languages
var (
	supportedFormats   = []string{"epub", "pdf", "mobi", "azw", "azw3"}
	supportedLanguages = []string{"english", "spanish", "french", "german", "italian", "portuguese", "russian", "chinese", "japanese", "korean", "arabic", "dutch", "polish", "turkish"}
	languageMap        = map[string]string{
		"english": "English", "spanish": "Spanish", "french": "French",
		"german": "German", "italian": "Italian", "portuguese": "Portuguese",
		"russian": "Russian", "chinese": "Chinese", "japanese": "Japanese",
		"korean": "Korean", "arabic": "Arabic", "dutch": "Dutch",
		"polish": "Polish", "turkish": "Turkish", "en": "English",
		"es": "Spanish", "fr": "French", "de": "German", "it": "Italian",
		"pt": "Portuguese", "ru": "Russian", "zh": "Chinese", "ja": "Japanese",
	}
)

// getMimeType returns the MIME type for a given format
func getMimeType(format string) string {
	switch strings.ToLower(format) {
	case "pdf":
		return "application/pdf"
	case "epub":
		return "application/epub+zip"
	case "mobi":
		return "application/x-mobipocket-ebook"
	case "azw", "azw3":
		return "application/vnd.amazon.ebook"
	default:
		return "application/octet-stream"
	}
}

// containsAny checks if string s contains any of the substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// cleanTitle removes file paths and cleans up a book title
func cleanTitle(title string) string {
	if title == "" {
		return ""
	}
	title = strings.TrimSpace(title)

	// Extract filename from path (take part after last /)
	if idx := strings.LastIndex(title, "/"); idx >= 0 && idx < len(title)-1 {
		title = title[idx+1:]
	}

	// Clean up common patterns
	title = strings.ReplaceAll(title, "\\", " ")
	for _, prefix := range []string{"lgli/", "upload/", "nexusstc/", "!!", "zlib/"} {
		title = strings.TrimPrefix(title, prefix)
	}

	// Remove file extensions
	for _, ext := range []string{".epub", ".mobi", ".pdf", ".azw3", ".zip", ".nodrm"} {
		if strings.HasSuffix(strings.ToLower(title), ext) {
			title = title[:len(title)-len(ext)]
			break
		}
	}

	// Replace underscores, clean whitespace, take first line
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.Join(strings.Fields(title), " ")
	if idx := strings.Index(title, "\n"); idx > 0 {
		title = title[:idx]
	}

	// Remove drive letters (e.g., "R:")
	if len(title) > 2 && title[1] == ':' {
		title = strings.TrimSpace(title[2:])
	}

	return strings.TrimSpace(title)
}

// SendFileToKindle sends file data to Kindle email address (exported for test email command)
func SendFileToKindle(fileData []byte, filename, mimeType, subject, smtpHost, smtpPort, smtpUser, smtpPassword, fromEmail, kindleEmail string) error {
	l := logger.GetLogger()

	// Sanitize EPUBs before sending. Many Anna's Archive EPUBs were round-tripped
	// out of Amazon's ecosystem and carry invalid data-Amzn* XHTML attributes that
	// make Amazon's own Send-to-Kindle converter fail with "E999 - Send to Kindle
	// Internal Error". Stripping them yields a spec-compliant (epubcheck-clean)
	// EPUB that Amazon converts normally. No-op for non-EPUB or already-clean files.
	if mimeType == "application/epub+zip" || strings.HasSuffix(strings.ToLower(filename), ".epub") {
		if cleaned, stripped, err := SanitizeEPUB(fileData); err != nil {
			l.Warn("EPUB sanitize failed; sending original", zap.Error(err))
		} else if stripped > 0 {
			l.Info("Sanitized EPUB before sending to Kindle",
				zap.String("filename", filename),
				zap.Int("stripped_attrs", stripped),
				zap.Int("bytes_before", len(fileData)),
				zap.Int("bytes_after", len(cleaned)),
			)
			fileData = cleaned
		}
		// Validate the (now-sanitized) EPUB. Reject DRM / structurally broken
		// files up front with a clear reason, so the user can pick another edition
		// instead of getting a silent E999 from Amazon hours later.
		if err := ValidateEPUB(fileData); err != nil {
			return fmt.Errorf("this EPUB can't be sent to Kindle: %w", err)
		}
	}

	// Gmail has a 25MB attachment limit. Base64 encoding increases size by ~33%,
	// so we need to check if the original file is under ~19MB (19MB * 1.33 ≈ 25MB)
	// Adding some overhead for email headers, we'll use 18MB as the limit
	const maxFileSizeForEmail = 18 * 1024 * 1024 // 18MB

	fileSize := int64(len(fileData))
	if fileSize > maxFileSizeForEmail {
		return fmt.Errorf("file too large for email: %d bytes (%.2f MB). Gmail has a 25MB attachment limit. Files larger than ~18MB cannot be sent via email. Consider downloading directly instead",
			fileSize, float64(fileSize)/(1024*1024))
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
	emailBody.WriteString(fmt.Sprintf("%s\r\n", subject))
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

	l.Info("Sending file to Kindle",
		zap.String("filename", filename),
		zap.String("kindle_email", kindleEmail),
		zap.Int64("file_size_bytes", fileSize),
		zap.Float64("file_size_mb", float64(fileSize)/(1024*1024)),
	)

	err := smtp.SendMail(addr, auth, fromEmail, []string{kindleEmail}, emailBody.Bytes())
	if err != nil {
		// Check if error is related to file size
		errStr := err.Error()
		if strings.Contains(errStr, "exceeded") || strings.Contains(errStr, "size limit") || strings.Contains(errStr, "552") {
			return fmt.Errorf("file too large for Gmail (25MB limit): %w. Original file size: %d bytes (%.2f MB). Consider downloading directly instead",
				err, fileSize, float64(fileSize)/(1024*1024))
		}
		if strings.Contains(errStr, "broken pipe") {
			return fmt.Errorf("SMTP connection closed prematurely (likely due to file size or network issue): %w. File size: %d bytes (%.2f MB). Try again or download directly",
				err, fileSize, float64(fileSize)/(1024*1024))
		}
		return fmt.Errorf("failed to send email: %w", err)
	}

	l.Info("File sent to Kindle successfully",
		zap.String("filename", filename),
		zap.String("kindle_email", kindleEmail),
		zap.Int64("file_size_bytes", fileSize),
	)

	return nil
}

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
	// Try to detect from Content-Type header first
	if contentType != "" {
		ct := strings.ToLower(contentType)
		for _, fmt := range supportedFormats {
			if strings.Contains(ct, fmt) {
				return fmt, getMimeType(fmt)
			}
		}
	}

	// Detect from file magic bytes
	if len(fileData) >= 4 && string(fileData[0:4]) == "%PDF" {
		return "pdf", getMimeType("pdf")
	}
	if len(fileData) >= 2 && string(fileData[0:2]) == "PK" {
		// A zip that starts with PK is only an EPUB if it actually contains EPUB
		// structure; CBZ/DOCX/ODT also start with PK and Amazon rejects them.
		if isEPUBZip(fileData) {
			return "epub", getMimeType("epub")
		}
		return "unknown", "application/octet-stream"
	}
	// MOBI: check at offset 60 (PalmDOC format) or offset 0
	if len(fileData) >= 68 && string(fileData[60:68]) == "BOOKMOBI" {
		return "mobi", getMimeType("mobi")
	}
	if len(fileData) >= 8 && (string(fileData[0:8]) == "BOOKMOBI" || string(fileData[0:8]) == "ITZEBX01" || string(fileData[0:8]) == "ITZEBX02") {
		if string(fileData[0:8]) == "BOOKMOBI" {
			return "mobi", getMimeType("mobi")
		}
		return "azw3", getMimeType("azw3")
	}

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
		langLower := strings.ToLower(potentialLang)
		if mappedLang, ok := languageMap[langLower]; ok {
			language = mappedLang
		} else if len(potentialLang) > 1 && len(potentialLang) < 20 {
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

// downloadFileData downloads a file from Anna's Archive using the API.
// It tries multiple download servers and returns the file data.
func downloadFileData(hash, secretKey string) ([]byte, error) {
	l := logger.GetLogger()

	var fileData []byte
	var lastErr error

	// Try each mirror, then each download server (domain_index), until one works.
	for _, base := range annasBases() {
		for domainIndex := 0; domainIndex <= 4; domainIndex++ {
			apiURL := annasDownloadURL(base, hash, secretKey, domainIndex)
			l.Info("Fetching download URL from Anna's Archive API",
				zap.String("hash", hash),
				zap.Int("domainIndex", domainIndex),
			)

			resp, err := http.Get(apiURL)
			if err != nil {
				lastErr = fmt.Errorf("failed to get download URL: %w", err)
				continue
			}

			var apiResp fastDownloadResponse
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				resp.Body.Close()
				lastErr = fmt.Errorf("failed to decode API response: %w", err)
				continue
			}
			resp.Body.Close()

			// Check HTTP status code first
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
				if apiResp.Error != "" {
					lastErr = fmt.Errorf("API error (status %d): %s", resp.StatusCode, apiResp.Error)
				} else {
					lastErr = fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
				}
				continue
			}

			if apiResp.DownloadURL == "" {
				if apiResp.Error != "" {
					lastErr = fmt.Errorf("Anna's Archive API error: %s", apiResp.Error)
				} else {
					lastErr = errors.New("failed to get download URL from API")
				}
				continue
			}

			// Create HTTP client with timeout
			client := &http.Client{Timeout: 60 * time.Second}
			downloadResp, err := client.Get(apiResp.DownloadURL)
			if err != nil {
				l.Warn("Download server failed, trying next",
					zap.Int("domainIndex", domainIndex),
					zap.Error(err),
				)
				lastErr = fmt.Errorf("failed to download file: %w", err)
				continue
			}

			if downloadResp.StatusCode != http.StatusOK {
				downloadResp.Body.Close()
				l.Warn("Download server returned non-200, trying next",
					zap.Int("domainIndex", domainIndex),
					zap.Int("status", downloadResp.StatusCode),
				)
				lastErr = fmt.Errorf("download server returned status %d", downloadResp.StatusCode)
				continue
			}

			// Successfully connected, read the file
			fileData, err = io.ReadAll(downloadResp.Body)
			downloadResp.Body.Close()
			if err != nil {
				lastErr = fmt.Errorf("failed to read file: %w", err)
				fileData = nil
				continue
			}

			// Guard against HTML interstitials / captcha pages / truncated bodies
			// being saved or emailed as a book. detectFileFormat returns "unknown"
			// for anything that isn't a recognized ebook (PDF/EPUB/MOBI/AZW3).
			if detectedFormat, _ := detectFileFormat("", fileData); detectedFormat == "unknown" {
				l.Warn("Download returned a non-book response; trying next server",
					zap.Int("domainIndex", domainIndex),
					zap.Int("size", len(fileData)),
				)
				lastErr = fmt.Errorf("download returned a non-book response (%d bytes)", len(fileData))
				fileData = nil
				continue
			}

			l.Info("Successfully downloaded from server",
				zap.Int("domainIndex", domainIndex),
				zap.Int("size", len(fileData)),
			)
			break
		}
		if len(fileData) > 0 {
			break
		}
	}

	if len(fileData) == 0 {
		if lastErr != nil {
			return nil, fmt.Errorf("all download servers/mirrors failed: %w", lastErr)
		}
		return nil, errors.New("failed to download file from any server")
	}

	return fileData, nil
}

func FindBook(query string) ([]*Book, error) {
	return FindBookWithFormat(query, "")
}

func FindBookWithFormat(query, preferredFormat string) ([]*Book, error) {
	l := logger.GetLogger()

	// When the relay is configured (Fly deployment), forward the
	// search to the Pi's annas-mcp via the relay. The Pi already
	// scrapes annas-archive successfully from a residential IP.
	if _, _, ok := relay.Config(); ok {
		return findBookViaRelay(query, preferredFormat)
	}

	l.Info("Starting search", zap.String("query", query))
	bookList := scrapeSearch(query)
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

		// Clean up title
		title = cleanTitle(title)

		// Extract metadata - look for text containing format/language/size info
		var metaText, authorsText string
		containerText := strings.ToLower(container.Text())
		sizeIndicators := []string{"mb", "kb", "gb", "bytes"}

		// Look for format indicators in the container text
		if containsAny(containerText, supportedFormats) {
			container.Find("div, span").Each(func(i int, s *goquery.Selection) {
				text := strings.ToLower(strings.TrimSpace(s.Text()))
				if (containsAny(text, supportedFormats) || containsAny(text, supportedLanguages) || containsAny(text, sizeIndicators)) && len(text) < 500 {
					if metaText == "" || (len(text) < len(metaText) && strings.Contains(text, ",")) {
						metaText = strings.TrimSpace(s.Text())
					}
				}
			})
		}

		// Also look for language indicators more broadly
		if metaText == "" {
			for _, lang := range supportedLanguages {
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
			Language: strings.TrimSpace(language),
			Format:   strings.TrimSpace(formatTrimmed),
			Size:     strings.TrimSpace(size),
			Title:    title,
			Authors:  strings.TrimSpace(authorsText),
			URL:      e.Request.AbsoluteURL(link),
			Hash:     hash,
		}

		// Only add if we have at least a hash
		if hash != "" {
			bookListParsed = append(bookListParsed, book)
		}
	}

	// Sort results: prioritize preferred format (default to epub for Kindle)
	// EPUBs are best for Kindle: small, reflowable, searchable
	if preferredFormat == "" {
		preferredFormat = "epub"
	}
	preferredFormat = strings.ToLower(preferredFormat)

	// Sort to put preferred format first
	if len(bookListParsed) > 0 {
		// Move preferred format to front
		var preferred []*Book
		var others []*Book

		for _, book := range bookListParsed {
			if strings.ToLower(book.Format) == preferredFormat {
				preferred = append(preferred, book)
			} else {
				others = append(others, book)
			}
		}

		// Combine: preferred first, then others
		bookListParsed = append(preferred, others...)

		l.Info("Search results sorted by format preference",
			zap.String("preferredFormat", preferredFormat),
			zap.Int("preferredCount", len(preferred)),
			zap.Int("othersCount", len(others)),
		)
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

	// Relay path: Pi performs the download itself. We don't get the
	// file bytes back (Fly has no persistent storage for them anyway).
	if _, _, ok := relay.Config(); ok {
		return downloadViaRelay(b.Hash, b.Title, b.Format, b.Authors, "")
	}

	// Download file using shared helper
	fileData, err := downloadFileData(b.Hash, secretKey)
	if err != nil {
		return err
	}

	// Detect actual file format from file content
	actualFormat, _ := detectFileFormat("", fileData)

	// Use detected format if available, otherwise fall back to search result format
	format := actualFormat
	if format == "unknown" {
		format = b.Format
	}

	filename := sanitizeFilename(b.Title) + "." + format
	filePath := filepath.Join(folderPath, filename)

	// Check if file already exists to avoid duplicates
	if _, err := os.Stat(filePath); err == nil {
		l.Info("File already exists, skipping save",
			zap.String("path", filePath),
		)
	} else {
		if err := os.WriteFile(filePath, fileData, 0644); err != nil {
			// Log but don't fail if we can't save to disk
			l.Warn("Failed to save file to disk", zap.Error(err))
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

	// Relay path: the Pi-side annas-mcp handles the download + SMTP (and the
	// auto-fallback below) itself, so we just forward the request (incl. author
	// so the Pi can safely match alternate editions).
	if _, _, ok := relay.Config(); ok {
		if kindleEmail == "" {
			return errors.New("kindle_email required for EmailToKindle via relay")
		}
		return downloadViaRelay(b.Hash, b.Title, b.Format, b.Authors, kindleEmail)
	}

	// Check if email is configured
	if smtpHost == "" || smtpUser == "" || smtpPassword == "" || fromEmail == "" {
		return errors.New("email configuration incomplete: SMTP_HOST, SMTP_USER, SMTP_PASSWORD, and FROM_EMAIL must be set")
	}

	// Try the requested edition first.
	firstErr := sendOneEdition(b, secretKey, smtpHost, smtpPort, smtpUser, smtpPassword, fromEmail, kindleEmail)
	if firstErr == nil {
		l.Info("Book sent to Kindle successfully",
			zap.String("title", b.Title),
			zap.String("kindle_email", kindleEmail),
		)
		return nil
	}
	l.Warn("Requested edition could not be sent; trying alternate editions",
		zap.String("title", b.Title), zap.Error(firstErr))

	// Auto-fallback: try other EPUB editions of the SAME book so one bad file
	// (corrupt, DRM, etc.) doesn't fail the reader — "it should just work". We
	// only do this when the author is known, to be certain we never deliver a
	// different book that merely shares the title.
	if strings.TrimSpace(b.Authors) != "" {
		for _, alt := range findAlternateEditions(b.Title, b.Authors, b.Hash) {
			if err := sendOneEdition(alt, secretKey, smtpHost, smtpPort, smtpUser, smtpPassword, fromEmail, kindleEmail); err == nil {
				l.Info("Delivered an alternate edition after the requested one failed",
					zap.String("title", b.Title), zap.String("alt_hash", alt.Hash))
				return nil
			} else {
				l.Warn("Alternate edition also failed",
					zap.String("alt_hash", alt.Hash), zap.Error(err))
			}
		}
	}

	altNote := ""
	if strings.TrimSpace(b.Authors) != "" {
		altNote = " (no working alternate edition was found either)"
	}
	return fmt.Errorf("couldn't deliver %q to Kindle%s: %w", b.Title, altNote, firstErr)
}

func (b *Book) String() string {
	return fmt.Sprintf("Title: %s\nAuthors: %s\nPublisher: %s\nLanguage: %s\nFormat: %s\nSize: %s\nURL: %s\nHash: %s",
		b.Title, b.Authors, b.Publisher, b.Language, b.Format, b.Size, b.URL, b.Hash)
}
