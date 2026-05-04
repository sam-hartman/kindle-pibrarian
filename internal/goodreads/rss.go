package goodreads

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// fetchShelfAt is the inner helper exposed for tests; it takes a base URL
// instead of using the constant.
func fetchShelfAt(base, userID, shelf string) ([]ShelfBook, error) {
	u := fmt.Sprintf("%s/review/list_rss/%s?shelf=%s", base, url.PathEscape(userID), url.QueryEscape(shelf))
	resp, err := http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("fetch shelf: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("shelf fetch returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read shelf body: %w", err)
	}

	var feed struct {
		Channel struct {
			Items []struct {
				Title         string `xml:"title"`
				Link          string `xml:"link"`
				AuthorName    string `xml:"author_name"`
				ISBN          string `xml:"isbn"`
				BookPublished string `xml:"book_published"`
				BookImageURL  string `xml:"book_image_url"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse shelf RSS: %w", err)
	}

	books := make([]ShelfBook, 0, len(feed.Channel.Items))
	for _, it := range feed.Channel.Items {
		books = append(books, ShelfBook{
			Title:         it.Title,
			Author:        it.AuthorName,
			ISBN:          it.ISBN,
			GoodreadsURL:  it.Link,
			CoverURL:      it.BookImageURL,
			PublishedYear: it.BookPublished,
		})
	}
	return books, nil
}

// FetchShelf is the public entry point with a 10-minute in-process cache.
type cacheEntry struct {
	books   []ShelfBook
	expires time.Time
}

var (
	shelfCache   = map[string]cacheEntry{}
	shelfCacheMu sync.RWMutex
	cacheTTL     = 10 * time.Minute
)

func FetchShelf(userID, shelf string) ([]ShelfBook, error) {
	if shelf == "" {
		shelf = "to-read"
	}
	key := userID + ":" + shelf
	shelfCacheMu.RLock()
	if e, ok := shelfCache[key]; ok && time.Now().Before(e.expires) {
		shelfCacheMu.RUnlock()
		return e.books, nil
	}
	shelfCacheMu.RUnlock()

	books, err := fetchShelfAt(goodreadsBase, userID, shelf)
	if err != nil {
		return nil, err
	}
	shelfCacheMu.Lock()
	shelfCache[key] = cacheEntry{books: books, expires: time.Now().Add(cacheTTL)}
	shelfCacheMu.Unlock()
	return books, nil
}
