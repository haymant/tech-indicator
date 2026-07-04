---
title: Sync Endpoint — Technical Design
feature_id: F-001
artifact: design
status: draft
version: 1.0.0
owner_agent: Architect
parent_feature: F-001
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: Architect
    description: Initial design based on requirements F-001.
---

# Design — Sync Endpoint POST /api/sync

## 1. Design Summary

Add a single POST endpoint to the existing Go `net/http` server that triggers an on-demand market-data sync from Tiingo into MotherDuck using the indicator library's `asset.Sync` engine. The endpoint returns 202 immediately (fire-and-forget). The sync runs in a background goroutine.

## 2. Components & Interfaces

### 2.1 New Packages

```
internal/
├── repository/
│   ├── duckdb_dialect.go    # DuckDB/MotherDuck SQL dialect
│   ├── duckdb_dialect_test.go
│   └── motherduck.go        # Registers "motherduck" repository builder
└── model/
    └── sync.go              # Sync request/response types (new file)
```

### 2.2 Changes to Existing Files

| File | Change |
|------|--------|
| `internal/handler/handler.go` | Add `POST /api/sync` route; register motherduck builder in `New()` |
| `internal/handler/handler.go` | Add `handleSync` method |
| `cmd/server/main.go` | No changes needed (handler already wired) |

### 2.3 DuckDBDialect

Implements `asset.SQLRepositoryDialect` with DuckDB/MotherDuck-compatible DML.

```
┌─────────────────────────────────────────────┐
│             DuckDBDialect                    │
├─────────────────────────────────────────────┤
│ + CreateTable() string                       │
│ + DropTable()   string                       │
│ + Assets()      string                       │
│ + GetSince()    string                       │
│ + LastDate()    string                       │
│ + Append()      string                       │
└─────────────────────────────────────────────┘
```

### 2.4 Repository Registration

On server startup, register the `"motherduck"` repository builder via `asset.RegisterRepositoryBuilder("motherduck", builder)`. The builder receives the `MOTHERDUCK_URL` as config and returns an `*asset.SQLRepository` backed by the `duckdb` driver with `DuckDBDialect`.

### 2.5 Sync Handler

```
POST /api/sync
  Request body (optional JSON):
    { "assets": [...], "days": N, "workers": N, "delay": N }

  Handler logic:
    1. Read TIINGO_API_KEY and MOTHERDUCK_URL from env
    2. If either is missing → 500 with field name in message
    3. Parse request body (absent → use defaults)
    4. Construct TiingoRepository (source)
    5. Construct SQLRepository via NewRepository("motherduck", url) (target)
    6. Configure asset.Sync with requested params
    7. Launch sync.Run in a goroutine
    8. Return 202 Accepted

  Response:
    202: { "status": "accepted", "message": "Sync started", ... }
    400: { "status": "error", "message": "..." }
    405: Method not allowed (if not POST)
    500: { "status": "error", "message": "..." }
```

### 2.6 Request/Response Models

```go
// SyncRequest is the optional JSON body for POST /api/sync.
type SyncRequest struct {
    Assets  []string `json:"assets,omitempty"`
    Days    int      `json:"days,omitempty"`
    Workers int      `json:"workers,omitempty"`
    Delay   int      `json:"delay,omitempty"`
}

// SyncResponse is returned on 202 Accepted.
type SyncResponse struct {
    Status    string   `json:"status"`
    Message   string   `json:"message"`
    Assets    []string `json:"assets,omitempty"`
    Days      int      `json:"days,omitempty"`
    Workers   int      `json:"workers,omitempty"`
    Timestamp string   `json:"timestamp"`
}

// ErrorResponse is returned on 4xx/5xx.
type ErrorResponse struct {
    Status    string `json:"status"`
    Message   string `json:"message"`
    Timestamp string `json:"timestamp"`
}
```

## 3. Data Flow

```
┌──────────┐   POST /api/sync    ┌──────────────────┐
│  Client  │ ──────────────────►  │  handleSync      │
└──────────┘                     │  (handler)        │
                                 └────────┬─────────┘
                                          │
                          ┌───────────────┼───────────────┐
                          │               │               │
                          ▼               ▼               ▼
                   ┌───────────┐   ┌───────────┐   ┌──────────┐
                   │ os.Getenv │   │ json.Decode│   │ asset.   │
                   │ TIINGO_KEY│   │ SyncRequest│   │ NewSync()│
                   │ MOTHERDUCK│   └───────────┘   └────┬─────┘
                   └─────┬─────┘                        │
                         │                              │
                         ▼                              ▼
                   ┌───────────┐                 ┌──────────────┐
                   │ TiingoRepo│                 │ asset.Sync   │
                   │ (source)  │                 │ .Run(source, │
                   │ SQLRepo   │                 │  target, dt) │
                   │ (target)  │                 └──────┬───────┘
                   └───────────┘                        │
                                          ┌─────────────┼─────────────┐
                                          │  goroutine  │             │
                                          ▼                          │
                                    Tiingo API ◄──── GET /tiingo/... │
                                          │                          │
                                          ▼                          │
                                    MotherDuck ◄──── INSERT INTO     │
                                                      snapshots      │
                                                                    │
                                                          202 returned
                                                          immediately
```

## 4. MotherDuck / DuckDB Dialect

The indicator library's `SQLRepository` auto-creates the table on connect via `dialect.CreateTable()`. For DuckDB, this uses `CREATE TABLE IF NOT EXISTS`, which is idempotent. No external migration tool is required for the library-owned schema.

### Index

A composite B-tree index on `(name, date)` accelerates the primary query pattern. At ~252 rows/year/asset, 100K rows is reached at just 14 assets over 30 years, so the index is essential from day one. DuckDB's columnar storage with min-max row-group pruning further reduces scan costs.

**Partitioning note**: DuckDB 1.5 (current) does not expose `PARTITION BY LIST` in `CREATE TABLE` for MotherDuck. The composite index provides sufficient pruning — partitioning can be revisited if the dataset exceeds millions of rows and DuckDB's version adds DDL-level partitioning support.

### SQL Statements

```sql
-- CreateTable (idempotent — includes index)
CREATE TABLE IF NOT EXISTS snapshots (
    name   TEXT NOT NULL,
    date   DATE NOT NULL,
    open   DOUBLE NOT NULL,
    high   DOUBLE NOT NULL,
    low    DOUBLE NOT NULL,
    close  DOUBLE NOT NULL,
    volume DOUBLE NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_snapshots_name_date ON snapshots (name, date);

-- DropTable
DROP TABLE IF EXISTS snapshots;

-- Assets
SELECT DISTINCT name FROM snapshots ORDER BY name;

-- GetSince (params: $1 = name, $2 = date)
SELECT date, open, high, low, close, volume
FROM snapshots
WHERE name = $1 AND date >= $2
ORDER BY date;

-- LastDate (params: $1 = name)
SELECT MAX(date) FROM snapshots WHERE name = $1;

-- Append (params: $1..$7 = name, date, open, high, low, close, volume)
INSERT INTO snapshots
    (name, date, open, high, low, close, volume)
VALUES ($1, $2, $3, $4, $5, $6, $7);
```

## 5. Failure Modes

| Failure | Symptom | Response |
|---------|---------|----------|
| `TIINGO_API_KEY` not set | Env var empty | 500 with `"TIINGO_API_KEY not set"` |
| `MOTHERDUCK_URL` not set | Env var empty | 500 with `"MOTHERDUCK_URL not set"` |
| Invalid JSON body | json.Decode error | 400 with `"Invalid request body"` |
| MotherDuck connection failure | `NewSQLRepository` error | 500 with connection error message |
| Tiingo API failure | `sync.Run` logs error | 202 still returned; errors in logs |
| Method is not POST | r.Method != "POST" | 405 Method Not Allowed |

## 6. Non-Functional

| Aspect | Decision |
|--------|----------|
| Concurrency | Fire-and-forget goroutine; no mutex, no queue |
| Logging | `slog.Default()` passed to sync; handler uses `slog` for errors |
| Latency | Returns 202 within <50ms (no blocking on sync) |
| Secrets | Only server-side env vars; never sent to client |
| Portability | Pure standard library + Go modules; no framework dependency |

## 7. Test Strategy Inputs

- **Unit tests** for `DuckDBDialect`: verify each method returns correct SQL string.
- **Unit tests** for `handleSync`: test with mockable env vars and verify response codes.
- **Integration test** (manual or CI): actual POST to running server with valid env vars.

## 8. Boundary Notes

- This feature is **pure Go** (no Next.js, no Python). The hybrid-stack boundary is not invoked.
- The `indicator/` library is treated as a **read-only dependency** — no modifications.
- MotherDuck is the **only target** supported. Other SQL databases would need their own dialect.
