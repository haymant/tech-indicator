package handler

import (
	"bytes"
	"embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"vercel-go-starter/internal/model"
)

var testEmptyFS embed.FS

const testAPIKey = "test-api-key-abc"

// mockSyncRunner is a no-op sync runner used in tests to avoid real database connections.
func mockSyncRunner(_ string, _ string, _ model.SyncRequest) error {
	return nil
}

// newTestHandler creates a Handler with the mock sync runner.
func newTestHandler() *Handler {
	h := New(testEmptyFS)
	h.syncFunc = mockSyncRunner
	return h
}

// setTestEnv sets common env vars for tests that need them.
func setTestEnv(t *testing.T) {
	t.Helper()
	os.Setenv("TIINGO_API_KEY", "test-key-123")
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("TECH_INDICATOR_API_KEY", testAPIKey)
}

// clearTestEnv clears common env vars set by setTestEnv.
func clearTestEnv() {
	os.Unsetenv("TIINGO_API_KEY")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("TECH_INDICATOR_API_KEY")
}

// authorizedRequest creates a POST request with the bearer token header.
func authorizedRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(http.MethodPost, "/api/sync", http.NoBody)
	} else {
		req = httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
	}
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	return req
}

func TestSyncHandler_ValidBody_Returns200(t *testing.T) {
	setTestEnv(t)
	defer clearTestEnv()

	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := authorizedRequest(t, `{"assets":["aapl","msft"],"days":90,"workers":2}`)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp model.SyncResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %s", resp.Status)
	}
	if len(resp.Assets) != 2 {
		t.Errorf("expected 2 assets, got %d", len(resp.Assets))
	}
	if resp.Days != 90 {
		t.Errorf("expected days 90, got %d", resp.Days)
	}
	if resp.Workers != 2 {
		t.Errorf("expected workers 2, got %d", resp.Workers)
	}
}

func TestSyncHandler_NoBody_Returns200WithDefaults(t *testing.T) {
	setTestEnv(t)
	defer clearTestEnv()

	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := authorizedRequest(t, "")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp model.SyncResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Days != 365 {
		t.Errorf("expected default days 365, got %d", resp.Days)
	}
	if resp.Workers != 1 {
		t.Errorf("expected default workers 1, got %d", resp.Workers)
	}
}

func TestSyncHandler_InvalidJSON_Returns400(t *testing.T) {
	setTestEnv(t)
	defer clearTestEnv()

	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := authorizedRequest(t, "not-json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var resp model.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.Contains(resp.Message, "Invalid") {
		t.Errorf("expected message containing 'Invalid', got %s", resp.Message)
	}
}

func TestSyncHandler_GET_Returns405(t *testing.T) {
	setTestEnv(t)
	defer clearTestEnv()

	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/sync", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestSyncHandler_MissingTiingoKey_Returns500(t *testing.T) {
	os.Unsetenv("TIINGO_API_KEY")
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("TECH_INDICATOR_API_KEY", testAPIKey)
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("TECH_INDICATOR_API_KEY")

	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}

	var resp model.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.Contains(resp.Message, "TIINGO_API_KEY") {
		t.Errorf("expected message mentioning TIINGO_API_KEY, got %s", resp.Message)
	}
}

func TestSyncHandler_MissingDatabaseURL_Returns500(t *testing.T) {
	os.Setenv("TIINGO_API_KEY", "test-key-123")
	os.Unsetenv("DATABASE_URL")
	os.Setenv("TECH_INDICATOR_API_KEY", testAPIKey)
	defer os.Unsetenv("TIINGO_API_KEY")
	defer os.Unsetenv("TECH_INDICATOR_API_KEY")

	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}

	var resp model.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.Contains(resp.Message, "DATABASE_URL") {
		t.Errorf("expected message mentioning DATABASE_URL, got %s", resp.Message)
	}
}

func TestSyncHandler_EmptyBody_Returns200(t *testing.T) {
	setTestEnv(t)
	defer clearTestEnv()

	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewReader([]byte{}))
	req.ContentLength = 0
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestSyncHandler_MissingToken_Returns401(t *testing.T) {
	setTestEnv(t)
	defer clearTestEnv()

	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", http.NoBody)
	// No Authorization header.
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	var resp model.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.Contains(resp.Message, "Unauthorized") {
		t.Errorf("expected message containing 'Unauthorized', got %s", resp.Message)
	}
}

func TestSyncHandler_WrongToken_Returns401(t *testing.T) {
	setTestEnv(t)
	defer clearTestEnv()

	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", http.NoBody)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	var resp model.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.Contains(resp.Message, "Unauthorized") {
		t.Errorf("expected message containing 'Unauthorized', got %s", resp.Message)
	}
}

func TestSyncHandler_EmptyBearerToken_Returns401(t *testing.T) {
	setTestEnv(t)
	defer clearTestEnv()

	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// curl -H "Authorization: Bearer " when the shell variable is empty.
	req := httptest.NewRequest(http.MethodPost, "/api/sync", http.NoBody)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
