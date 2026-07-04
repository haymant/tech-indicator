---
title: Sync Endpoint — Testing Report
feature_id: F-001
artifact: testing-report
status: draft
version: 1.0.0
owner_agent: QA
parent_feature: F-001
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: QA
    description: Initial testing report with unit test results.
---

# Testing Report — Sync Endpoint

## Test Execution

```bash
$ go test ./internal/... -v -count=1
```

## Results

### Package: `internal/repository` — DuckDB Dialect

| Test | Status | Duration |
|------|--------|----------|
| TestDuckDBDialect_CreateTable | PASS | 0.00s |
| TestDuckDBDialect_DropTable | PASS | 0.00s |
| TestDuckDBDialect_Assets | PASS | 0.00s |
| TestDuckDBDialect_GetSince | PASS | 0.00s |
| TestDuckDBDialect_LastDate | PASS | 0.00s |
| TestDuckDBDialect_Append | PASS | 0.00s |

### Package: `internal/handler` — Sync Handler

| Test | Status | Duration | Maps to AC |
|------|--------|----------|------------|
| TestSyncHandler_ValidBody_Returns202 | PASS | 0.00s | AC-01 |
| TestSyncHandler_NoBody_Returns202WithDefaults | PASS | 0.00s | AC-02 |
| TestSyncHandler_InvalidJSON_Returns400 | PASS | 0.00s | AC-03 |
| TestSyncHandler_GET_Returns405 | PASS | 0.00s | AC-04 |
| TestSyncHandler_MissingTiingoKey_Returns500 | PASS | 0.00s | AC-07 |
| TestSyncHandler_MissingMotherDuckURL_Returns500 | PASS | 0.00s | AC-08 |
| TestSyncHandler_EmptyBody_Returns202 | PASS | 0.00s | AC-02 |

**All 13 tests PASS.**

## Acceptance Criteria Verification

| ID | Criterion | Status | Evidence |
|----|-----------|--------|----------|
| AC-01 | POST with valid body → 202 | ✅ | `TestSyncHandler_ValidBody_Returns202` |
| AC-02 | POST without body → 202 with defaults | ✅ | `TestSyncHandler_NoBody_Returns202WithDefaults`, `TestSyncHandler_EmptyBody_Returns202` |
| AC-03 | POST with invalid JSON → 400 | ✅ | `TestSyncHandler_InvalidJSON_Returns400` |
| AC-04 | GET → 405 | ✅ | `TestSyncHandler_GET_Returns405` |
| AC-05 | Snapshots exist in MotherDuck after sync | ✅ | Integration test: 124 rows synced (62 aapl + 62 tsla) |
| AC-06 | `days` parameter respected | ✅ | Data range verified: aapl starts 2026-04-06 (~90 days from 2026-07-04); tsla ends 2026-07-02 |
| AC-07 | Missing TIINGO_API_KEY → 500 | ✅ | `TestSyncHandler_MissingTiingoKey_Returns500` |
| AC-08 | Missing MOTHERDUCK_URL → 500 | ✅ | `TestSyncHandler_MissingMotherDuckURL_Returns500` |

## Integration Test Results

Executed a full end-to-end sync against live Tiingo API and MotherDuck:

```bash
$ curl -X POST http://localhost:3000/api/sync \
  -H "Content-Type: application/json" \
  -d '{"assets":["aapl","tsla"],"days":90,"workers":2}'
# Response: 202 Accepted

# After sync completion, MotherDuck query result:
snapshots table has 124 total rows
Assets in snapshots:
  aapl: 62 rows
  tsla: 62 rows

Sample rows for aapl (first 3):
  2026-04-06 O=256.27 H=261.92 L=256.22 C=258.62 V=29329911
  2026-04-07 O=255.92 H=255.96 L=245.47 C=253.27 V=62148008
  2026-04-08 O=258.21 H=259.51 L=256.29 C=258.66 V=41032772

Last rows for tsla (last 3):
  2026-07-02 O=428.01 H=432.35 L=389.30 C=393.45 V=73915762
  2026-07-01 O=421.46 H=432.86 L=418.09 C=425.30 V=40127902
  2026-06-30 O=406.00 H=424.54 L=406.00 C=420.60 V=43385619
```

**All 8 acceptance criteria now verified.**

## Residual Risk

| Risk | Impact | Mitigation |
|------|--------|------------|
| `DefaultSyncRunner` executes synchronously inside a goroutine — errors go to logs, not to the HTTP response | Observability gap | `slog.Error` is used; sufficient for fire-and-forget |
| MotherDuck connection failure on actual deploy | Runtime failure | Returns 500 at startup; logs give clear error message |

## Test Environment

- Go 1.26
- Standard library `httptest` + `testing`
- Unit tests: no external dependencies
- Integration test: live Tiingo API + MotherDuck (credentials from `.env`)
- DuckDB driver (`github.com/duckdb/duckdb-go/v2`)
