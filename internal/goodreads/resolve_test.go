package goodreads

import "testing"

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
