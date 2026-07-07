package anna

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sam-hartman/kindle-pibrarian/internal/logger"
	"go.uber.org/zap"
)

// maxAlternateEditions caps how many other editions we try when the requested
// file can't be sent, so one failure can't fan out into a long download storm.
const maxAlternateEditions = 3

// sendOneEdition downloads a single edition by hash, validates it, saves an
// optional local backup, and emails it to the Kindle. It returns an error
// describing why the edition could not be sent (corrupt/HTML download, DRM,
// MOBI/AZW, SMTP failure, ...). EPUB sanitize + validation happen inside
// SendFileToKindle.
func sendOneEdition(b *Book, secretKey, smtpHost, smtpPort, smtpUser, smtpPassword, fromEmail, kindleEmail string) error {
	l := logger.GetLogger()

	fileData, err := downloadFileData(b.Hash, secretKey)
	if err != nil {
		return err
	}

	actualFormat, _ := detectFileFormat("", fileData)

	// Optional local backup (best-effort; never fails the send).
	if downloadPath := os.Getenv("ANNAS_DOWNLOAD_PATH"); downloadPath != "" {
		nameFormat := actualFormat
		if nameFormat == "unknown" {
			nameFormat = b.Format
		}
		filePath := filepath.Join(downloadPath, sanitizeFilename(b.Title)+"."+nameFormat)
		if _, statErr := os.Stat(filePath); statErr != nil {
			if werr := os.WriteFile(filePath, fileData, 0644); werr != nil {
				l.Warn("Failed to save backup copy", zap.String("path", filePath), zap.Error(werr))
			}
		}
	}

	if actualFormat == "unknown" {
		return fmt.Errorf("the downloaded file for %q is not a recognized ebook (likely a corrupt download or an error page)", b.Title)
	}

	// PDFs read poorly on Kindle and large ones exceed the email size cap, so a
	// PDF-only book often can't be delivered at all. Convert to EPUB first when a
	// converter is available. Best-effort: on any failure we send the original
	// PDF unchanged, so this never regresses PDF delivery.
	if actualFormat == "pdf" {
		if epubData, cerr := ConvertPDFToEPUB(fileData); cerr != nil {
			l.Warn("PDF→EPUB conversion skipped; sending original PDF",
				zap.String("title", b.Title), zap.Error(cerr))
		} else {
			l.Info("Converted PDF to EPUB before sending",
				zap.String("title", b.Title),
				zap.Int("pdf_bytes", len(fileData)),
				zap.Int("epub_bytes", len(epubData)),
			)
			fileData = epubData
			actualFormat = "epub"
		}
	}

	if actualFormat == "mobi" || actualFormat == "azw" || actualFormat == "azw3" {
		return fmt.Errorf("this edition is %s, which Amazon's Send-to-Kindle email no longer accepts", strings.ToUpper(actualFormat))
	}

	mimeType := getMimeType(actualFormat)
	filename := sanitizeFilename(b.Title) + "." + actualFormat
	if strings.TrimSuffix(strings.ToLower(filename), "."+actualFormat) == "" {
		filename = b.Hash + "." + actualFormat // guard against an empty/garbled title
	}

	return SendFileToKindle(fileData, filename, mimeType, "Book: "+b.Title,
		smtpHost, smtpPort, smtpUser, smtpPassword, fromEmail, kindleEmail)
}

// findAlternateEditions searches for other EPUB editions of the same book to try
// when the requested file can't be sent. To avoid ever delivering the WRONG
// book, candidates must match the requested title AND (when known) author.
func findAlternateEditions(title, authors, excludeHash string) []*Book {
	l := logger.GetLogger()
	if strings.TrimSpace(title) == "" {
		return nil
	}
	results, err := FindBookWithFormat(title, "epub")
	if err != nil {
		l.Warn("alternate-edition search failed", zap.Error(err))
		return nil
	}

	wantTitle := normalizeForMatch(title)
	out := make([]*Book, 0, maxAlternateEditions)
	for _, r := range results {
		if r == nil || r.Hash == "" || r.Hash == excludeHash {
			continue
		}
		if normalizeForMatch(r.Title) != wantTitle {
			continue
		}
		if authors != "" && !authorsOverlap(r.Authors, authors) {
			continue
		}
		out = append(out, r)
		if len(out) >= maxAlternateEditions {
			break
		}
	}
	return out
}

// normalizeForMatch lowercases and reduces a string to alphanumeric tokens
// separated by single spaces, for tolerant title/author comparison.
func normalizeForMatch(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastSpace = false
		case !lastSpace:
			b.WriteRune(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// authorsOverlap reports whether two author strings share a meaningful token
// (e.g. a surname), used to confirm an alternate edition is the same book.
func authorsOverlap(a, b string) bool {
	ta := tokenSet(a)
	for t := range tokenSet(b) {
		if ta[t] {
			return true
		}
	}
	return false
}

func tokenSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, t := range strings.Fields(normalizeForMatch(s)) {
		if len(t) >= 3 { // skip short tokens / initials to avoid false matches
			m[t] = true
		}
	}
	return m
}
