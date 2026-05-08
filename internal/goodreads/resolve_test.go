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
		input           string
		want            string
		wantDisplayName string
	}{
		{"https://www.goodreads.com/user/show/1234567-jane-doe", "1234567", "Jane Doe"},
		{"https://goodreads.com/user/show/9876543", "9876543", ""},
		{"http://www.goodreads.com/user/show/42-douglas", "42", "Douglas"},
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
		if got.DisplayName != tc.wantDisplayName {
			t.Errorf("input=%q: DisplayName = %q, want %q", tc.input, got.DisplayName, tc.wantDisplayName)
		}
	}
}

func TestResolveUserID_ReviewListURL(t *testing.T) {
	cases := []struct {
		input           string
		want            string
		wantDisplayName string
	}{
		{"https://www.goodreads.com/review/list/170950204?shelf=to-read", "170950204", ""},
		{"https://www.goodreads.com/review/list/170950204-sam-hartman", "170950204", "Sam Hartman"},
		{"https://www.goodreads.com/review/list/170950204-sam-hartman?shelf=to-read", "170950204", "Sam Hartman"},
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
		if got.DisplayName != tc.wantDisplayName {
			t.Errorf("input=%q: DisplayName = %q, want %q", tc.input, got.DisplayName, tc.wantDisplayName)
		}
		if got.ProfileURL != "https://www.goodreads.com/user/show/"+tc.want {
			t.Errorf("input=%q: ProfileURL = %q", tc.input, got.ProfileURL)
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
	if got.DisplayName != "Jane Doe" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Jane Doe")
	}
}

func TestResolveUserID_RejectsForeignRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://evil.example.com/user/show/1234567-jane-doe")
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer srv.Close()

	got, err := resolveUsernameAt(srv.URL, "janedoe")
	if err == nil {
		t.Fatalf("expected error for foreign redirect, got nil")
	}
	if got != nil {
		t.Errorf("expected nil ResolvedUser, got %+v", got)
	}
}

func TestResolveUserID_SearchFallback(t *testing.T) {
	html := `<html><body>
		<div class="result"><a class="userReview" href="/user/show/9876-found-person">Found Person</a></div>
	</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	got, err := searchPeopleAt(srv.URL, "found person")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "9876" {
		t.Errorf("UserID = %q, want 9876", got.UserID)
	}
	if got.Confidence != 0.5 {
		t.Errorf("Confidence = %v, want 0.5", got.Confidence)
	}
	if got.DisplayName != "Found Person" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Found Person")
	}
}
