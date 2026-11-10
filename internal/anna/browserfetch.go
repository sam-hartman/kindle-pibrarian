package anna

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// browserFetchURL returns the base URL of the warm-browser fetcher service
// (BROWSER_FETCH_URL, e.g. http://172.17.0.1:8083). Anna's Archive sits behind
// a DDoS-Guard JavaScript challenge that plain HTTP clients cannot pass, so
// when this is set, search page loads go through a real browser instead.
func browserFetchURL() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("BROWSER_FETCH_URL")), "/")
}

// browserTransport is an http.RoundTripper that sends each request to the
// browser fetcher (GET {fetcher}/fetch?url=<target>) and returns the rendered
// HTML as the response body. The request URL is untouched, so colly still
// resolves relative links against the original mirror.
type browserTransport struct {
	fetcher string
	client  *http.Client
}

func (t *browserTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.client.Get(t.fetcher + "/fetch?url=" + url.QueryEscape(req.URL.String()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("browser fetcher returned %d: %.200s", resp.StatusCode, body)
	}
	return &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}, nil
}

// searchHTTPClient returns the client used for search scraping: the browser
// fetcher when BROWSER_FETCH_URL is set, otherwise a direct client.
func searchHTTPClient() *http.Client {
	if f := browserFetchURL(); f != "" {
		return &http.Client{
			Timeout:   60 * time.Second,
			Transport: &browserTransport{fetcher: f, client: &http.Client{Timeout: 55 * time.Second}},
		}
	}
	return &http.Client{Timeout: 30 * time.Second}
}
