// Package goodreads resolves Goodreads users by URL/username/ID and reads
// public shelf RSS feeds. It exists because the public Goodreads API was
// closed to new keys in December 2020.
package goodreads

// ShelfBook represents one entry from a Goodreads shelf RSS feed.
type ShelfBook struct {
	Title         string `json:"title"`
	Author        string `json:"author"`
	ISBN          string `json:"isbn,omitempty"`
	GoodreadsURL  string `json:"goodreads_url"`
	CoverURL      string `json:"cover_url,omitempty"`
	PublishedYear string `json:"published_year,omitempty"`
}

// ResolvedUser is the result of looking up a Goodreads user by URL/username/ID.
type ResolvedUser struct {
	UserID      string  `json:"user_id"`
	DisplayName string  `json:"display_name"`
	ProfileURL  string  `json:"profile_url"`
	Confidence  float64 `json:"confidence"` // 1.0 = exact (URL/ID/username redirect), 0.5 = search heuristic
}
