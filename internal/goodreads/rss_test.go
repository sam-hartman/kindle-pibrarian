package goodreads

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func TestFetchShelf_WritesCacheOnFirstFetch(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	origBase := goodreadsBase
	goodreadsBase = srv.URL
	t.Cleanup(func() {
		goodreadsBase = origBase
		shelfCacheMu.Lock()
		delete(shelfCache, "11111:to-read")
		shelfCacheMu.Unlock()
	})

	shelfCacheMu.Lock()
	delete(shelfCache, "11111:to-read")
	shelfCacheMu.Unlock()

	if _, err := FetchShelf("11111", "to-read"); err != nil {
		t.Fatalf("first FetchShelf error: %v", err)
	}
	if _, err := FetchShelf("11111", "to-read"); err != nil {
		t.Fatalf("second FetchShelf error: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("upstream request count = %d, want 1 (second call should hit cache)", got)
	}
}

func TestFetchShelf_RejectsNonNumericUserID(t *testing.T) {
	got, err := FetchShelf("not-a-number", "to-read")
	if err == nil {
		t.Fatalf("expected error for non-numeric user id, got nil")
	}
	if got != nil {
		t.Errorf("expected nil books, got %+v", got)
	}
}

func TestFetchShelf_CachesAcrossCalls(t *testing.T) {
	shelfCacheMu.Lock()
	shelfCache["99999:to-read"] = cacheEntry{
		books:   []ShelfBook{{Title: "Cached"}},
		expires: time.Now().Add(5 * time.Minute),
	}
	shelfCacheMu.Unlock()
	t.Cleanup(func() {
		shelfCacheMu.Lock()
		delete(shelfCache, "99999:to-read")
		shelfCacheMu.Unlock()
	})

	got, err := FetchShelf("99999", "to-read")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Cached" {
		t.Errorf("got %+v, want one cached book", got)
	}
}
