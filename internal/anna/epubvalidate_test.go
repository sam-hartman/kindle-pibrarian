package anna

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// makeZip builds a zip from name->content; mimetype (if present) is stored first.
func makeZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if mt, ok := entries["mimetype"]; ok {
		w, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
		w.Write([]byte(mt))
	}
	for name, body := range entries {
		if name == "mimetype" {
			continue
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func validEPUBEntries() map[string]string {
	return map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="id"><metadata><dc:identifier xmlns:dc="http://purl.org/dc/elements/1.1/" id="id">x</dc:identifier></metadata><manifest/><spine/></package>`,
		"OEBPS/Text/01.xhtml":    `<html><body><p>hi</p></body></html>`,
	}
}

func TestIsEPUBZip(t *testing.T) {
	if !isEPUBZip(makeZip(t, validEPUBEntries())) {
		t.Fatal("a real EPUB should be recognized as an EPUB zip")
	}
	// A plain zip (e.g. CBZ) with no mimetype/container is not an EPUB.
	if isEPUBZip(makeZip(t, map[string]string{"pages/1.jpg": "notreallyajpg"})) {
		t.Fatal("a non-EPUB zip must NOT be treated as an EPUB")
	}
	if isEPUBZip([]byte("not a zip")) {
		t.Fatal("non-zip must not be an EPUB")
	}
}

func TestDetectFileFormat_NonEpubZipIsUnknown(t *testing.T) {
	cbz := makeZip(t, map[string]string{"001.jpg": "data", "002.jpg": "data"})
	if f, _ := detectFileFormat("", cbz); f != "unknown" {
		t.Fatalf("a non-EPUB zip should detect as unknown, got %q", f)
	}
	if f, _ := detectFileFormat("", makeZip(t, validEPUBEntries())); f != "epub" {
		t.Fatalf("a real EPUB should detect as epub, got %q", f)
	}
	if f, _ := detectFileFormat("", []byte("%PDF-1.4 ...")); f != "pdf" {
		t.Fatalf("PDF magic should detect as pdf, got %q", f)
	}
}

func TestValidateEPUB_Valid(t *testing.T) {
	if err := ValidateEPUB(makeZip(t, validEPUBEntries())); err != nil {
		t.Fatalf("valid EPUB should pass, got: %v", err)
	}
}

func TestValidateEPUB_RejectsDRM(t *testing.T) {
	e := validEPUBEntries()
	e["META-INF/encryption.xml"] = `<encryption/>`
	err := ValidateEPUB(makeZip(t, e))
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "drm") {
		t.Fatalf("DRM'd EPUB should be rejected with a DRM message, got: %v", err)
	}
}

func TestValidateEPUB_RejectsNonZipAndMissingParts(t *testing.T) {
	if err := ValidateEPUB([]byte("not a zip")); err == nil {
		t.Fatal("non-zip should be rejected")
	}
	noMime := validEPUBEntries()
	delete(noMime, "mimetype")
	if err := ValidateEPUB(makeZip(t, noMime)); err == nil {
		t.Fatal("EPUB missing mimetype should be rejected")
	}
	noOPF := validEPUBEntries()
	delete(noOPF, "OEBPS/content.opf")
	if err := ValidateEPUB(makeZip(t, noOPF)); err == nil {
		t.Fatal("EPUB whose container points at a missing OPF should be rejected")
	}
}
