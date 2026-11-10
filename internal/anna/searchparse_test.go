package anna

import (
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// parseFixture mirrors FindBookWithFormat's loop over /md5/ links.
func parseFixture(t *testing.T, path string) []*Book {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var out []*Book
	doc.Find("a[href^='/md5/']").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		hash := strings.TrimPrefix(href, "/md5/")
		if seen[hash] || isRecentDownloadsLink(a) {
			return
		}
		seen[hash] = true
		b, ok := parseResultRow(a)
		if !ok {
			t.Errorf("link %s not parsed as a result row", hash)
			return
		}
		out = append(out, b)
	})
	return out
}

func TestParseSearchFixture(t *testing.T) {
	books := parseFixture(t, "testdata/search_moby_dick.html")
	if len(books) != 7 {
		t.Fatalf("got %d results, want 7 (recent-downloads ticker must be skipped)", len(books))
	}

	first := books[0]
	want := Book{
		Title:     "Moby Dick: Or, The Whale",
		Authors:   "Enriched eBook Features Editor MAry K. Bercaw Edwards",
		Publisher: "Penguin USA, Inc., New York, New York State, 2009",
		Language:  "English",
		Format:    "epub",
		Size:      "1.5MB",
		Hash:      "05ff6931d11746bc699307f7a7595b6a",
	}
	if *first != want {
		t.Errorf("first result:\n got %+v\nwant %+v", *first, want)
	}

	for _, b := range books {
		if b.Title == "" || b.Format == "" || b.Size == "" || b.Language == "" || len(b.Hash) != 32 {
			t.Errorf("incomplete result: %+v", *b)
		}
		if strings.Contains(b.Authors, "/") || strings.Contains(b.Authors, "\n") {
			t.Errorf("authors look like a file path: %q", b.Authors)
		}
	}

	var melville, multiLang bool
	for _, b := range books {
		if strings.Contains(b.Authors, "Melville") {
			melville = true
		}
		if strings.Contains(b.Language, ", ") {
			multiLang = true
		}
	}
	if !melville {
		t.Error("expected at least one result with Melville as author")
	}
	if !multiLang {
		t.Error("expected a multi-language result joined with \", \"")
	}
}

func TestParseMetaLine(t *testing.T) {
	cases := []struct{ in, lang, format, size string }{
		{"✅ English [en] · EPUB · 1.5MB · 2009 · 📕 Book (fiction) · 🚀/lgli/zlib · ", "English", "epub", "1.5MB"},
		{"English [en] · Dutch [nl] · EPUB · 0.2MB · 2011 · Book", "English, Dutch", "epub", "0.2MB"},
		{"Portuguese [pt] · English [en] · AZW3 · 1.3MB · 2022", "Portuguese, English", "azw3", "1.3MB"},
		{"English [en] · FB2 · 1.3MB · Book (fiction)", "English", "fb2", "1.3MB"},
	}
	for _, c := range cases {
		l, f, s := parseMetaLine(c.in)
		if l != c.lang || f != c.format || s != c.size {
			t.Errorf("parseMetaLine(%q) = %q,%q,%q want %q,%q,%q", c.in, l, f, s, c.lang, c.format, c.size)
		}
	}
}
