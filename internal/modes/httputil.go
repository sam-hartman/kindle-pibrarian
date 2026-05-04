package modes

import (
	"encoding/json"
	"net/http"
)

// writeJSONError writes a JSON error envelope with the given HTTP status.
// The message is intended to be safe for client display (no upstream details).
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// truncate returns s shortened to at most n runes, suitable for log fields
// that should not blow up if a caller passes huge input.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	return s[:n]
}
