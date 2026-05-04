package goodreads

import (
	"fmt"
	"regexp"
	"strings"
)

var numericRe = regexp.MustCompile(`^\d+$`)

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
			ProfileURL: "https://www.goodreads.com/user/show/" + input,
			Confidence: 1.0,
		}, nil
	}

	return nil, fmt.Errorf("not yet implemented for non-numeric input")
}
