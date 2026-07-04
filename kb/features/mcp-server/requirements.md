---
title: MCP Server — Expose REST APIs as MCP Tools
feature_id: F-004
artifact: requirements
status: draft
version: 2.0.0
owner_agent: BA
parent_feature: null
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: BA
    description: Rewrite to use official Go MCP SDK (modelcontextprotocol/go-sdk). Pure Go, no Python. CORS must-have for agent hosts.
---

# Requirements — MCP Server for Technical Indicator API (F-004)

## 1. Business Context

The tech-indicator API currently exposes four REST endpoints (sync, list indicators, calculate indicators, query indicator values). These are usable by traditional HTTP clients but **not discoverable or callable by AI agents** that communicate via the **Model Context Protocol (MCP)**.

MCP is the standard protocol that AI coding agents (Claude Desktop, Copilot, Cursor, etc.) use to discover and invoke external tools. By exposing our existing functionality as MCP tools over **Streamable HTTP**, any MCP-compatible host can connect to our server and use it to fetch market data, compute technical indicators, and query results — all through natural language.

## 2. Constraints

| # | Constraint | Rationale |
|---|-----------|-----------|
| C-01 | **Use the official Go MCP SDK** (`github.com/modelcontextprotocol/go-sdk`). | Tier 1 SDK from the MCP team (Google). Full spec coverage, JSON-RPC, OAuth support, conformance tested. Pure Go, no Python. |
| C-02 | **MCP transport must be Streamable HTTP** with `Stateless: true`. | Required for Vercel serverless — no in-memory session survives between invocations or cold starts. |
| C-03 | **CORS must be permissive** (`Access-Control-Allow-Origin: *`, expose `mcp-session-id`). | **Critical for agent hosts** (MCP Inspector, browser-based tool connectors). Without `expose_headers`, browsers hide the `mcp-session-id` header and the transport breaks. |
| C-04 | **Bearer token auth** via standard `net/http` middleware wrapping the MCP handler. | Same auth as existing REST API (`TECH_INDICATOR_API_KEY`). Standard HTTP middleware, not SDK-level. |
| C-05 | **MCP tools must call existing Go code internally**, not reimplement business logic. | The existing handlers are the source of truth. The MCP layer is a protocol adapter. |
| C-06 | **MCP server must be deployable on Vercel** as part of the existing Go binary. | Ships with the existing binary — no separate service, no Python runtime. |
| C-07 | **Each tool must have typed input/output structs** with `jsonschema` tags for auto-generated schemas. | The Go SDK's `AddTool` uses struct tags to generate JSON Schema that MCP hosts present to users. |
| C-08 | **Must support `tools/list` and `tools/call`** MCP methods. | Minimum required by the MCP specification. |

## 3. Functional Requirements

### FR-01: MCP Tool — `sync_asset_data`

Maps to POST `/api/sync`.

| Field | Value |
|-------|-------|
| **Tool name** | `sync_asset_data` |
| **Description** | Sync historical OHLCV market data from Tiingo into MotherDuck for one or more ticker symbols. |

**Input struct:**

```go
type SyncAssetInput struct {
    Assets  []string `json:"assets"  jsonschema:"required,ticker symbols to sync, e.g. AAPL,MSFT"`
    Days    int      `json:"days"    jsonschema:"number of lookback days, default 365"`
    Workers int      `json:"workers" jsonschema:"concurrent workers, default 1"`
}
```

**Implementation:** Reuses `DefaultSyncRunner` from the existing handler. Returns the sync result as JSON `TextContent`.

---

### FR-02: MCP Tool — `list_indicators`

Maps to GET `/api/indicators`.

| Field | Value |
|-------|-------|
| **Tool name** | `list_indicators` |
| **Description** | List all available technical indicators with category, description, inputs, and default parameters. |

**Input:** None (read-only catalog). No auth required.

**Implementation:** Reads the static `indicator.Registry` map. Returns the catalog as JSON `TextContent`. No database call.

---

### FR-03: MCP Tool — `calculate_indicators`

Maps to POST `/api/indicators/calculate`.

| Field | Value |
|-------|-------|
| **Tool name** | `calculate_indicators` |
| **Description** | Compute technical indicators for specified assets using OHLCV data already synced to MotherDuck. Results stored in the `indicators` table with idempotent upsert. |

**Input struct:**

```go
type CalculateIndicatorsInput struct {
    Assets     []string `json:"assets"     jsonschema:"ticker symbols, e.g. AAPL,MSFT; defaults to all assets"`
    Indicators []string `json:"indicators" jsonschema:"specific indicators, e.g. rsi_14,sma_20; defaults to all 89"`
    Days       int      `json:"days"       jsonschema:"lookback days, defaults to all available"`
}
```

**Implementation:** Calls the existing `calculateForAsset` logic internally. Returns a summary (assets processed, indicators computed) as JSON `TextContent`.

---

### FR-04: MCP Tool — `query_indicator_values`

Maps to GET `/api/indicators/values`.

| Field | Value |
|-------|-------|
| **Tool name** | `query_indicator_values` |
| **Description** | Fetch computed indicator values for given symbols and indicators. Returns time-series data points with dates. |

**Input struct:**

```go
type QueryIndicatorValuesInput struct {
    Symbols    string `json:"symbols"    jsonschema:"required,comma-separated tickers, e.g. AAPL,MSFT"`
    Indicators string `json:"indicators" jsonschema:"comma-separated indicator names, e.g. rsi_14,sma_20"`
    DateFrom   string `json:"date_from"  jsonschema:"start date ISO, e.g. 2025-01-01"`
    DateTo     string `json:"date_to"    jsonschema:"end date ISO, e.g. 2026-07-04"`
}
```

**Implementation:** Queries MotherDuck `indicators` table directly via `pgx`. Returns time-series data grouped by symbol and indicator as JSON `TextContent`.

---

### FR-05: MCP Server Instructions

The MCP server must expose instructions describing the workflow to AI agents:

```
Technical Indicator MCP Server

Provides access to a technical analysis API for stock market data.

Available tools:
- sync_asset_data: Sync OHLCV market data from Tiingo into MotherDuck.
- list_indicators: List all 89 supported technical indicators with metadata.
- calculate_indicators: Compute indicators (RSI, MACD, SMA, etc.) for assets.
- query_indicator_values: Fetch computed indicator time-series values.

Workflow:
1. First, sync data for the ticker(s) you care about.
2. Optionally list indicators to see what's available.
3. Calculate the indicators you need.
4. Query the computed values for analysis or reporting.
```

### FR-06: CORS Configuration (Must Have)

CORS must be configured as standard `net/http` middleware. **Without proper CORS, agent hosts running in browsers cannot connect.**

Required headers:
- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: POST, GET, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type, Authorization, Mcp-Session-Id`
- `Access-Control-Expose-Headers: Mcp-Session-Id` ← critical

CORS preflight (`OPTIONS`) must pass through before auth check.

### FR-07: Authentication

Bearer token auth as standard `net/http` middleware, using the **same env var** (`TECH_INDICATOR_API_KEY`) as the existing REST API.

**Exact implementation pattern:**

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        expected := os.Getenv("TECH_INDICATOR_API_KEY")
        if expected == "" {
            next.ServeHTTP(w, r) // auth disabled when unset (local dev)
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
```

**Key points:**
- Reads `TECH_INDICATOR_API_KEY` env var (same variable as the REST endpoints use in `requireBearerAuth`).
- Compares the full `Authorization` header string: `"Bearer " + expected`.
- When the env var is empty, auth is disabled (local dev convenience).
- CORS preflight (`OPTIONS`) passes through before auth check.
- Returns **403** on missing or invalid token.

### FR-08: Deployment

The MCP server runs as part of the existing Go binary. Route added to the existing `http.ServeMux`:

```go
mcpServer := mcp.NewServer(&mcp.Implementation{
    Name:    "tech-indicator",
    Version: "1.0.0",
}, nil)
registerTools(mcpServer)

mcpHandler := mcp.NewStreamableHTTPHandler(
    func(r *http.Request) *mcp.Server { return mcpServer },
    &mcp.StreamableHTTPOptions{Stateless: true},
)

// Middleware stack: auth → cors → mcp
mux.Handle("/api/mcp", authMiddleware(corsMiddleware(mcpHandler)))
```

Deploys as part of the existing Vercel Go deployment — no separate service needed.

## 4. Architecture

```
                    ┌─────────────────────────────┐
                    │   HTTP Request /api/mcp      │
                    └─────────────┬───────────────┘
                                  │
                          ┌───────▼────────┐
                          │  Auth Middleware │  Bearer token check
                          └───────┬────────┘
                                  │
                          ┌───────▼────────┐
                          │  CORS Middleware │  Access-Control-* headers
                          └───────┬────────┘
                                  │
              ┌───────────────────▼────────────────────┐
              │  StreamableHTTPHandler (Stateless: true) │  JSON-RPC over HTTP
              └───────────────────┬────────────────────┘
                                  │
              ┌───────────────────▼────────────────────┐
              │          MCP Server                     │
              │  tools/list → dispatch catalog          │
              │  tools/call → route to handler          │
              └───────┬──────────┬──────────┬─────────┘
                      │          │          │
              ┌───────▼──┐ ┌────▼───┐ ┌───▼────────┐
              │syncAsset │ │listInd │ │calcInd     │
              │(Tiingo)  │ │(static)│ │(engine+SQL)│
              └──────────┘ └────────┘ └────────────┘
                                     ┌───▼────────┐
                                     │queryValues │
                                     │(pgx query) │
                                     └────────────┘
```

Backend mapping:

| Tool | Calls |
|------|-------|
| `sync_asset_data` | `DefaultSyncRunner` (existing) |
| `list_indicators` | `indicator.Registry` (static) |
| `calculate_indicators` | `engine.ComputeIndicators` + `engine.WriteIndicators` + caching |
| `query_indicator_values` | Direct `pgx` query against `indicators` table |

## 5. Non-Functional Requirements

| Aspect | Requirement |
|--------|-------------|
| SDK | `github.com/modelcontextprotocol/go-sdk` (Tier 1, official) |
| Transport | Streamable HTTP, `Stateless: true` |
| Cold start | < 100ms (compiled Go binary) |
| Auth | Bearer token via `net/http` middleware |
| CORS | `net/http` middleware, expose `mcp-session-id` |
| Schema | Auto-generated from Go structs via `jsonschema` tags |
| Runtime | Go 1.26 (same as existing project) |
| Deploy | Ships in existing Go binary, single `vercel --prod` |

## 6. Acceptance Criteria

| ID | Criterion | Verification |
|----|-----------|-------------|
| AC-01 | `tools/list` returns 4 tools with correct schemas | `curl POST .../api/mcp ... -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'` |
| AC-02 | `tools/call sync_asset_data` syncs data | Invoke with assets=["SMH"], verify `snapshots` table |
| AC-03 | `tools/call list_indicators` returns catalog | Verify 89 indicators in response |
| AC-04 | `tools/call calculate_indicators` computes and stores | Invoke with assets=["SMH"], verify `indicators` table |
| AC-05 | `tools/call query_indicator_values` returns data | Invoke with symbols=SMH, verify time-series JSON |
| AC-06 | Invalid auth returns 403 | Test with no/wrong `Authorization` header |
| AC-07 | CORS preflight returns correct headers | `curl -X OPTIONS ... -I` shows `access-control-allow-origin: *` |
| AC-08 | `mcp-session-id` in expose headers | Response includes `access-control-expose-headers: mcp-session-id` |
| AC-09 | MCP Inspector connects and shows tools | Inspector lists 4 tools, can invoke each |
| AC-10 | Works on Vercel after cold start | Deploy, wait 30s, first request succeeds |

## 7. Dependencies

```
github.com/modelcontextprotocol/go-sdk   # Tier 1 Go MCP SDK
  ├── github.com/google/jsonschema-go    # Schema generation from struct tags
  ├── JSON-RPC implementation            # Built-in
  └── Streamable HTTP + SSE transports   # Built-in
```

Install: `go get github.com/modelcontextprotocol/go-sdk`

## 8. File Changes

| File | Action |
|------|--------|
| `internal/mcp/handler.go` | **NEW** — server setup, tool registration |
| `internal/mcp/tools.go` | **NEW** — all 4 tool handlers |
| `internal/mcp/middleware.go` | **NEW** — CORS + auth middleware |
| `internal/mcp/mcp_test.go` | **NEW** — unit tests |
| `cmd/server/main.go` | **MODIFY** — register `/api/mcp` route |
| `go.mod` / `go.sum` | **MODIFY** — add SDK dependency |
