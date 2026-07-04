---
title: Sync Endpoint — POST /api/sync
feature_id: F-001
artifact: requirements
status: draft
version: 1.0.0
owner_agent: BA
parent_feature: null
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: BA
    description: Initial requirements draft.
---

# Requirements — Sync Endpoint

## 1. Business Context

The project `tech-indicator` wraps the external Go library `github.com/cinar/indicator/v2` (the **indicator library**, at path `indicator/` in this repository). The indicator library provides technical analysis indicators, backtesting, and an **asset sync** capability (`asset.Sync`) that copies financial market snapshots between a **source repository** (e.g., Tiingo) and a **target repository** (e.g., filesystem or SQL database).

Currently the application exposes general-purpose stub endpoints (`/api/data`, `/api/items/`). There is no way to trigger an on-demand market-data sync from the application itself.

## 2. Constraints

| # | Constraint | Rationale |
|---|-----------|-----------|
| C-01 | **The `indicator/` folder is an external Go dependency. Do not modify it at any phase.** | It is the upstream `github.com/cinar/indicator/v2` library pulled via `go.mod`. Changes would diverge from upstream and violate the dependency contract. |
| C-02 | The sync must work with **MotherDuck** as the target data store, using the `MOTHERDUCK_URL` environment variable from `.env`. | MotherDuck provides a serverless DuckDB-in-the-cloud experience suitable as a long-term storage layer for market snapshots. |
| C-03 | The sync must use **Tiingo** as the data source, using the `TIINGO_API_KEY` environment variable from `.env`. | Tiingo is the only remote data-source repository the indicator library ships with; the API key already exists. |
| C-04 | The endpoint must be a **POST** request. | Market-data sync is a state-mutating, potentially long-running operation. GET must remain idempotent and safe per HTTP semantics. |
| C-05 | The library uses Go `database/sql` with a pluggable `SQLRepositoryDialect` interface. No DuckDB dialect ships with the library. | A custom DuckDB/MotherDuck dialect must be written in the application layer, not patched into the library. |
| C-06 | The server currently uses only the Go standard library (`net/http`). New routes should follow the established pattern. | Keeps the dependency footprint small and consistent with the existing codebase conventions. |

## 3. Functional Requirements

### FR-01: POST /api/sync — Trigger Asset Synchronisation

**Description**

Expose a POST endpoint that triggers the `asset.Sync` workflow from the indicator library, pulling recent market snapshots from Tiingo and storing them into MotherDuck.

**Request**

```
POST /api/sync
Content-Type: application/json
```

Optional JSON body:

```json
{
  "assets": ["aapl", "msft", "googl"],
  "days": 90,
  "workers": 2
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `assets` | `[]string` | No | All assets known to the source repository | Ticker symbols to sync. |
| `days` | `int` | No | `365` | Look-back period in days for assets that have no local data yet. |
| `workers` | `int` | No | `1` | Number of concurrent sync workers. |
| `delay` | `int` | No | `5` | Delay in seconds between source GET requests (rate limiting). |

**Response — 202 Accepted**

```json
{
  "status": "accepted",
  "message": "Sync started",
  "assets": ["aapl", "msft"],
  "days": 90,
  "workers": 2,
  "timestamp": "2026-07-04T12:00:00Z"
}
```

**Response — 400 Bad Request**

```json
{
  "status": "error",
  "message": "Invalid request body",
  "timestamp": "2026-07-04T12:00:00Z"
}
```

**Response — 500 Internal Server Error**

```json
{
  "status": "error",
  "message": "Failed to initialize repositories",
  "timestamp": "2026-07-04T12:00:00Z"
}
```

### FR-02: Repository Wiring

The endpoint must construct:

1. **Source**: a `TiingoRepository` configured with the `TIINGO_API_KEY` from the environment.
2. **Target**: a `SQLRepository` backed by DuckDB/MotherDuck using the `MOTHERDUCK_URL` from the environment, with a custom DuckDB dialect that implements `SQLRepositoryDialect`.

Connection errors, missing environment variables, or dialect failures must result in a **500** response with a descriptive error message.

### FR-03: Asset Selection Logic

- If the request body is absent or `assets` is null/empty, sync **all assets** known to the target repository (MotherDuck). If MotherDuck is empty (first run), this is a no-op — the user must explicitly list assets for the initial sync.
- If `assets` is provided, sync only those tickers.

### FR-04: Idempotency & Safety

- The endpoint should not prevent concurrent sync requests at the application level (but the library's `Sync` itself uses worker coordination).
- A sync running simultaneously for the same asset should not corrupt data; MotherDuck/DuckDB handles transactional idempotency at the row level.

## 4. Non-Functional Requirements

| # | Requirement | Target |
|---|-------------|--------|
| NFR-01 | **Response time**: The endpoint MUST respond within 2 seconds (202 Accepted). The actual sync runs asynchronously in the background. | ≤ 2 s |
| NFR-02 | **Observability**: Sync progress, errors, and completion must be logged via `slog` (the library's logger is pluggable). | Structured logs |
| NFR-03 | **Security**: The endpoint must not expose the `TIINGO_API_KEY` or `MOTHERDUCK_URL` to the client. These are server-side env vars only. | No secrets in responses |
| NFR-04 | **Deployability**: Must work unchanged on Vercel Serverless Functions (current deployment target). | Same deploy flow |

## 5. Acceptance Criteria

| ID | Criterion | How to verify |
|----|-----------|---------------|
| AC-01 | A POST request to `/api/sync` with a valid body returns HTTP 202. | `curl -X POST -d '{"assets":["aapl"],"days":30}' localhost:3000/api/sync` → status 202 |
| AC-02 | A POST request to `/api/sync` without a body returns HTTP 202 and syncs using defaults. | `curl -X POST localhost:3000/api/sync` → status 202 |
| AC-03 | A POST request with an invalid JSON body returns HTTP 400. | `curl -X POST -d 'not-json' localhost:3000/api/sync` → status 400 |
| AC-04 | A GET request to `/api/sync` returns HTTP 405 Method Not Allowed. | `curl localhost:3000/api/sync` → status 405 |
| AC-05 | After sync, AAPL snapshots exist in the MotherDuck `snapshots` table. | Query MotherDuck: `SELECT COUNT(*) FROM snapshots WHERE name = 'aapl'` > 0 |
| AC-06 | The sync respects the `days` parameter: only assets with no local data look back `days` days. | Verified by log inspection or data recency check. |
| AC-07 | Missing `TIINGO_API_KEY` returns 500 with a clear error. | Unset env var, call endpoint → status 500 with "TIINGO_API_KEY" in message. |
| AC-08 | Missing `MOTHERDUCK_URL` returns 500 with a clear error. | Unset env var, call endpoint → status 500 with "MOTHERDUCK_URL" in message. |

## 6. Open Questions

| # | Question | Status |
|---|----------|--------|
| OQ-01 | Should the sync be truly async (goroutine with no client tracking) or should we add a job-tracking mechanism with a `/api/sync/{id}/status` endpoint? | **Needs BA/Architect decision.** Current scope: fire-and-forget (202 Accepted). |

## 7. Key Architectural Decisions (pre-design)

### 7.1 SQL Database Migration — MotherDuck / DuckDB

The indicator library's `NewSQLRepository` function **auto-creates the snapshots table** on every connection via `dialect.CreateTable()`:

```go
_, err = db.Exec(dialect.CreateTable())
```

The DuckDB dialect's `CreateTable()` method uses `CREATE TABLE IF NOT EXISTS snapshots (...)` plus `CREATE INDEX IF NOT EXISTS` — this acts as an **auto-migration on connect**. No formal migration tool (`goose`, `golang-migrate`, etc.) is required for the initial schema, because:

- The table schema is fixed and owned by the indicator library (`date`, `open`, `high`, `low`, `close`, `volume`).
- Both `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS` are idempotent.
- MotherDuck is DuckDB-compatible; the same SQL works.

A formal migration tool (like `goose`) would only be needed if the application layer needed to **add its own columns** or **manage schema versions independently** from the library. That decision belongs in the design phase.

### 7.2 Data Volume & Indexing

At ~252 trading days per year, 30 years of daily data produces:

| Assets | Total rows |
|--------|-----------|
| 1 | 7,560 |
| 14 | **105,840** (crosses 100K) |
| 50 | 378,000 |
| 100 | 756,000 |
| S&P 500 | ~3.8M |

A composite B-tree index on `(name, date)` is added to the `CreateTable()` DDL. This accelerates the three core queries (`GetSince`, `LastDate`, `Append`) without requiring partitioning.

DuckDB 1.5 (current MotherDuck version) does not expose `PARTITION BY LIST` in `CREATE TABLE`. Columnar storage with min-max row-group pruning provides additional filtering. Partitioning can be revisited if the dataset exceeds millions of rows and DuckDB adds DDL-level partitioning support.

### 7.2 Custom DuckDB SQLRepositoryDialect

Since the indicator library does **not** ship a DuckDB dialect, the application layer must create one:

```go
type DuckDBDialect struct{}

func (d *DuckDBDialect) CreateTable() string {
    return `CREATE TABLE IF NOT EXISTS snapshots (
        name TEXT NOT NULL,
        date DATE NOT NULL,
        open DOUBLE NOT NULL,
        high DOUBLE NOT NULL,
        low DOUBLE NOT NULL,
        close DOUBLE NOT NULL,
        volume DOUBLE NOT NULL
    )`
}
func (d *DuckDBDialect) DropTable() string   { return "DROP TABLE IF EXISTS snapshots" }
func (d *DuckDBDialect) Assets() string      { return "SELECT DISTINCT name FROM snapshots ORDER BY name" }
func (d *DuckDBDialect) GetSince() string    { return "SELECT date, open, high, low, close, volume FROM snapshots WHERE name = $1 AND date >= $2 ORDER BY date" }
func (d *DuckDBDialect) LastDate() string    { return "SELECT MAX(date) FROM snapshots WHERE name = $1" }
func (d *DuckDBDialect) Append() string      { return "INSERT INTO snapshots (name, date, open, high, low, close, volume) VALUES ($1, $2, $3, $4, $5, $6, $7)" }
```

### 7.3 Custom Repository Registration

The application will register a `"motherduck"` repository builder via `asset.RegisterRepositoryBuilder("motherduck", ...)` so the sync can construct a `SQLRepository` by name.

## 8. Out of Scope (for this feature)

- Rate-limiting or queuing of concurrent sync requests.
- WebSocket or SSE-based progress streaming.
- Persistent sync-job status tracking.
- UI for triggering sync.
- Deleting or purging snapshots via API.
- Authentication/authorization on the endpoint.
- Support for any data source other than Tiingo.
- Support for any data target other than MotherDuck (via DuckDB dialect).
