---
title: MCP Server — Technical Design
feature_id: F-004
artifact: design
status: draft
version: 1.0.0
owner_agent: Architect
parent_feature: F-004
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: Architect
    description: Initial design for Go MCP SDK-based server.
---

# Design — MCP Server for Technical Indicator API

## 1. Design Summary

Add a new `internal/mcp/` package that wraps the existing Go REST API handlers as MCP tools using the official `github.com/modelcontextprotocol/go-sdk`. The MCP server is served via Streamable HTTP at `/api/mcp` as part of the existing Go binary — no separate deployment.

## 2. Components & Interfaces

### 2.1 New Package

```
internal/mcp/
├── handler.go       # NewMCPServer() — creates server, registers tools
├── tools.go         # Tool handler implementations
├── middleware.go    # CORS + Bearer auth middleware
└── mcp_test.go      # Unit tests + curl-based SIT verification
```

### 2.2 Core Types (from SDK)

```go
// Server is the top-level MCP server.
server := mcp.NewServer(&mcp.Implementation{Name: "tech-indicator", Version: "1.0.0"}, nil)

// Tool registration with typed handler.
type Input struct {
    Name string `json:"name" jsonschema:"required,the name"`
}
mcp.AddTool(server, &mcp.Tool{
    Name:        "tool_name",
    Description: "...",
}, handlerFunc)

// Handler signature.
func handlerFunc(ctx context.Context, req *mcp.CallToolRequest, input Input) (*mcp.CallToolResult, Output, error)

// Streamable HTTP transport (stateless for Vercel).
handler := mcp.NewStreamableHTTPHandler(
    func(r *http.Request) *mcp.Server { return server },
    &mcp.StreamableHTTPOptions{Stateless: true},
)
```

### 2.3 Middleware Stack

```
Request → AuthMiddleware → CORSMiddleware → StreamableHTTPHandler → MCPServer → ToolHandler
```

Both middlewares are standard `net/http` middleware, same pattern as the existing REST handlers.

## 3. Tool Definitions

### 3.1 sync_asset_data

```go
type SyncAssetInput struct {
    Assets  []string `json:"assets"  jsonschema:"required,ticker symbols to sync, e.g. AAPL,MSFT"`
    Days    int      `json:"days"    jsonschema:"number of lookback days, default 365"`
    Workers int      `json:"workers" jsonschema:"concurrent workers, default 1"`
}

type SyncAssetOutput struct {
    Status  string   `json:"status"`
    Message string   `json:"message"`
    Assets  []string `json:"assets"`
}
```

Handler calls `DefaultSyncRunner(tiingoKey, databaseURL, req)` and returns the result.

### 3.2 list_indicators

```go
type ListIndicatorsOutput struct {
    Count      int                        `json:"count"`
    Indicators []model.IndicatorEntry     `json:"indicators"`
    Categories map[string]model.CatalogCategory `json:"categories"`
}
```

Handler reads `indicator.Registry` static map. No auth required.

### 3.3 calculate_indicators

```go
type CalculateIndicatorsInput struct {
    Assets     []string `json:"assets"     jsonschema:"ticker symbols, defaults to all"`
    Indicators []string `json:"indicators" jsonschema:"specific indicators, defaults to all 89"`
    Days       int      `json:"days"       jsonschema:"lookback days, defaults to all available"`
}
```

Handler calls `handler.calculateForAsset()` internally for each asset. Reuses the engine batch compute + write path.

### 3.4 query_indicator_values

```go
type QueryIndicatorValuesInput struct {
    Symbols    string `json:"symbols"    jsonschema:"required,comma-separated tickers"`
    Indicators string `json:"indicators" jsonschema:"comma-separated indicator names"`
    DateFrom   string `json:"date_from"  jsonschema:"start date ISO, e.g. 2025-01-01"`
    DateTo     string `json:"date_to"    jsonschema:"end date ISO, e.g. 2026-07-04"`
}
```

Handler queries MotherDuck `indicators` table via `pgx` and returns grouped time-series data.

## 4. Middleware

### 4.1 Bearer Auth

Reads `TECH_INDICATOR_API_KEY` env var. Compares full `Authorization` header. Skips if env var is unset. Passes OPTIONS through.

### 4.2 CORS

Sets headers:
- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: POST, GET, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type, Authorization, Mcp-Session-Id`
- `Access-Control-Expose-Headers: Mcp-Session-Id`

## 5. Route Registration

In `cmd/server/main.go`:

```go
import "vercel-go-starter/internal/mcp"

func main() {
    mux := http.NewServeMux()
    h := handler.New(starter.StaticFiles)
    h.RegisterRoutes(mux)

    // Register MCP endpoint.
    mcpHandler := mcp.NewHandler()
    mux.Handle("/api/mcp", mcpHandler)

    http.ListenAndServe(":3000", mux)
}
```

`mcp.NewHandler()` returns the full middleware-wrapped handler.

## 6. Implementation Sequence

1. Add `github.com/modelcontextprotocol/go-sdk` dependency
2. Create `internal/mcp/middleware.go` (CORS + auth)
3. Create `internal/mcp/handler.go` (server setup, tool registration)
4. Create `internal/mcp/tools.go` (4 tool handlers)
5. Modify `cmd/server/main.go` to register `/api/mcp`
6. Create `internal/mcp/mcp_test.go` with comprehensive tests
7. Run all tests, verify no regressions
8. Run curl tests against local server
9. Deploy to Vercel
