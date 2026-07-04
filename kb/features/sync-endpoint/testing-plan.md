---
title: Sync Endpoint — Testing Plan
feature_id: F-001
artifact: testing-plan
status: draft
version: 1.0.0
owner_agent: QA
parent_feature: F-001
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: QA
    description: Initial testing plan.
---

# Testing Plan — Sync Endpoint

## Scope

Unit-level verification of all new components. Integration tests for the full sync pipeline require live Tiingo API credentials and a MotherDuck token, which are staged in `.env` for manual / CI runs.

## Test Cases

### TC-01: DuckDBDialect — CreateTable returns valid DDL

| Field | Value |
|-------|-------|
| **Source** | `internal/repository/duckdb_dialect_test.go` |
| **Input** | `NewDuckDBDialect().CreateTable()` |
| **Expected** | Returns `CREATE TABLE IF NOT EXISTS snapshots (...)` with all 7 columns. |
| **Maps to AC** | AC-05 (schema correctness) |

### TC-02: DuckDBDialect — DropTable

| Field | Value |
|-------|-------|
| **Source** | `duckdb_dialect_test.go` |
| **Input** | `NewDuckDBDialect().DropTable()` |
| **Expected** | Returns `DROP TABLE IF EXISTS snapshots` |

### TC-03: DuckDBDialect — Assets query

| Field | Value |
|-------|-------|
| **Source** | `duckdb_dialect_test.go` |
| **Input** | `NewDuckDBDialect().Assets()` |
| **Expected** | Returns `SELECT DISTINCT name FROM snapshots ORDER BY name` |

### TC-04: DuckDBDialect — GetSince query

| Field | Value |
|-------|-------|
| **Source** | `duckdb_dialect_test.go` |
| **Input** | `NewDuckDBDialect().GetSince()` |
| **Expected** | Returns `SELECT date, open, high, low, close, volume FROM snapshots WHERE name = $1 AND date >= $2 ORDER BY date` with DuckDB-style `$N` placeholders |

### TC-05: DuckDBDialect — LastDate query

| Field | Value |
|-------|-------|
| **Source** | `duckdb_dialect_test.go` |
| **Input** | `NewDuckDBDialect().LastDate()` |
| **Expected** | Returns `SELECT MAX(date) FROM snapshots WHERE name = $1` |

### TC-06: DuckDBDialect — Append query

| Field | Value |
|-------|-------|
| **Source** | `duckdb_dialect_test.go` |
| **Input** | `NewDuckDBDialect().Append()` |
| **Expected** | Returns `INSERT INTO snapshots (name, date, open, high, low, close, volume) VALUES ($1, $2, $3, $4, $5, $6, $7)` with 7 `$N` placeholders |

### TC-07: Sync Handler — POST with valid body returns 202

| Field | Value |
|-------|-------|
| **Source** | `internal/handler/sync_handler_test.go` |
| **Input** | POST `/api/sync` with body `{"assets":["aapl"],"days":30,"workers":2}` |
| **Expected** | Status 202; response JSON matches `SyncResponse` with assets, days, workers fields |

### TC-08: Sync Handler — POST without body returns 202

| Field | Value |
|-------|-------|
| **Source** | `sync_handler_test.go` |
| **Input** | POST `/api/sync` with empty body |
| **Expected** | Status 202; response assets is nil / empty, days=365, workers=1 |

### TC-09: Sync Handler — POST with invalid JSON returns 400

| Field | Value |
|-------|-------|
| **Source** | `sync_handler_test.go` |
| **Input** | POST `/api/sync` with body `not-json` |
| **Expected** | Status 400 |

### TC-10: Sync Handler — GET returns 405

| Field | Value |
|-------|-------|
| **Source** | `sync_handler_test.go` |
| **Input** | GET `/api/sync` |
| **Expected** | Status 405 |

### TC-11: Sync Handler — Missing TIINGO_API_KEY returns 500

| Field | Value |
|-------|-------|
| **Source** | `sync_handler_test.go` |
| **Input** | POST `/api/sync` with `TIINGO_API_KEY` unset |
| **Expected** | Status 500; message contains `TIINGO_API_KEY` |

### TC-12: Sync Handler — Missing MOTHERDUCK_URL returns 500

| Field | Value |
|-------|-------|
| **Source** | `sync_handler_test.go` |
| **Input** | POST `/api/sync` with `MOTHERDUCK_URL` unset |
| **Expected** | Status 500; message contains `MOTHERDUCK_URL` |

## Test Environment

- **Unit tests** run with `go test ./internal/...` — no external dependencies.
- Handler tests use `httptest.NewRecorder` and `httptest.NewRequest` from stdlib.
- DuckDB or MotherDuck **not required** for unit tests (dialect tests verify SQL strings, handler tests mock env vars).

## Evidence Collection

After implementation, run:

```bash
go test ./internal/... -v -count=1 2>&1
```

Attach output to `testing-report.md`.
