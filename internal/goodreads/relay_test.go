package goodreads

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sam-hartman/kindle-pibrarian/internal/relay"
)

func TestResolveUsernameViaRelay_HitsRelayWithHeaders(t *testing.T) {
	var (
		gotPath   atomic.Value
		gotSecret atomic.Value
		gotTarget atomic.Value
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath.Store(r.URL.Path)
		gotSecret.Store(r.Header.Get(relay.HeaderSecret))
		gotTarget.Store(r.Header.Get(relay.HeaderTarget))
		w.Header().Set("Location", "/user/show/1234567-jane-doe")
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer srv.Close()

	t.Setenv(relay.EnvBaseURL, srv.URL)
	t.Setenv(relay.EnvSecret, "supersecret")

	got, err := resolveUsernameViaRelay("janedoe")
	if err != nil {
		t.Fatalf("resolveUsernameViaRelay: %v", err)
	}
	if got.UserID != "1234567" {
		t.Errorf("UserID = %q", got.UserID)
	}
	if got.DisplayName != "Jane Doe" {
		t.Errorf("DisplayName = %q", got.DisplayName)
	}
	if v, _ := gotPath.Load().(string); v != "/janedoe" {
		t.Errorf("relay path = %q", v)
	}
	if v, _ := gotSecret.Load().(string); v != "supersecret" {
		t.Errorf("X-Relay-Secret = %q", v)
	}
	if v, _ := gotTarget.Load().(string); v != relay.TargetGoodreads {
		t.Errorf("X-Relay-Target = %q", v)
	}
}

func TestResolveUsernameViaRelay_RejectsForeignRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://evil.example.com/user/show/1234567-jane-doe")
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer srv.Close()
	t.Setenv(relay.EnvBaseURL, srv.URL)
	t.Setenv(relay.EnvSecret, "x")

	if _, err := resolveUsernameViaRelay("janedoe"); err == nil {
		t.Fatal("expected foreign-host rejection")
	}
}

func TestFetchShelfViaRelay_ParsesAndUsesRelay(t *testing.T) {
	rss := `<?xml version="1.0"?><rss><channel>
		<item><title>Book A</title><link>https://x</link><author_name>Auth</author_name></item>
	</channel></rss>`
	var gotPath atomic.Value
	var gotTarget atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath.Store(r.URL.Path + "?" + r.URL.RawQuery)
		gotTarget.Store(r.Header.Get(relay.HeaderTarget))
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rss))
	}))
	defer srv.Close()
	t.Setenv(relay.EnvBaseURL, srv.URL)
	t.Setenv(relay.EnvSecret, "x")

	got, err := fetchShelfViaRelay("1234567", "to-read")
	if err != nil {
		t.Fatalf("fetchShelfViaRelay: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Book A" {
		t.Fatalf("unexpected books: %+v", got)
	}
	p, _ := gotPath.Load().(string)
	if !strings.HasPrefix(p, "/review/list_rss/1234567") {
		t.Errorf("relay path = %q", p)
	}
	if !strings.Contains(p, "shelf=to-read") {
		t.Errorf("missing shelf query: %q", p)
	}
	if v, _ := gotTarget.Load().(string); v != relay.TargetGoodreads {
		t.Errorf("X-Relay-Target = %q", v)
	}
}

func TestFetchShelf_PrefersRelayWhenConfigured(t *testing.T) {
	rss := `<?xml version="1.0"?><rss><channel>
		<item><title>Relay Book</title><link>https://x</link><author_name>Auth</author_name></item>
	</channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rss))
	}))
	defer srv.Close()
	t.Setenv(relay.EnvBaseURL, srv.URL)
	t.Setenv(relay.EnvSecret, "x")

	// Clear cache so we actually fetch.
	shelfCacheMu.Lock()
	delete(shelfCache, "5555:to-read")
	shelfCacheMu.Unlock()
	t.Cleanup(func() {
		shelfCacheMu.Lock()
		delete(shelfCache, "5555:to-read")
		shelfCacheMu.Unlock()
	})

	got, err := FetchShelf("5555", "to-read")
	if err != nil {
		t.Fatalf("FetchShelf: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Relay Book" {
		t.Fatalf("got %+v", got)
	}
}
