// Package relay routes outbound requests through Sam's Pi relay
// (resy-relay) when configured via environment variables.
//
// The relay sits behind a Tailscale Funnel and forwards requests to a
// limited set of upstream targets identified by the X-Relay-Target
// header. Authentication is via a shared secret in the X-Relay-Secret
// header. The relay forwards path + query string + body to the upstream
// as-is, so callers should construct the path/query exactly as the
// upstream expects.
//
// When the relay is not configured (env vars unset), Config returns
// ok=false and callers should fall back to direct requests. This keeps
// the same binary usable both on Sam's Pi (direct) and on Fly (relay).
package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// EnvBaseURL is the env var holding the relay's base URL, e.g.
	// https://raspberrypi.tailddbc27.ts.net:8443
	EnvBaseURL = "UPSTREAM_RELAY_URL"

	// EnvSecret is the env var holding the X-Relay-Secret value.
	EnvSecret = "UPSTREAM_RELAY_SECRET"

	// HeaderSecret is the relay auth header.
	HeaderSecret = "X-Relay-Secret"

	// HeaderTarget is the upstream target selector header.
	HeaderTarget = "X-Relay-Target"

	// TargetPiAnnasMCP forwards to the Pi's annas-mcp HTTP server.
	TargetPiAnnasMCP = "pi-annas-mcp"

	// TargetGoodreads forwards to https://www.goodreads.com via
	// curl_cffi chrome131 impersonation, with allow_redirects=False.
	TargetGoodreads = "goodreads"

	// defaultTimeout: Anna's search through the relay can be slow.
	defaultTimeout = 60 * time.Second
)

// Config returns (baseURL, secret, true) when the relay is configured.
// Returns ("", "", false) when either env var is empty so callers can
// fall back to direct requests.
func Config() (baseURL, secret string, ok bool) {
	base := strings.TrimRight(os.Getenv(EnvBaseURL), "/")
	s := os.Getenv(EnvSecret)
	if base == "" || s == "" {
		return "", "", false
	}
	return base, s, true
}

// Client returns an *http.Client suitable for relay calls.
// Redirects are NOT followed because some targets (e.g. Goodreads
// username -> /user/show/<id>) need the raw 301 Location header.
func Client() *http.Client {
	return &http.Client{
		Timeout: defaultTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// NewRequest builds a relay request.
//
//	method        - HTTP method (GET, POST, ...)
//	target        - one of TargetPiAnnasMCP / TargetGoodreads
//	pathAndQuery  - path (and optional query string) the upstream should
//	                receive, with or without a leading slash
//	body          - optional body. If a []byte or io.Reader, sent as-is.
//	                Otherwise JSON-marshalled and Content-Type set to
//	                application/json.
//
// Returns an error if the relay is not configured.
func NewRequest(method, target, pathAndQuery string, body interface{}) (*http.Request, error) {
	base, secret, ok := Config()
	if !ok {
		return nil, fmt.Errorf("relay not configured (set %s and %s)", EnvBaseURL, EnvSecret)
	}
	if target == "" {
		return nil, fmt.Errorf("relay target must not be empty")
	}

	// Normalize path: ensure exactly one slash between base and path.
	if !strings.HasPrefix(pathAndQuery, "/") {
		pathAndQuery = "/" + pathAndQuery
	}
	url := base + pathAndQuery

	bodyReader, contentType, err := encodeBody(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build relay request: %w", err)
	}
	req.Header.Set(HeaderSecret, secret)
	req.Header.Set(HeaderTarget, target)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

// encodeBody returns an io.Reader plus a content type. nil body returns
// (nil, "", nil).
func encodeBody(body interface{}) (io.Reader, string, error) {
	if body == nil {
		return nil, "", nil
	}
	switch v := body.(type) {
	case nil:
		return nil, "", nil
	case []byte:
		return bytes.NewReader(v), "", nil
	case string:
		return strings.NewReader(v), "", nil
	case io.Reader:
		return v, "", nil
	default:
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, "", fmt.Errorf("marshal relay body: %w", err)
		}
		return bytes.NewReader(buf), "application/json", nil
	}
}
