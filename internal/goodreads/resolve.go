package goodreads

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const goodreadsBase = "https://www.goodreads.com"

var numericRe = regexp.MustCompile(`^\d+$`)
var profileURLRe = regexp.MustCompile(`goodreads\.com/user/show/(\d+)`)
var looseUserIDRe = regexp.MustCompile(`/user/show/(\d+)`)

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
		return &ResolvedUser{
			UserID:     id,
			ProfileURL: goodreadsBase + "/user/show/" + id,
			Confidence: 1.0,
		}, nil
	}

	// Treat as username
	if u, err := resolveUsernameAt(goodreadsBase, input); err == nil {
		return u, nil
	}
	return nil, errors.New("could not resolve to a Goodreads user")
}

// resolveUsernameAt is the inner helper exposed for tests.
func resolveUsernameAt(base, username string) (*ResolvedUser, error) {
	client := &http.Client{
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
	return &ResolvedUser{
		UserID:     m[1],
		ProfileURL: goodreadsBase + "/user/show/" + m[1],
		Confidence: 1.0,
	}, nil
}
