package goodreads

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveUserID_NumericInput(t *testing.T) {
	got, err := ResolveUserID("1234567")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "1234567" {
		t.Errorf("UserID = %q, want %q", got.UserID, "1234567")
	}
	if got.Confidence != 1.0 {
		t.Errorf("Confidence = %v, want 1.0", got.Confidence)
	}
	if got.ProfileURL != "https://www.goodreads.com/user/show/1234567" {
		t.Errorf("ProfileURL = %q", got.ProfileURL)
	}
}

func TestResolveUserID_ProfileURL(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"https://www.goodreads.com/user/show/1234567-jane-doe", "1234567"},
		{"https://goodreads.com/user/show/9876543", "9876543"},
		{"http://www.goodreads.com/user/show/42-douglas", "42"},
	}
	for _, tc := range cases {
		got, err := ResolveUserID(tc.input)
		if err != nil {
			t.Errorf("input=%q: unexpected error: %v", tc.input, err)
			continue
		}
		if got.UserID != tc.want {
			t.Errorf("input=%q: UserID = %q, want %q", tc.input, got.UserID, tc.want)
		}
		if got.Confidence != 1.0 {
			t.Errorf("input=%q: Confidence = %v, want 1.0", tc.input, got.Confidence)
		}
	}
}

func TestResolveUserID_UsernameRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/janedoe" {
			w.Header().Set("Location", "/user/show/1234567-jane-doe")
			w.WriteHeader(http.StatusMovedPermanently)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	got, err := resolveUsernameAt(srv.URL, "janedoe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "1234567" {
		t.Errorf("UserID = %q, want 1234567", got.UserID)
	}
	if got.Confidence != 1.0 {
		t.Errorf("Confidence = %v, want 1.0", got.Confidence)
	}
}
