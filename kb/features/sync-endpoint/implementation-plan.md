---
title: Sync Endpoint — Implementation Plan
feature_id: F-001
artifact: implementation-plan
status: draft
version: 1.0.0
owner_agent: Architect
parent_feature: F-001
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: Architect
    description: Initial implementation plan.
---

# Implementation Plan — Sync Endpoint

## Phase 1: DuckDB Dialect & Repository Registration

**Files to create:**
- `internal/repository/duckdb_dialect.go`
- `internal/repository/duckdb_dialect_test.go`
- `internal/repository/motherduck.go`

**Steps:**
1. Define `DuckDBDialect` struct implementing `asset.SQLRepositoryDialect`.
2. Implement all 6 dialect methods with DuckDB-compatible SQL.
3. Write unit tests for each dialect method.
4. Register `"motherduck"` repository builder via `asset.RegisterRepositoryBuilder`.

**Verification:** Unit tests pass; dialect SQL strings are semantically correct.

## Phase 2: Sync Models

**Files to create:**
- `internal/model/sync.go`

**Steps:**
1. Define `SyncRequest` struct with `Assets`, `Days`, `Workers`, `Delay` fields.
2. Define `SyncResponse` struct for 202 Accepted.
3. Define `ErrorResponse` struct for 4xx/5xx.

**Verification:** Compiles; types match design spec.

## Phase 3: Sync Handler

**Files to modify:**
- `internal/handler/handler.go`

**Steps:**
1. Add `handleSync` method to `Handler` struct.
2. Validate method is POST (405 otherwise).
3. Read `TIINGO_API_KEY` and `MOTHERDUCK_URL` from env (500 if missing).
4. Parse optional JSON body with defaults.
5. Construct source (TiingoRepository) and target (SQLRepository via `asset.NewRepository("motherduck", url)`).
6. Configure and launch `asset.Sync.Run` in a goroutine.
7. Return 202 Accepted.
8. Register `/api/sync` route in `RegisterRoutes`.
9. Import motherduck registration package in handler (side-effect import).

**Verification:** Unit tests pass for all response codes.

## Phase 4: Handler Tests

**Files to create:**
- `internal/handler/sync_handler_test.go`

**Steps:**
1. Test POST with valid body → 202.
2. Test POST without body → 202 with defaults.
3. Test POST with invalid JSON → 400.
4. Test GET → 405.
5. Test POST with missing env vars → 500.

**Verification:** All tests pass; coverage of sync handler is complete.

## Phase 5: Update Feature Index

**Files to modify:**
- `kb/features/feature-index.md`

**Steps:**
1. Update design and implementation plan statuses.

## Dependency Graph

```
Phase 1 (dialect) ─► Phase 3 (handler)
                           │
Phase 2 (models) ─────────┘
                           │
                    Phase 4 (tests)
```

Phases 1 and 2 are independent and can be parallelised.
