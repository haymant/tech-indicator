---
title: MCP Server — Testing Plan
feature_id: F-004
artifact: testing-plan
status: draft
version: 1.0.0
owner_agent: QA
parent_feature: F-004
last_updated: 2026-07-04
---

# Testing Plan — MCP Server

## Unit Tests

### TC-01: tools/list returns 4 tools

| Field | Value |
|-------|-------|
| **Source** | `internal/mcp/mcp_test.go` |
| **Method** | Initialize MCP client, call `tools/list` |
| **Expected** | 4 tools: sync_asset_data, list_indicators, calculate_indicators, query_indicator_values |

### TC-02: tools/list includes sync_asset_data schema

| Field | Value |
|-------|-------|
| **Source** | `mcp_test.go` |
| **Expected** | Tool has `inputSchema` with `assets` (required, array), `days` (int), `workers` (int) |

### TC-03: list_indicators returns catalog

| Field | Value |
|-------|-------|
| **Source** | `mcp_test.go` |
| **Expected** | Returns JSON with `count` ≥ 89 and `categories` |

### TC-04: CORS headers on OPTIONS

| Field | Value |
|-------|-------|
| **Source** | `mcp_test.go` |
| **Expected** | `Access-Control-Allow-Origin: *`, `Access-Control-Expose-Headers: mcp-session-id` |

### TC-05: Auth rejects invalid token

| Field | Value |
|-------|-------|
| **Source** | `mcp_test.go` |
| **Expected** | 403 with no/wrong `Authorization` header |

### TC-06: Auth allows valid token

| Field | Value |
|-------|-------|
| **Expected** | Request with valid Bearer token passes through |

## Curl SIT Tests

### SIT-01: tools/list

```bash
curl -X POST http://localhost:3000/api/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json,text/event-stream" \
  -H "Authorization: Bearer $TECH_INDICATOR_API_KEY" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
# Expected: 200, 4 tools in result.tools
```

### SIT-02: tools/call list_indicators

```bash
curl -X POST http://localhost:3000/api/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json,text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_indicators","arguments":{}}}'
# Expected: 200, JSON with indicators array
```
