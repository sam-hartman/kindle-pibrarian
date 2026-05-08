package goodreads

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/sam-hartman/kindle-pibrarian/internal/relay"
)

var (
	// goodreadsBase is the production base URL. It's a var (not const) so
	// tests can override it to point at a local httptest server.
	goodreadsBase = "https://www.goodreads.com"

	numericRe       = regexp.MustCompile(`^\d+$`)
	profileURLRe    = regexp.MustCompile(`goodreads\.com/user/show/(\d+)(?:-([^/?#]+))?`)
	reviewListURLRe = regexp.MustCompile(`goodreads\.com/review/list/(\d+)(?:-([^/?#]+))?`)
	looseUserIDRe   = regexp.MustCompile(`/user/show/(\d+)(?:-([^/?#]+))?`)
)

// ResolveUserID accepts a numeric ID, a profile URL, or a username and returns
// the canonical Goodreads user ID and display info.
func ResolveUserID(input string) (*ResolvedUser, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}

	if numericRe.MatchString(input) {
		return &ResolvedUser{
			UserID:     input,
			ProfileURL: goodreadsBase + "/user/show/" + input,
			Confidence: 1.0,
		}, nil
	}

	for _, re := range []*regexp.Regexp{profileURLRe, reviewListURLRe} {
		if m := re.FindStringSubmatch(input); m != nil {
			id := m[1]
			slug := ""
			if len(m) > 2 {
				slug = m[2]
			}
			return &ResolvedUser{
				UserID:      id,
				DisplayName: nameFromSlug(slug),
				ProfileURL:  goodreadsBase + "/user/show/" + id,
				Confidence:  1.0,
			}, nil
		}
	}

	// Treat as username. When the relay is configured, prefer routing
	// through it (Goodreads blocks Fly egress IPs).
	if _, _, ok := relay.Config(); ok {
		if u, err := resolveUsernameViaRelay(input); err == nil {
			return u, nil
		}
	} else {
		if u, err := resolveUsernameAt(goodreadsBase, input); err == nil {
			return u, nil
		}
	}
	// Fallback: low-confidence people search. The /search endpoint
	// returns a normal HTML page so it works through the relay too.
	if _, _, ok := relay.Config(); ok {
		if u, err := searchPeopleViaRelay(input); err == nil {
			return u, nil
		}
	} else {
		if u, err := searchPeopleAt(goodreadsBase, input); err == nil {
			return u, nil
		}
	}
	return nil, errors.New("could not resolve to a Goodreads user")
}

// resolveUsernameViaRelay does what resolveUsernameAt does, but routes
// the request through the Pi relay (X-Relay-Target: goodreads). The
// relay sets allow_redirects=False so the 301 with Location reaches us.
func resolveUsernameViaRelay(username string) (*ResolvedUser, error) {
	req, err := relay.NewRequest("GET", relay.TargetGoodreads, "/"+username, nil)
	if err != nil {
		return nil, err
	}
	resp, err := relay.Client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s via relay: %w", username, err)
	}
	defer resp.Body.Close()

	loc := resp.Header.Get("Location")
	if loc == "" {
		return nil, fmt.Errorf("no redirect for username %q (status %d)", username, resp.StatusCode)
	}
	if strings.HasPrefix(loc, "http://") || strings.HasPrefix(loc, "https://") {
		u, err := url.Parse(loc)
		if err != nil {
			return nil, fmt.Errorf("parse redirect %q: %w", loc, err)
		}
		host := strings.ToLower(u.Host)
		if host != "goodreads.com" && !strings.HasSuffix(host, ".goodreads.com") {
			return nil, fmt.Errorf("redirect to foreign host %q", u.Host)
		}
	} else if !strings.HasPrefix(loc, "/") {
		return nil, fmt.Errorf("unexpected redirect target %q", loc)
	}
	m := profileURLRe.FindStringSubmatch(loc)
	if m == nil {
		m = looseUserIDRe.FindStringSubmatch(loc)
	}
	if m == nil {
		return nil, fmt.Errorf("no user id in redirect %q", loc)
	}
	slug := ""
	if len(m) > 2 {
		slug = m[2]
	}
	return &ResolvedUser{
		UserID:      m[1],
		DisplayName: nameFromSlug(slug),
		ProfileURL:  goodreadsBase + "/user/show/" + m[1],
		Confidence:  1.0,
	}, nil
}

// searchPeopleViaRelay scrapes the people-search HTML through the relay.
func searchPeopleViaRelay(q string) (*ResolvedUser, error) {
	path := "/search?q=" + url.QueryEscape(q) + "&search_type=people"
	req, err := relay.NewRequest("GET", relay.TargetGoodreads, path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := relay.Client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("search people via relay: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("people search via relay returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read search body: %w", err)
	}
	m := searchUserRe.FindSubmatch(body)
	if m == nil {
		return nil, errors.New("no people search match")
	}
	id := string(m[1])
	slug := ""
	if len(m) > 2 {
		slug = string(m[2])
	}
	return &ResolvedUser{
		UserID:      id,
		DisplayName: nameFromSlug(slug),
		ProfileURL:  goodreadsBase + "/user/show/" + id,
		Confidence:  0.5,
	}, nil
}

var searchUserRe = regexp.MustCompile(`/user/show/(\d+)(?:-([^"'/?#]+))?`)

// searchPeopleAt scrapes the Goodreads people search and returns the first
// match as a low-confidence ResolvedUser.
func searchPeopleAt(base, q string) (*ResolvedUser, error) {
	u := base + "/search?q=" + url.QueryEscape(q) + "&search_type=people"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("search people: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("people search returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read search body: %w", err)
	}
	m := searchUserRe.FindSubmatch(body)
	if m == nil {
		return nil, errors.New("no people search match")
	}
	id := string(m[1])
	slug := ""
	if len(m) > 2 {
		slug = string(m[2])
	}
	return &ResolvedUser{
		UserID:      id,
		DisplayName: nameFromSlug(slug),
		ProfileURL:  goodreadsBase + "/user/show/" + id,
		Confidence:  0.5,
	}, nil
}

// resolveUsernameAt is the inner helper exposed for tests.
func resolveUsernameAt(base, username string) (*ResolvedUser, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(base + "/" + username)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", username, err)
	}
	defer resp.Body.Close()

	loc := resp.Header.Get("Location")
	if loc == "" {
		return nil, fmt.Errorf("no redirect for username %q", username)
	}
	if strings.HasPrefix(loc, "http://") || strings.HasPrefix(loc, "https://") {
		u, err := url.Parse(loc)
		if err != nil {
			return nil, fmt.Errorf("parse redirect %q: %w", loc, err)
		}
		host := strings.ToLower(u.Host)
		if host != "goodreads.com" && !strings.HasSuffix(host, ".goodreads.com") {
			return nil, fmt.Errorf("redirect to foreign host %q", u.Host)
		}
	} else if !strings.HasPrefix(loc, "/") {
		return nil, fmt.Errorf("unexpected redirect target %q", loc)
	}
	m := profileURLRe.FindStringSubmatch(loc)
	if m == nil {
		// Try a leading-slash variant: "/user/show/123"
		m = looseUserIDRe.FindStringSubmatch(loc)
	}
	if m == nil {
		return nil, fmt.Errorf("no user id in redirect %q", loc)
	}
	slug := ""
	if len(m) > 2 {
		slug = m[2]
	}
	return &ResolvedUser{
		UserID:      m[1],
		DisplayName: nameFromSlug(slug),
		ProfileURL:  goodreadsBase + "/user/show/" + m[1],
		Confidence:  1.0,
	}, nil
}

// nameFromSlug converts a Goodreads URL slug like "jane-doe" into a
// display name like "Jane Doe". If the slug is empty, returns "".
func nameFromSlug(slug string) string {
	if slug == "" {
		return ""
	}
	parts := strings.Split(slug, "-")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		for i := 1; i < len(runes); i++ {
			runes[i] = unicode.ToLower(runes[i])
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, " ")
}
