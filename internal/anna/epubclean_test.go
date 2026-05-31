package anna

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

// buildEPUB creates a minimal in-memory EPUB. The first entry is the stored
// `mimetype`, followed by the supplied XHTML body as OEBPS/Text/01.xhtml.
func buildEPUB(t *testing.T, xhtml string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	mt, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatalf("create mimetype: %v", err)
	}
	if _, err := mt.Write([]byte("application/epub+zip")); err != nil {
		t.Fatalf("write mimetype: %v", err)
	}

	add := func(name, body string) {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	add("META-INF/container.xml", `<?xml version="1.0"?><container/>`)
	add("OEBPS/Text/01.xhtml", xhtml)

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func readEntry(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer rc.Close()
			b, _ := io.ReadAll(rc)
			return b
		}
	}
	t.Fatalf("entry %s not found", name)
	return nil
}

func TestSanitizeEPUB_StripsAmznAttributes(t *testing.T) {
	dirty := `<html><body><p data-AmznRemoved="a1.1" class="x">hi</p>` +
		`<span data-AmznRemoved-Style="b">there</span></body></html>`
	in := buildEPUB(t, dirty)

	out, n, err := SanitizeEPUB(in)
	if err != nil {
		t.Fatalf("SanitizeEPUB error: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 attributes stripped, got %d", n)
	}

	got := readEntry(t, out, "OEBPS/Text/01.xhtml")
	if bytes.Contains(bytes.ToLower(got), []byte("data-amzn")) {
		t.Fatalf("data-Amzn* still present after sanitize: %s", got)
	}
	// Surrounding markup must be preserved.
	if !bytes.Contains(got, []byte(`class="x"`)) || !bytes.Contains(got, []byte("hi")) {
		t.Fatalf("legitimate markup was damaged: %s", got)
	}
}

func TestSanitizeEPUB_MimetypeFirstAndStored(t *testing.T) {
	in := buildEPUB(t, `<html><body><p data-AmznRemoved="z">x</p></body></html>`)
	out, _, err := SanitizeEPUB(in)
	if err != nil {
		t.Fatalf("SanitizeEPUB error: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen zip: %v", err)
	}
	if len(zr.File) == 0 || zr.File[0].Name != "mimetype" {
		t.Fatalf("mimetype must be the first entry, got %v", zr.File)
	}
	if zr.File[0].Method != zip.Store {
		t.Fatalf("mimetype must be stored (method %d), got method %d", zip.Store, zr.File[0].Method)
	}
	if mt := readEntry(t, out, "mimetype"); string(mt) != "application/epub+zip" {
		t.Fatalf("mimetype content corrupted: %q", mt)
	}
}

func TestSanitizeEPUB_CleanFileUnchanged(t *testing.T) {
	in := buildEPUB(t, `<html><body><p class="ok">clean</p></body></html>`)
	out, n, err := SanitizeEPUB(in)
	if err != nil {
		t.Fatalf("SanitizeEPUB error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 strips on clean file, got %d", n)
	}
	if !bytes.Equal(in, out) {
		t.Fatal("clean EPUB bytes should be returned unchanged (idempotent)")
	}
}

func TestSanitizeEPUB_NonZipReturnedAsIs(t *testing.T) {
	in := []byte("this is not a zip at all")
	out, n, err := SanitizeEPUB(in)
	if err != nil {
		t.Fatalf("unexpected error on non-zip: %v", err)
	}
	if n != 0 || !bytes.Equal(in, out) {
		t.Fatalf("non-zip input must be returned untouched (n=%d, equal=%v)", n, bytes.Equal(in, out))
	}
}
