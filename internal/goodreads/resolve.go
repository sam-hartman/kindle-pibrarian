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
)

const goodreadsBase = "https://www.goodreads.com"

var numericRe = regexp.MustCompile(`^\d+$`)
var profileURLRe = regexp.MustCompile(`goodreads\.com/user/show/(\d+)(?:-([^/?#]+))?`)
var looseUserIDRe = regexp.MustCompile(`/user/show/(\d+)(?:-([^/?#]+))?`)

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

	if m := profileURLRe.FindStringSubmatch(input); m != nil {
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

	// Treat as username
	if u, err := resolveUsernameAt(goodreadsBase, input); err == nil {
		return u, nil
	}
	// Fallback: low-confidence people search.
	if u, err := searchPeopleAt(goodreadsBase, input); err == nil {
		return u, nil
	}
	return nil, errors.New("could not resolve to a Goodreads user")
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
