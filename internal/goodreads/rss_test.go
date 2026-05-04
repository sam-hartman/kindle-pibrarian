package goodreads

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Jane's bookshelf: to-read</title>
    <item>
      <title>Project Hail Mary</title>
      <link>https://www.goodreads.com/book/show/54493401-project-hail-mary</link>
      <author_name>Andy Weir</author_name>
      <isbn>0593135202</isbn>
      <book_published>2021</book_published>
      <book_image_url>https://images.example/cover.jpg</book_image_url>
    </item>
    <item>
      <title>The Three-Body Problem</title>
      <link>https://www.goodreads.com/book/show/20518872-the-three-body-problem</link>
      <author_name>Liu Cixin</author_name>
      <isbn>0765382032</isbn>
      <book_published>2008</book_published>
    </item>
  </channel>
</rss>`

func TestFetchShelf_ParsesRSS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	got, err := fetchShelfAt(srv.URL, "1234567", "to-read")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2", len(got))
	}
	if got[0].Title != "Project Hail Mary" || got[0].Author != "Andy Weir" {
		t.Errorf("first item: %+v", got[0])
	}
	if got[0].ISBN != "0593135202" || got[0].PublishedYear != "2021" {
		t.Errorf("first item meta: %+v", got[0])
	}
	if got[0].CoverURL != "https://images.example/cover.jpg" {
		t.Errorf("first item cover: %q", got[0].CoverURL)
	}
}

func TestFetchShelf_Caps100(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n<rss version=\"2.0\"><channel>")
	for i := 0; i < 105; i++ {
		fmt.Fprintf(&b, `<item><title>Book %d</title><link>https://x/%d</link><author_name>A</author_name></item>`, i, i)
	}
	b.WriteString("</channel></rss>")
	feed := b.String()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(feed))
	}))
	defer srv.Close()

	got, err := fetchShelfAt(srv.URL, "1234567", "to-read")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 100 {
		t.Errorf("len(got) = %d, want 100", len(got))
	}
}
