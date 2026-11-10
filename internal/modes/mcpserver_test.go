package modes

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The HTTP JSON-RPC download handler previously dropped the "author" argument,
// which disabled the safe alternate-edition fallback on the Pi (it only runs
// when the author is known). These tests lock in that author now survives the
// parse.
func TestParseDownloadArgs_IncludesAuthorAndEmail(t *testing.T) {
	dp, err := parseDownloadArgs(map[string]interface{}{
		"hash":         "abc123",
		"title":        "The Deal",
		"format":       "epub",
		"author":       "Elle Kennedy",
		"kindle_email": "reader_x@kindle.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dp.BookHash != "abc123" {
		t.Errorf("hash = %q, want %q", dp.BookHash, "abc123")
	}
	if dp.Author != "Elle Kennedy" {
		t.Errorf("author dropped: got %q, want %q", dp.Author, "Elle Kennedy")
	}
	if dp.KindleEmail != "reader_x@kindle.com" {
		t.Errorf("kindle_email = %q, want %q", dp.KindleEmail, "reader_x@kindle.com")
	}
}

func TestParseDownloadArgs_AuthorOptional(t *testing.T) {
	dp, err := parseDownloadArgs(map[string]interface{}{
		"hash":   "abc123",
		"title":  "The Deal",
		"format": "epub",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dp.Author != "" {
		t.Errorf("author should default to empty, got %q", dp.Author)
	}
}

func TestParseDownloadArgs_MissingRequired(t *testing.T) {
	for name, args := range map[string]map[string]interface{}{
		"no hash":   {"title": "T", "format": "epub"},
		"no title":  {"hash": "h", "format": "epub"},
		"no format": {"hash": "h", "title": "T"},
	} {
		if _, err := parseDownloadArgs(args); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestToolResultMap_PropagatesIsError(t *testing.T) {
	failed := toolResultMap(&mcp.CallToolResultFor[any]{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: "Sorry — I couldn't send that book."}},
	})
	if failed["isError"] != true {
		t.Fatalf("failed send must carry isError=true, got %v", failed)
	}
	ok := toolResultMap(&mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: "Book sent"}},
	})
	if _, present := ok["isError"]; present {
		t.Fatalf("successful result must not set isError, got %v", ok)
	}
}
