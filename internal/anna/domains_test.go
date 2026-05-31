package anna

import (
	"reflect"
	"testing"
)

func TestAnnasBases_DefaultWhenUnset(t *testing.T) {
	t.Setenv("ANNAS_BASE_URLS", "")
	got := annasBases()
	if !reflect.DeepEqual(got, defaultAnnasBases) {
		t.Fatalf("expected default mirrors %v, got %v", defaultAnnasBases, got)
	}
	if got[0] != "https://annas-archive.gl" {
		t.Fatalf("expected a live mirror first, got %q", got[0])
	}
}

func TestAnnasBases_EnvOverrideOrderedAndTrimmed(t *testing.T) {
	t.Setenv("ANNAS_BASE_URLS", " https://m1.example/ , https://m2.example ,, ")
	got := annasBases()
	want := []string{"https://m1.example", "https://m2.example"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestAnnasURLBuilders(t *testing.T) {
	if got := annasSearchURL("https://m.example", "the deal"); got != "https://m.example/search?q=the+deal" {
		t.Fatalf("search URL: %q", got)
	}
	got := annasDownloadURL("https://m.example", "abc123", "secret", 2)
	want := "https://m.example/dyn/api/fast_download.json?md5=abc123&key=secret&domain_index=2"
	if got != want {
		t.Fatalf("download URL: got %q want %q", got, want)
	}
}
