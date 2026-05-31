package anna

import "testing"

func TestNormalizeForMatch(t *testing.T) {
	cases := map[string]string{
		"The Deal":                     "the deal",
		"The Deal (Off-Campus Book 1)": "the deal off campus book 1",
		"  THE   DEAL!! ":              "the deal",
		"the_deal":                     "the deal",
	}
	for in, want := range cases {
		if got := normalizeForMatch(in); got != want {
			t.Errorf("normalizeForMatch(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAuthorsOverlap(t *testing.T) {
	// Same author, different formatting → overlap (safe to treat as same book).
	if !authorsOverlap("Elle Kennedy", "Kennedy, Elle") {
		t.Error("expected overlap for the same author in different order")
	}
	if !authorsOverlap("lgli/Elle Kennedy - The Deal.epub", "Elle Kennedy") {
		t.Error("expected overlap when author appears inside a messy path string")
	}
	// Different authors with the same title → NO overlap (must not auto-send).
	if authorsOverlap("Holly Hart", "Elle Kennedy") {
		t.Error("different authors must not overlap")
	}
	// Short initials should not create false matches.
	if authorsOverlap("J. K.", "J. R.") {
		t.Error("short initials must not overlap")
	}
}
