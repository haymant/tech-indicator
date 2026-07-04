package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// parseSSEResponse extracts the JSON payload from an SSE-formatted response.
func parseSSEResponse(raw string) string {
	// SSE format: "event: message\ndata: {json}\n\n"
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			return strings.TrimPrefix(line, "data: ")
		}
	}
	// Fallback: try parsing as raw JSON (for potential non-SSE responses).
	return raw
}

// mcpPost sends an MCP JSON-RPC request and returns the response body.
func mcpPost(t *testing.T, handler http.Handler, body string, auth string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/mcp",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json,text/event-stream")
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return parseSSEResponse(w.Body.String())
}

// mcpOptions sends an OPTIONS preflight request.
func mcpOptions(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodOptions, "/api/mcp", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

// ─── TC-01: tools/list returns 4 tools ─────────────────────────────────────

func TestMCPServer_ToolList(t *testing.T) {
	handler := NewHandler()

	// Send initialize + tools/list in sequence (stateless mode).
	initResp := mcpPost(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"test","version":"1.0"}}}`, "test-key")
	t.Log("init:", initResp)

	listResp := mcpPost(t, handler, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, "test-key")
	t.Log("list:", listResp)

	var result struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				InputSchema any    `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(listResp), &result); err != nil {
		t.Fatalf("failed to parse tools/list response: %v\nbody: %s", err, listResp)
	}

	if len(result.Result.Tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(result.Result.Tools))
	}

	names := make(map[string]bool)
	for _, tool := range result.Result.Tools {
		names[tool.Name] = true
	}

	expected := []string{"sync_asset_data", "list_indicators", "calculate_indicators", "query_indicator_values"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

// ─── TC-02: tools/list includes sync_asset_data with correct schema ────────

func TestMCPServer_SyncAssetSchema(t *testing.T) {
	handler := NewHandler()

	mcpPost(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"test","version":"1.0"}}}`, "test-key")

	listResp := mcpPost(t, handler, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, "test-key")

	var result struct {
		Result struct {
			Tools []struct {
				Name        string          `json:"name"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	json.Unmarshal([]byte(listResp), &result)

	var syncTool struct {
		InputSchema struct {
			Properties map[string]struct {
				Type string `json:"type"`
			} `json:"properties"`
			Required []string `json:"required"`
		} `json:"inputSchema"`
	}
	for _, tool := range result.Result.Tools {
		if tool.Name == "sync_asset_data" {
			json.Unmarshal(tool.InputSchema, &syncTool.InputSchema)
			break
		}
	}

	if _, ok := syncTool.InputSchema.Properties["assets"]; !ok {
		t.Error("sync_asset_data missing 'assets' property")
	}
	if _, ok := syncTool.InputSchema.Properties["days"]; !ok {
		t.Error("sync_asset_data missing 'days' property")
	}

	hasRequired := false
	for _, r := range syncTool.InputSchema.Required {
		if r == "assets" {
			hasRequired = true
		}
	}
	if !hasRequired {
		t.Error("sync_asset_data should have 'assets' as required")
	}
}

// ─── TC-03: list_indicators tool call returns catalog ─────────────────────

func TestMCPServer_ListIndicatorsCall(t *testing.T) {
	handler := NewHandler()

	mcpPost(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"test","version":"1.0"}}}`, "test-key")

	resp := mcpPost(t, handler, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_indicators","arguments":{}}}`, "test-key")

	var result struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		t.Fatalf("failed to parse response: %v\nbody: %s", err, resp)
	}

	if len(result.Result.Content) == 0 {
		t.Fatal("no content in response")
	}

	var catalog struct {
		Count      int                    `json:"count"`
		Indicators []any                  `json:"indicators"`
		Categories map[string]interface{} `json:"categories"`
	}
	if err := json.Unmarshal([]byte(result.Result.Content[0].Text), &catalog); err != nil {
		t.Fatalf("failed to parse catalog: %v\ntext: %s", err, result.Result.Content[0].Text)
	}

	if catalog.Count < 17 {
		t.Errorf("expected at least 17 indicators, got %d", catalog.Count)
	}
	if len(catalog.Categories) == 0 {
		t.Error("expected categories in catalog")
	}
}

// ─── TC-04: CORS headers on OPTIONS ────────────────────────────────────────

func TestMCPServer_CORSHeaders(t *testing.T) {
	handler := NewHandler()
	w := mcpOptions(t, handler)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %s", origin)
	}

	exposeHeaders := w.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(exposeHeaders, "Mcp-Session-Id") {
		t.Errorf("expected Access-Control-Expose-Headers to contain Mcp-Session-Id, got %s", exposeHeaders)
	}

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content for OPTIONS, got %d", w.Code)
	}
}

// ─── TC-05: Auth rejects missing token ─────────────────────────────────────

func TestMCPServer_AuthRejectsMissingToken(t *testing.T) {
	// Set the env var for this test.
	os.Setenv("TECH_INDICATOR_API_KEY", "test-secret-key")
	defer os.Unsetenv("TECH_INDICATOR_API_KEY")

	handler := NewHandler()

	// No auth header.
	req := httptest.NewRequest(http.MethodPost, "/api/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json,text/event-stream")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// ─── TC-06: Auth rejects wrong token ───────────────────────────────────────

func TestMCPServer_AuthRejectsWrongToken(t *testing.T) {
	os.Setenv("TECH_INDICATOR_API_KEY", "test-secret-key")
	defer os.Unsetenv("TECH_INDICATOR_API_KEY")

	handler := NewHandler()

	// Wrong token.
	req := httptest.NewRequest(http.MethodPost, "/api/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json,text/event-stream")
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// ─── TC-07: Auth allows valid token ────────────────────────────────────────

func TestMCPServer_AuthAllowsValidToken(t *testing.T) {
	os.Setenv("TECH_INDICATOR_API_KEY", "test-secret-key")
	defer os.Unsetenv("TECH_INDICATOR_API_KEY")

	handler := NewHandler()

	resp := mcpPost(t, handler,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"test","version":"1.0"}}}`, "test-secret-key")

	var result struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		t.Fatalf("expected valid response with correct auth, got error: %v\nbody: %s", err, resp)
	}
	if len(result.Result) == 0 {
		t.Error("expected non-empty result with valid auth")
	}
}

// ─── TC-08: Auth disabled when env var unset ───────────────────────────────

func TestMCPServer_AuthDisabledWhenUnset(t *testing.T) {
	os.Unsetenv("TECH_INDICATOR_API_KEY")

	handler := NewHandler()

	resp := mcpPost(t, handler,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"test","version":"1.0"}}}`, "")

	var result struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		t.Fatalf("expected success without auth when env var unset: %v\nbody: %s", err, resp)
	}
}

// ─── TC-09: OPTIONS passes through auth check ──────────────────────────────

func TestMCPServer_CORSPreflightPassesAuth(t *testing.T) {
	os.Setenv("TECH_INDICATOR_API_KEY", "test-secret-key")
	defer os.Unsetenv("TECH_INDICATOR_API_KEY")

	handler := NewHandler()

	// OPTIONS with no auth should pass.
	w := mcpOptions(t, handler)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS with no auth, got %d", w.Code)
	}
}

// ─── TC-10: helpers ────────────────────────────────────────────────────────

func TestItoa(t *testing.T) {
	if itoa(0) != "0" {
		t.Errorf("itoa(0) = %s", itoa(0))
	}
	if itoa(1) != "1" {
		t.Errorf("itoa(1) = %s", itoa(1))
	}
	if itoa(123) != "123" {
		t.Errorf("itoa(123) = %s", itoa(123))
	}
}

func TestSplitComma(t *testing.T) {
	r := splitComma("a,b,c")
	if len(r) != 3 || r[0] != "a" || r[1] != "b" || r[2] != "c" {
		t.Errorf("splitComma(a,b,c) = %v", r)
	}
	r = splitComma("")
	if r != nil {
		t.Errorf("splitComma('') = %v", r)
	}
	r = splitComma("aapl, msft, googl")
	if len(r) != 3 || r[1] != "msft" {
		t.Errorf("splitComma with spaces = %v", r)
	}
}

// ─── TC-11: errorResult creates proper error response ──────────────────────

func TestErrorResult(t *testing.T) {
	r := errorResult("test error")
	if !r.IsError {
		t.Error("expected IsError = true")
	}
	if len(r.Content) == 0 {
		t.Fatal("expected content")
	}
	tc, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if tc.Text != "test error" {
		t.Errorf("expected 'test error', got '%s'", tc.Text)
	}
}
