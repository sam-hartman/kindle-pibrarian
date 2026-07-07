package anna

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// makeMinimalEPUB builds the smallest byte blob that isEPUBZip accepts, so tests
// don't depend on a real Calibre install.
func makeMinimalEPUB(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	mt, _ := zw.Create("mimetype")
	mt.Write([]byte("application/epub+zip"))
	c, _ := zw.Create("META-INF/container.xml")
	c.Write([]byte(`<?xml version="1.0"?><container/>`))
	if err := zw.Close(); err != nil {
		t.Fatalf("build epub: %v", err)
	}
	return buf.Bytes()
}

// writeStubConverter writes an executable shell script that mimics the parts of
// `ebook-convert <in> <out> ...` we rely on: it writes `body` to the out path
// (argv[2]). Returns the script path.
func writeStubConverter(t *testing.T, body []byte) string {
	t.Helper()
	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture.bin")
	if err := os.WriteFile(fixture, body, 0600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "ebook-convert-stub")
	sh := "#!/bin/sh\ncp \"" + fixture + "\" \"$2\"\n"
	if err := os.WriteFile(script, []byte(sh), 0700); err != nil {
		t.Fatal(err)
	}
	return script
}

func TestPDFConverterAvailable_False(t *testing.T) {
	orig := pdfConverterCmd
	t.Cleanup(func() { pdfConverterCmd = orig })
	pdfConverterCmd = "definitely-not-a-real-binary-xyz"
	if PDFConverterAvailable() {
		t.Error("expected converter to be unavailable")
	}
}

func TestConvertPDFToEPUB_UnavailableFallsBack(t *testing.T) {
	orig := pdfConverterCmd
	t.Cleanup(func() { pdfConverterCmd = orig })
	pdfConverterCmd = "definitely-not-a-real-binary-xyz"
	if _, err := ConvertPDFToEPUB([]byte("%PDF-1.4 ...")); err != ErrConverterUnavailable {
		t.Errorf("want ErrConverterUnavailable, got %v", err)
	}
}

func TestConvertPDFToEPUB_EmptyInput(t *testing.T) {
	if _, err := ConvertPDFToEPUB(nil); err == nil {
		t.Error("expected error for empty input")
	}
}

func TestConvertPDFToEPUB_Success(t *testing.T) {
	orig := pdfConverterCmd
	t.Cleanup(func() { pdfConverterCmd = orig })
	pdfConverterCmd = writeStubConverter(t, makeMinimalEPUB(t))

	got, err := ConvertPDFToEPUB([]byte("%PDF-1.4 fake pdf bytes"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isEPUBZip(got) {
		t.Error("converted output is not a valid epub")
	}
}

func TestConvertPDFToEPUB_RejectsNonEPUBOutput(t *testing.T) {
	orig := pdfConverterCmd
	t.Cleanup(func() { pdfConverterCmd = orig })
	// Stub emits junk (a text-only/image-only PDF can yield a degenerate result).
	pdfConverterCmd = writeStubConverter(t, []byte("not an epub at all"))

	if _, err := ConvertPDFToEPUB([]byte("%PDF-1.4 fake")); err == nil {
		t.Error("expected error when conversion output is not a valid epub")
	}
}
