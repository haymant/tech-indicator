package mcp

import (
	"net/http"
	"os"
)

// corsMiddleware adds permissive CORS headers required by MCP agent hosts.
// The Access-Control-Expose-Headers: mcp-session-id is critical — without it,
// browsers hide the session ID header and MCP transport breaks.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware checks Bearer token against TECH_INDICATOR_API_KEY env var.
// When the env var is empty, auth is disabled (local dev convenience).
// CORS preflight passes through before auth check.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := os.Getenv("TECH_INDICATOR_API_KEY")
		if expected == "" {
			next.ServeHTTP(w, r)
			return
		}

		// CORS preflight passes through unauthenticated.
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+expected {
			http.Error(w, "Forbidden: invalid or missing API key", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
