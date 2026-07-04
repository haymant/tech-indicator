package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewHandler creates the MCP server, registers all 4 tools, and returns
// an http.Handler wrapped with CORS + auth middleware.
func NewHandler() http.Handler {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "tech-indicator",
		Version: "1.0.0",
	}, nil)

	registerTools(server)

	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:                  true,
			DisableLocalhostProtection: true, // required on Vercel
		},
	)

	return corsMiddleware(authMiddleware(mcpHandler))
}
