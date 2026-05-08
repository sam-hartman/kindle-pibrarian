package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setEnv(t *testing.T, base, secret string) {
	t.Helper()
	t.Setenv(EnvBaseURL, base)
	t.Setenv(EnvSecret, secret)
}

func TestConfig_NotConfigured(t *testing.T) {
	t.Setenv(EnvBaseURL, "")
	t.Setenv(EnvSecret, "")
	if _, _, ok := Config(); ok {
		t.Fatal("expected ok=false when env unset")
	}
}

func TestConfig_TrimsTrailingSlash(t *testing.T) {
	setEnv(t, "https://relay.example/", "s")
	base, secret, ok := Config()
	if !ok || base != "https://relay.example" || secret != "s" {
		t.Fatalf("got base=%q secret=%q ok=%v", base, secret, ok)
	}
}

func TestNewRequest_NotConfigured(t *testing.T) {
	t.Setenv(EnvBaseURL, "")
	t.Setenv(EnvSecret, "")
	if _, err := NewRequest("GET", TargetGoodreads, "/foo", nil); err == nil {
		t.Fatal("expected error when relay not configured")
	}
}

func TestNewRequest_GETSetsHeadersAndPath(t *testing.T) {
	setEnv(t, "https://relay.example", "topsecret")
	req, err := NewRequest("GET", TargetGoodreads, "cassandra", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if req.Method != "GET" {
		t.Errorf("Method = %q", req.Method)
	}
	if req.URL.String() != "https://relay.example/cassandra" {
		t.Errorf("URL = %q", req.URL.String())
	}
	if got := req.Header.Get(HeaderSecret); got != "topsecret" {
		t.Errorf("X-Relay-Secret = %q", got)
	}
	if got := req.Header.Get(HeaderTarget); got != TargetGoodreads {
		t.Errorf("X-Relay-Target = %q", got)
	}
	if got := req.Header.Get("Content-Type"); got != "" {
		t.Errorf("Content-Type should be empty for GET, got %q", got)
	}
}

func TestNewRequest_JSONBody(t *testing.T) {
	setEnv(t, "https://relay.example", "s")
	req, err := NewRequest("POST", TargetPiAnnasMCP, "/mcp", map[string]string{"hello": "world"})
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	body, _ := io.ReadAll(req.Body)
	if !strings.Contains(string(body), `"hello":"world"`) {
		t.Errorf("body = %q", body)
	}
}

func TestNewRequest_ByteBodyNoContentType(t *testing.T) {
	setEnv(t, "https://relay.example", "s")
	req, err := NewRequest("POST", TargetPiAnnasMCP, "/mcp", []byte("raw"))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if got := req.Header.Get("Content-Type"); got != "" {
		t.Errorf("Content-Type should be unset for raw bytes, got %q", got)
	}
}

func TestNewRequest_RejectsEmptyTarget(t *testing.T) {
	setEnv(t, "https://relay.example", "s")
	if _, err := NewRequest("GET", "", "/x", nil); err == nil {
		t.Fatal("expected error for empty target")
	}
}

func TestClient_DoesNotFollowRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			w.Header().Set("Location", "/redirected")
			w.WriteHeader(http.StatusMovedPermanently)
			return
		}
		t.Errorf("client followed redirect to %s", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := Client().Get(srv.URL + "/start")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Errorf("status = %d, want 301", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/redirected" {
		t.Errorf("Location = %q", loc)
	}
}
