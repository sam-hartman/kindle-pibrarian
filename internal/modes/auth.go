package modes

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

// RequirePasscode is HTTP middleware that gates all non-public requests behind
// a shared passcode supplied via the WEB_PASSCODE environment variable.
//
// Behavior:
//   - If WEB_PASSCODE is empty/unset, the middleware is a no-op (passes through).
//   - GET /health and GET / are always allowed (used by health checks and discovery).
//   - OPTIONS requests always pass through so the downstream CORS handler can
//     respond to preflight.
//   - All other requests must include "Authorization: Bearer <passcode>".
//     The comparison uses crypto/subtle.ConstantTimeCompare to avoid timing
//     leaks. Mismatches receive 401 Unauthorized.
func RequirePasscode(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		passcode := os.Getenv("WEB_PASSCODE")
		if passcode == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Always allow CORS preflight; downstream CORS middleware sets headers.
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Always allow health and root for discovery / liveness probes.
		if r.Method == http.MethodGet && (r.URL.Path == "/health" || r.URL.Path == "/") {
			next.ServeHTTP(w, r)
			return
		}

		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		supplied := header[len(prefix):]
		if subtle.ConstantTimeCompare([]byte(supplied), []byte(passcode)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// WithCORS is HTTP middleware that applies a permissive CORS policy suitable
// for a personal web app talking to this backend from any origin (the passcode
// is the actual access control).
//
// Behavior:
//   - Sets Access-Control-Allow-Origin: *
//   - Sets Access-Control-Allow-Methods: GET, POST, OPTIONS
//   - Sets Access-Control-Allow-Headers: Authorization, Content-Type
//   - Sets Access-Control-Max-Age: 86400
//   - For OPTIONS preflight, short-circuits with 204 No Content.
func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
