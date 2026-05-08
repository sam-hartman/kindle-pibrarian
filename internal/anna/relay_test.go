package anna

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sam-hartman/kindle-pibrarian/internal/relay"
)

// helper: stand up a fake relay server, set env vars, return the server.
// The handler decodes the JSON-RPC envelope and exposes it via the
// captured pointer so individual tests can assert on it.
func mockRelay(t *testing.T, respondWith jsonRPCResponse) (*httptest.Server, *capture) {
	t.Helper()
	cap := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.method = r.Method
		cap.path = r.URL.Path
		cap.headers = r.Header.Clone()
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &cap.body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(respondWith)
	}))
	t.Setenv(relay.EnvBaseURL, srv.URL)
	t.Setenv(relay.EnvSecret, "test-secret")
	return srv, cap
}

type capture struct {
	method  string
	path    string
	headers http.Header
	body    jsonRPCRequest
}

func TestFindBookViaRelay_HitsCorrectEndpoint(t *testing.T) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
	}
	resp.Result = &struct {
		StructuredContent *struct {
			Items []*Book `json:"items"`
		} `json:"structuredContent"`
		IsError bool `json:"isError"`
	}{
		StructuredContent: &struct {
			Items []*Book `json:"items"`
		}{
			Items: []*Book{
				{Title: "Test Book", Hash: "abc123", Format: "epub"},
			},
		},
	}
	srv, cap := mockRelay(t, resp)
	defer srv.Close()

	books, err := findBookViaRelay("project hail mary", "epub")
	if err != nil {
		t.Fatalf("findBookViaRelay: %v", err)
	}
	if len(books) != 1 || books[0].Title != "Test Book" {
		t.Fatalf("unexpected books: %+v", books)
	}

	if cap.method != "POST" {
		t.Errorf("method = %q, want POST", cap.method)
	}
	if cap.path != "/mcp" {
		t.Errorf("path = %q, want /mcp", cap.path)
	}
	if got := cap.headers.Get(relay.HeaderTarget); got != relay.TargetPiAnnasMCP {
		t.Errorf("X-Relay-Target = %q", got)
	}
	if got := cap.headers.Get(relay.HeaderSecret); got != "test-secret" {
		t.Errorf("X-Relay-Secret = %q", got)
	}
	if cap.body.Method != "tools/call" {
		t.Errorf("body.method = %q", cap.body.Method)
	}
	if cap.body.Params.Name != "search" {
		t.Errorf("tool name = %q", cap.body.Params.Name)
	}
	if cap.body.Params.Arguments["term"] != "project hail mary" {
		t.Errorf("term arg = %v", cap.body.Params.Arguments["term"])
	}
	if cap.body.Params.Arguments["format"] != "epub" {
		t.Errorf("format arg = %v", cap.body.Params.Arguments["format"])
	}
}

func TestDownloadViaRelay_KindleEmailIncluded(t *testing.T) {
	resp := jsonRPCResponse{JSONRPC: "2.0", ID: 1}
	resp.Result = &struct {
		StructuredContent *struct {
			Items []*Book `json:"items"`
		} `json:"structuredContent"`
		IsError bool `json:"isError"`
	}{}
	srv, cap := mockRelay(t, resp)
	defer srv.Close()

	if err := downloadViaRelay("abc", "Title", "epub", "user@kindle.com"); err != nil {
		t.Fatalf("downloadViaRelay: %v", err)
	}
	if cap.body.Params.Name != "download" {
		t.Errorf("tool name = %q", cap.body.Params.Name)
	}
	if cap.body.Params.Arguments["hash"] != "abc" {
		t.Errorf("hash arg = %v", cap.body.Params.Arguments["hash"])
	}
	if cap.body.Params.Arguments["kindle_email"] != "user@kindle.com" {
		t.Errorf("kindle_email arg = %v", cap.body.Params.Arguments["kindle_email"])
	}
}

func TestDownloadViaRelay_NoKindleEmailOmitsField(t *testing.T) {
	resp := jsonRPCResponse{JSONRPC: "2.0", ID: 1}
	resp.Result = &struct {
		StructuredContent *struct {
			Items []*Book `json:"items"`
		} `json:"structuredContent"`
		IsError bool `json:"isError"`
	}{}
	srv, cap := mockRelay(t, resp)
	defer srv.Close()

	if err := downloadViaRelay("abc", "Title", "epub", ""); err != nil {
		t.Fatalf("downloadViaRelay: %v", err)
	}
	if _, ok := cap.body.Params.Arguments["kindle_email"]; ok {
		t.Errorf("kindle_email should be omitted when empty, got %v", cap.body.Params.Arguments["kindle_email"])
	}
}

func TestCallRelayTool_PropagatesError(t *testing.T) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Error: &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{Code: -32000, Message: "boom"},
	}
	srv, _ := mockRelay(t, resp)
	defer srv.Close()

	if _, err := callRelayTool("search", map[string]interface{}{"term": "x"}); err == nil {
		t.Fatal("expected error from relay tool error")
	}
}

func TestDecodeJSONRPC_HandlesSSEFraming(t *testing.T) {
	raw := []byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"isError\":false}}\n\n")
	parsed, err := decodeJSONRPC(raw)
	if err != nil {
		t.Fatalf("decodeJSONRPC: %v", err)
	}
	if parsed.Result == nil || parsed.Result.IsError {
		t.Errorf("unexpected result: %+v", parsed.Result)
	}
}
