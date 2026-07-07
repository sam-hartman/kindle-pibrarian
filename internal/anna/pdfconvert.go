package anna

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// PDFs are a poor Kindle experience (fixed layout, no reflow) and large scans
// routinely blow past the ~18MB Send-to-Kindle email cap, so a PDF-only book
// often "just doesn't send". When the only edition we can get is a PDF, we
// convert it to EPUB (reflowable, far smaller) before emailing it.
//
// Conversion shells out to Calibre's `ebook-convert`, the de-facto standard for
// this. It's an OPTIONAL dependency: if Calibre isn't installed, or conversion
// fails/times out, callers fall back to sending the original PDF unchanged, so
// this can never make PDF delivery worse than it already is.
//
// Install on the Pi (Debian/Raspberry Pi OS):  sudo apt install -y calibre

// pdfConverterCmd is the Calibre CLI; a package var so tests can point it at a
// stub script instead of a real Calibre install.
var pdfConverterCmd = "ebook-convert"

// ErrConverterUnavailable means Calibre's ebook-convert isn't on PATH, so we
// can't convert and should send the original PDF as-is.
var ErrConverterUnavailable = errors.New("pdf→epub converter (calibre ebook-convert) not installed")

// pdfConvertTimeout bounds how long a single conversion may run. It is kept
// below the relay/HTTP request timeouts (60s) so a slow conversion fails fast
// and we fall back to the PDF, rather than the caller timing out while Calibre
// keeps grinding. Override with PDF_CONVERT_TIMEOUT_SEC.
func pdfConvertTimeout() time.Duration {
	const def = 40 * time.Second
	if v := os.Getenv("PDF_CONVERT_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return def
}

// PDFConverterAvailable reports whether Calibre's ebook-convert is on PATH.
func PDFConverterAvailable() bool {
	_, err := exec.LookPath(pdfConverterCmd)
	return err == nil
}

// ConvertPDFToEPUB converts PDF bytes to EPUB bytes via Calibre's ebook-convert.
// Returns ErrConverterUnavailable if Calibre isn't installed. The returned bytes
// are only used when err == nil; on any error the caller sends the original PDF.
func ConvertPDFToEPUB(pdfData []byte) ([]byte, error) {
	if len(pdfData) == 0 {
		return nil, errors.New("empty pdf data")
	}
	if !PDFConverterAvailable() {
		return nil, ErrConverterUnavailable
	}

	tmpDir, err := os.MkdirTemp("", "pib-pdf2epub-")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "in.pdf")
	outPath := filepath.Join(tmpDir, "out.epub")
	if err := os.WriteFile(inPath, pdfData, 0600); err != nil {
		return nil, fmt.Errorf("write temp pdf: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), pdfConvertTimeout())
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, pdfConverterCmd, inPath, outPath,
		"--enable-heuristics", // clean up PDF line breaks / hyphenation into reflowable text
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("pdf→epub conversion timed out after %s", pdfConvertTimeout())
		}
		return nil, fmt.Errorf("ebook-convert failed: %w (%s)", err, truncate(stderr.String(), 300))
	}

	epubData, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read converted epub: %w", err)
	}
	if len(epubData) == 0 {
		return nil, errors.New("conversion produced an empty epub")
	}
	// Sanity-check that we actually got an EPUB (a text-only / image-only PDF can
	// yield a degenerate output); reject anything that isn't a real EPUB zip so
	// the caller falls back to the original PDF.
	if !isEPUBZip(epubData) {
		return nil, errors.New("conversion output is not a valid epub")
	}
	return epubData, nil
}
