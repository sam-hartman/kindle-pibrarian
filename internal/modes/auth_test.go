package modes

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler is a minimal handler used by middleware tests.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestRequirePasscode_AllowsHealthAndRoot(t *testing.T) {
	t.Setenv("WEB_PASSCODE", "secret")
	h := RequirePasscode(okHandler())

	for _, path := range []string{"/health", "/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("path %s: expected 200, got %d", path, rec.Code)
		}
	}
}

func TestRequirePasscode_BlocksMissingHeader(t *testing.T) {
	t.Setenv("WEB_PASSCODE", "secret")
	h := RequirePasscode(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/goodreads/resolve", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequirePasscode_AllowsCorrectHeader(t *testing.T) {
	t.Setenv("WEB_PASSCODE", "secret")
	h := RequirePasscode(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/goodreads/resolve", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequirePasscode_NoOpWhenEnvUnset(t *testing.T) {
	t.Setenv("WEB_PASSCODE", "")
	h := RequirePasscode(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/goodreads/resolve", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (no-op), got %d", rec.Code)
	}
}

func TestWithCORS_PreflightShortCircuits(t *testing.T) {
	// Downstream handler should NOT be invoked for OPTIONS preflight.
	called := false
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	h := WithCORS(downstream)

	req := httptest.NewRequest(http.MethodOptions, "/goodreads/resolve", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected ACAO=*, got %q", got)
	}
	if called {
		t.Errorf("downstream handler should not be called for OPTIONS preflight")
	}
}

func TestWithCORS_PassThroughSetsHeaders(t *testing.T) {
	h := WithCORS(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected ACAO=*, got %q", got)
	}
}
