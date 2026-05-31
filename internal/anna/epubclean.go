package anna

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
)

// amznAttrRe matches Amazon-injected attributes such as `data-AmznRemoved="..."`.
// These are left behind when an EPUB is round-tripped out of Amazon's ecosystem
// (de-DRM'd / exported Kindle books). They are not valid EPUB XHTML attributes,
// so Amazon's own Send-to-Kindle converter fails on them with
// "E999 - Send to Kindle Internal Error". Stripping them makes the EPUB
// spec-compliant (epubcheck-clean) and lets Amazon convert it normally.
var amznAttrRe = regexp.MustCompile(`(?i)\s+data-Amzn[\w-]*(?:="[^"]*"|='[^']*'|)`)

func isXHTMLName(name string) bool {
	l := strings.ToLower(name)
	return strings.HasSuffix(l, ".xhtml") || strings.HasSuffix(l, ".html") || strings.HasSuffix(l, ".htm")
}

// SanitizeEPUB strips Amazon-injected data-Amzn* attributes from every XHTML
// document inside an EPUB and rebuilds the archive with `mimetype` stored first
// (as the EPUB spec requires).
//
// It returns the (possibly rewritten) bytes, the number of attribute
// occurrences removed, and an error. The contract is conservative and safe to
// call on any attachment:
//   - If data is not a valid zip/EPUB, the original bytes are returned with a
//     nil error and zero count (the caller can still send the file as-is).
//   - If nothing needs cleaning, the original bytes are returned unchanged so
//     the operation is idempotent and byte-stable for already-clean files.
func SanitizeEPUB(data []byte) ([]byte, int, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		// Not a zip (or corrupt) — leave the attachment untouched.
		return data, 0, nil
	}

	type entry struct {
		name string
		body []byte
	}
	entries := make([]entry, 0, len(zr.File))
	stripped := 0

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue // skip directory entries; they are reconstructed implicitly
		}
		rc, err := f.Open()
		if err != nil {
			return data, 0, nil
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return data, 0, nil
		}
		if isXHTMLName(f.Name) {
			if matches := amznAttrRe.FindAll(body, -1); len(matches) > 0 {
				stripped += len(matches)
				body = amznAttrRe.ReplaceAll(body, nil)
			}
		}
		entries = append(entries, entry{name: f.Name, body: body})
	}

	if stripped == 0 {
		// Already clean — return the original bytes unchanged.
		return data, 0, nil
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	write := func(name string, body []byte, store bool) error {
		hdr := &zip.FileHeader{Name: name}
		if store {
			hdr.Method = zip.Store
		} else {
			hdr.Method = zip.Deflate
		}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, err = w.Write(body)
		return err
	}

	// EPUB requires `mimetype` to be the first entry and stored uncompressed.
	for _, e := range entries {
		if e.name == "mimetype" {
			if err := write(e.name, e.body, true); err != nil {
				return data, 0, err
			}
		}
	}
	for _, e := range entries {
		if e.name == "mimetype" {
			continue
		}
		if err := write(e.name, e.body, false); err != nil {
			return data, 0, err
		}
	}
	if err := zw.Close(); err != nil {
		return data, 0, err
	}

	return buf.Bytes(), stripped, nil
}
