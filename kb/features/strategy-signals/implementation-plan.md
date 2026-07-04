---
title: Strategy Management, Signal Generation & Backtesting — Implementation Plan
feature_id: F-005
artifact: implementation-plan
status: draft
version: 1.0.0
owner_agent: Developer
parent_feature: F-003
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: Architect
    description: Initial implementation plan for F-005 with 5 phases.
---

# Implementation Plan — Strategy Management, Signal Generation & Backtesting (F-005)

## Phase Overview

| Phase | Focus | Files | Est. Effort |
|-------|-------|-------|-------------|
| **Phase 1** | Database migrations + model types | `internal/database/migrations.go`, `internal/model/strategy.go`, `internal/model/signal.go`, `internal/model/backtest.go` | Small |
| **Phase 2** | Strategy catalog + factory | `internal/strategy/catalog.go`, `internal/strategy/factory.go`, `internal/strategy/catalog_test.go` | Medium |
| **Phase 3** | Signal generation + repository | `internal/signal/generator.go`, `internal/signal/repository.go`, test files | Medium |
| **Phase 4** | Backtest runner + repository | `internal/backtest/runner.go`, `internal/backtest/repository.go`, test files | Medium |
| **Phase 5** | REST handlers + route registration | `internal/handler/strategy_handler.go`, `internal/handler/signal_handler.go`, `internal/handler/backtest_handler.go`, modify `internal/handler/handler.go` | Medium |
| **Phase 6** | MCP tools | `internal/mcp/strategy_tools.go`, `internal/mcp/signal_tools.go`, `internal/mcp/backtest_tools.go`, modify `internal/mcp/tools.go` | Small |
| **Phase 7** | Wire up startup + deploy | Modify `cmd/server/main.go`, `api/index.go`, `go.mod` | Small |

---

## Phase 1: Database Migrations + Model Types

### Step 1.1: Create `internal/database/migrations.go`

Dedicated package for idempotent DDL that runs at startup.

```go
package database

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5"
)

const strategiesDDL = `...`
const signalsDDL = `...`
const backtestResultsDDL = `...`

var Migrations = []string{strategiesDDL, signalsDDL, backtestResultsDDL}

func RunMigrations(databaseURL string) error {
    conn, err := pgx.Connect(context.Background(), databaseURL)
    if err != nil { return err }
    defer conn.Close(context.Background())
    for i, ddl := range Migrations {
        if _, err := conn.Exec(context.Background(), ddl); err != nil {
            return fmt.Errorf("migration %d failed: %w", i+1, err)
        }
    }
    return nil
}
```

**DDL Statements:**

1. `strategies` table — `CREATE TABLE IF NOT EXISTS ...`
2. `signals` table — `CREATE TABLE IF NOT EXISTS ...`
3. `backtest_results` table — `CREATE TABLE IF NOT EXISTS ...`

(Full DDL from design.md §3.)

### Step 1.2: Create model type files

- `internal/model/strategy.go` — `StrategyTypeEntry`, `StrategyTypesResponse`, `StrategyCreateRequest`, `StrategyResponse`, `StrategyListResponse`, `CategorySummary`
- `internal/model/signal.go` — `SignalGenerateRequest`, `SignalRecord`, `SignalGenerateResponse`, `SignalQueryResponse`
- `internal/model/backtest.go` — `BacktestRunRequest`, `BacktestRunResponse`, `BacktestResultResponse`, `BacktestResultsQueryResponse`

### Step 1.3: Add `StrategyRepository` to existing DB pattern

A `StrategyRepository` for CRUD on the `strategies` table. Reuses `pgx/v5` directly (same pattern as signal/backtest repositories — no ORM).

**Functions:**
- `Insert(ctx, s StrategyRecord) (int, error)` — INSERT, return new ID
- `GetByID(ctx, id int) (*StrategyRecord, error)`
- `GetAll(ctx, filter StrategyFilter) ([]StrategyRecord, int, error)` — with filtering
- `GetByType(ctx, strategyType string) ([]StrategyRecord, error)`

**Verification:** `go build ./...` compiles. Unit test creates/fetches strategies using `pgx` with a real MotherDuck connection or mocked via interface.

---

## Phase 2: Strategy Catalog + Factory

### Step 2.1: Create `internal/strategy/catalog.go`

Define the `Catalog` map with all 30+ strategy types. Each entry has:
- Type key, display name, category, description, default parameters
- A constructor function `func(map[string]any) (strategy.Strategy, error)`

**Strategy types to register:**

| Key | Constructor Pattern | Params |
|-----|-------------------|--------|
| `buy_and_hold_strategy` | `NewBuyAndHoldStrategy()` | none |
| `and_strategy` | `NewAndStrategy(...)` | sub-strategies |
| `or_strategy` | `NewOrStrategy(...)` | sub-strategies |
| `majority_strategy` | `NewMajorityStrategy(...)` | sub-strategies |
| `split_strategy` | `NewSplitStrategy(buy, sell)` | sub-strategies |
| `rsi_strategy` | `NewRSIStrategy(period)` | period, oversold, overbought |
| `stochastic_oscillator_strategy` | `NewStochasticOscillatorStrategy(k, d)` | k, d |
| `stochastic_rsi_strategy` | `NewStochasticRSIStrategy(k, d, rsiPeriod, stochasticPeriod)` | k, d, rsi_period, stochastic_period |
| `williams_r_strategy` | `NewWilliamsRStrategy(period)` | period |
| `awesome_oscillator_strategy` | `NewAwesomeOscillatorStrategy()` | none |
| `coppock_curve_strategy` | `NewCoppockCurveStrategy()` | none |
| `elder_ray_strategy` | `NewElderRayStrategy(period, maType)` | period, ma_type |
| `ichimoku_cloud_strategy` | `NewIchimokuCloudStrategy(conversion, base, span)` | conversion, base, span |
| `triple_rsi_strategy` | `NewTripleRSIStrategy()` | none |
| `golden_cross_strategy` | `NewGoldenCrossStrategy(short, long)` | short_period, long_period |
| `macd_strategy` | `NewMACDStrategy()` | none (uses MACD defaults: 12, 26, 9) |
| `bollinger_bands_strategy` | `NewBollingerBandsStrategy(period, stddev)` | period, stddev |
| `super_trend_strategy` | `NewSuperTrendStrategy(period, multiplier)` | period, multiplier |
| `donchian_channel_breakout_strategy` | `NewDonchianChannelBreakoutStrategy(period)` | period |
| `keltner_channel_strategy` | `NewKeltnerChannelStrategy(period, stddev)` | period, stddev |
| `obv_strategy` | `NewOBVStrategy()` | none |
| `chaikin_money_flow_strategy` | `NewChaikinMoneyFlowStrategy(period)` | period |
| `money_flow_index_strategy` | `NewMoneyFlowIndexStrategy(period)` | period |
| ... and all remaining strategies from the trend/momentum/volatility/volume packages |

**Helper functions:**
- `getParamInt(params map[string]any, key string, defaultVal int) int`
- `getParamFloat(params map[string]any, key string, defaultVal float64) float64`
- `getParamString(params map[string]any, key string, defaultVal string) string`

### Step 2.2: Create `internal/strategy/factory.go`

```go
package strategy

import "github.com/cinar/indicator/v2/strategy"

func Instantiate(strategyType string, params map[string]any) (strategy.Strategy, error) {
    def, ok := Catalog[strategyType]
    if !ok {
        return nil, fmt.Errorf("unknown strategy type: %s", strategyType)
    }
    return def.Constructor(params)
}

func ListTypes() []StrategyTypeEntry { ... }
func Categories() map[string]CategorySummary { ... }
func IsValid(strategyType string) bool { ... }
```

### Step 2.3: Write `internal/strategy/catalog_test.go`

- Test that all 30+ strategy types can be instantiated with default parameters
- Test that invalid type returns error
- Test that bad parameters return meaningful errors
- Test edge cases (zero values, missing params)

**Verification:** `go test ./internal/strategy/...` passes.

---

## Phase 3: Signal Generation + Repository

### Step 3.1: Create `internal/signal/generator.go`

```go
package signal

import (
    "context"
    "time"
    "github.com/cinar/indicator/v2/asset"
    "github.com/cinar/indicator/v2/helper"
    "github.com/cinar/indicator/v2/strategy"
)

type SignalRecord struct {
    StrategyID   int
    StrategyType string
    Underlying   string
    SignalDate   time.Time
    Action       string  // "buy", "sell", "hold"
    Price        float64
}

func Generate(
    ctx context.Context,
    st strategy.Strategy,
    snapshots []*asset.Snapshot,
    strategyID int,
    strategyType string,
    underlying string,
) ([]SignalRecord, error)
```

**Algorithm:**
1. Convert `snapshots` to channel via `helper.SliceToChan(snapshots)`
2. Duplicate the channel: one for strategy, one for price extraction
3. Call `strategy.ComputeWithOutcomeWithContext(ctx, st, snapshots[0])` → `actions`, `outcomes`
4. Drain `outcomes` (needed to unblock the pipeline)
5. Collect actions and match each to the corresponding snapshot's `Date` and `Close` price
6. Map `Action` enum to string: `Hold → "hold"`, `Buy → "buy"`, `Sell → "sell"`
7. Return `[]SignalRecord`

### Step 3.2: Create `internal/signal/repository.go`

```go
package signal

type Repository struct {
    databaseURL string
}

func NewRepository(databaseURL string) *Repository

func (r *Repository) ExistingSignals(ctx, strategyID, underlying, startDate, endDate) (bool, error)
func (r *Repository) GetSignals(ctx, filter) ([]SignalRecord, int, error)
func (r *Repository) InsertSignals(ctx, records []SignalRecord) error
func (r *Repository) DeleteSignals(ctx, strategyID, underlying, startDate, endDate) error
```

**Implementation notes:**
- `ExistingSignals`: `SELECT COUNT(*) FROM signals WHERE strategy_id=$1 AND underlying=$2 AND signal_date>=$3 AND signal_date<=$4`
- `InsertSignals`: Batch INSERT with `ON CONFLICT (strategy_id, underlying, signal_date) DO NOTHING`
- `GetSignals`: Dynamic SQL with optional filters, `LIMIT`/`OFFSET`, total via separate `COUNT(*)`
- `DeleteSignals`: `DELETE FROM signals WHERE strategy_id=$1 AND underlying=$2 AND signal_date>=$3 AND signal_date<=$4`

### Step 3.3: Write test files

- `internal/signal/generator_test.go` — Test with `BuyAndHoldStrategy` and synthetic snapshots; verify signals are produced in correct order
- `internal/signal/repository_test.go` — Test CRUD operations (requires MotherDuck connection or mocked)

**Verification:** `go test ./internal/signal/...` passes.

---

## Phase 4: Backtest Runner + Repository

### Step 4.1: Create `internal/backtest/runner.go`

```go
package backtest

import (
    "context"
    "time"
    "github.com/cinar/indicator/v2/asset"
    "github.com/cinar/indicator/v2/backtest"
    cindicator "github.com/cinar/indicator/v2/strategy"
)

type Result struct {
    StrategyID      int
    StrategyType    string
    Underlying      string
    StartDate       time.Time
    EndDate         time.Time
    TotalReturn     float64
    MaxDrawdown     float64
    SharpeRatio     float64
    WinRate         float64
    NumTransactions int
    FinalOutcome    float64
    FinalAction     string
    Parameters      map[string]any
}

func Run(
    ctx context.Context,
    st cindicator.Strategy,
    snapshots []*asset.Snapshot,
    strategyID int,
    strategyType string,
    underlying string,
    params map[string]any,
) (*Result, error)
```

**Algorithm:**
1. Create `backtest.NewDataReport()`
2. Create a minimal backtest instance: `b := backtest.NewBacktest(nil, dataReport)` — we control the data directly
3. Manually run the backtest for a single strategy + asset:
   - Create a snapshot channel from the slice
   - Split: `helper.Duplicate(snapshotChan, 2)`
   - Call `strategy.ComputeWithOutcome(st, snapshots[0])` → actions, outcomes
   - Fill `DataReport.Write()` with the data
4. Extract `*backtest.DataStrategyResult` from the data report
5. Compute derived metrics:
   - `TotalReturn` = `result.Outcome` (this is P&L from $1 initial)
   - `MaxDrawdown` = compute from outcome stream: peak - trough / peak
   - `SharpeRatio` = mean(daily_return) / std(daily_return) * sqrt(252)
   - `WinRate` = profitable trades / total trades
   - `NumTransactions` = count of Buy/Sell actions
   - `FinalAction` = map `result.Action` to string
6. Return `*Result`

### Step 4.2: Create `internal/backtest/repository.go`

```go
type Repository struct {
    databaseURL string
}

func NewRepository(databaseURL string) *Repository

func (r *Repository) ExistingResult(ctx, strategyID, underlying, startDate, endDate) (*Result, error)
func (r *Repository) InsertResult(ctx, result *Result) error
func (r *Repository) DeleteResult(ctx, strategyID, underlying, startDate, endDate) error
func (r *Repository) GetResults(ctx, filter) ([]Result, int, error)
```

### Step 4.3: Write test files

- `internal/backtest/runner_test.go` — Test with `BuyAndHoldStrategy`; expected outcome ≈ market return
- `internal/backtest/repository_test.go` — Test CRUD

**Verification:** `go test ./internal/backtest/...` passes.

---

## Phase 5: REST Handlers + Route Registration

### Step 5.1: Create `internal/handler/strategy_handler.go`

Handlers (all methods on `*Handler`):

| Method | Signature | Purpose |
|--------|-----------|---------|
| `handleListStrategyTypes` | `(w, r)` | GET /api/strategies/types — return strategy catalog |
| `handleListStrategies` | `(w, r)` | GET /api/strategies — query saved strategies |
| `handleCreateStrategy` | `(w, r)` | POST /api/strategies — create a new strategy |
| `handleStrategyByID` | `(w, r)` | POST /api/strategies/{id}/... — dispatch to signals or backtest |

**handleListStrategyTypes:**
- No auth required
- Returns `StrategyTypesResponse` from the in-memory catalog (no DB call)

**handleListStrategies:**
- No auth required
- Parses query params (`strategy_type`, `underlying`, `name`)
- Calls `StrategyRepository.GetAll()`
- Returns `StrategyListResponse`

**handleCreateStrategy:**
- Bearer auth required
- Parses JSON body into `StrategyCreateRequest`
- Validates: name required + unique, strategy_type known, underlying normalized to uppercase
- Creates strategy via `StrategyRepository.Insert()`
- Returns 201 + `StrategyResponse`

**handleStrategyByID:**
- Parses strategy ID from URL path
- Path routing:
  - `POST /api/strategies/{id}/signals` → delegate to handleGenerateSignals
  - `POST /api/strategies/{id}/backtest` → delegate to handleRunBacktest

### Step 5.2: Create `internal/handler/signal_handler.go`

| Method | Signature | Purpose |
|--------|-----------|---------|
| `handleGenerateSignals` | `(w, r, id)` | POST /api/strategies/{id}/signals |
| `handleQuerySignals` | `(w, r)` | GET /api/signals |

**handleGenerateSignals:**
- Bearer auth required
- Loads strategy from DB by ID
- Determines date range from strategy's `lookback_days`
- Checks existing signals — returns cached if exists and `force=false`
- Fetches snapshots from MotherDuck via `asset.NewRepository("motherduck", ...)`
- Instantiates strategy via `strategy.Instantiate()`
- Calls `signal.Generate()`
- Batch inserts via `signal.Repository.InsertSignals()`
- Returns `SignalGenerateResponse`

**handleQuerySignals:**
- No auth required
- Parses query params: `strategy_id`, `strategy_type`, `underlying`, `date_from`, `date_to`, `action`, `limit`, `offset`
- Calls `signal.Repository.GetSignals()`
- Returns `SignalQueryResponse`

### Step 5.3: Create `internal/handler/backtest_handler.go`

| Method | Signature | Purpose |
|--------|-----------|---------|
| `handleRunBacktest` | `(w, r, id)` | POST /api/strategies/{id}/backtest |
| `handleQueryBacktestResults` | `(w, r)` | GET /api/backtest-results |

**handleRunBacktest:**
- Bearer auth required
- Same pattern as signal generation: load strategy → check cache → fetch snapshots → run → store → return

**handleQueryBacktestResults:**
- No auth required
- Parses query params: `strategy_id`, `strategy_type`, `underlying`, `date_from`, `date_to`, `min_return`, `limit`, `offset`
- Calls `backtest.Repository.GetResults()`
- Returns `BacktestResultsQueryResponse`

### Step 5.4: Modify `internal/handler/handler.go`

Add new routes to `RegisterRoutes()`:

```go
mux.HandleFunc("/api/strategies/types",       h.handleListStrategyTypes)
mux.HandleFunc("/api/strategies",              h.handleListStrategies)     // GET
mux.HandleFunc("/api/strategies",              h.handleCreateStrategy)     // POST
mux.HandleFunc("/api/strategies/",             h.handleStrategyByID)       // POST sub-routes
mux.HandleFunc("/api/signals",                 h.handleQuerySignals)
mux.HandleFunc("/api/backtest-results",        h.handleQueryBacktestResults)
```

Note: `GET` and `POST` on `/api/strategies` share the same path. The handler dispatches on `r.Method` internally.

### Step 5.5: Write handler tests

- `internal/handler/strategy_handler_test.go`
- `internal/handler/signal_handler_test.go`
- `internal/handler/backtest_handler_test.go`

Use `httptest.NewRecorder()` and `httptest.NewRequest()`. Mock the repository layer where needed.

**Verification:** `go test ./internal/handler/...` passes.

---

## Phase 6: MCP Tools

### Step 6.1: Create MCP tool handler files

- `internal/mcp/strategy_tools.go` — `handleListStrategyTypes`, `handleListStrategies`, `handleCreateStrategy`
- `internal/mcp/signal_tools.go` — `handleGenerateSignals`, `handleQuerySignals`
- `internal/mcp/backtest_tools.go` — `handleRunBacktest`, `handleQueryBacktestResults`

Each handler:
1. Takes typed input struct
2. Calls the same `internal/strategy`, `internal/signal`, `internal/backtest` packages as REST handlers
3. Returns `*mcp.CallToolResult` via `textResult()` helper

**Input structs (with `jsonschema` tags):**

```go
type CreateStrategyInput struct {
    Name         string         `json:"name"         jsonschema:"required,strategy name, must be unique"`
    StrategyType string         `json:"strategy_type" jsonschema:"required,strategy type key from list_strategy_types"`
    Underlying   string         `json:"underlying"   jsonschema:"required,ticker symbol"`
    Timeframe    string         `json:"timeframe"    jsonschema:"data interval, default 1d"`
    LookbackDays int            `json:"lookback_days" jsonschema:"lookback days, default 365"`
    Parameters   map[string]any `json:"parameters"   jsonschema:"strategy-specific parameters"`
}

type GenerateSignalsInput struct {
    StrategyID int  `json:"strategy_id" jsonschema:"required,strategy ID"`
    Force      bool `json:"force"       jsonschema:"force regeneration, default false"`
}

type RunBacktestInput struct {
    StrategyID int  `json:"strategy_id" jsonschema:"required,strategy ID"`
    Force      bool `json:"force"       jsonschema:"force rerun, default false"`
}

type QuerySignalsInput struct {
    StrategyID   int    `json:"strategy_id"   jsonschema:"filter by strategy ID"`
    StrategyType string `json:"strategy_type" jsonschema:"filter by strategy type"`
    Underlying   string `json:"underlying"    jsonschema:"filter by ticker symbol"`
    DateFrom     string `json:"date_from"     jsonschema:"start date ISO format"`
    DateTo       string `json:"date_to"       jsonschema:"end date ISO format"`
    Action       string `json:"action"        jsonschema:"filter by action: buy, sell, hold"`
    Limit        int    `json:"limit"         jsonschema:"max results, default 1000"`
    Offset       int    `json:"offset"        jsonschema:"pagination offset"`
}

type QueryBacktestResultsInput struct {
    StrategyID   int     `json:"strategy_id"   jsonschema:"filter by strategy ID"`
    StrategyType string  `json:"strategy_type" jsonschema:"filter by strategy type"`
    Underlying   string  `json:"underlying"    jsonschema:"filter by ticker symbol"`
    DateFrom     string  `json:"date_from"     jsonschema:"min backtest end date"`
    DateTo       string  `json:"date_to"       jsonschema:"max backtest end date"`
    MinReturn    float64 `json:"min_return"    jsonschema:"minimum total return filter"`
    Limit        int     `json:"limit"         jsonschema:"max results, default 100"`
    Offset       int     `json:"offset"        jsonschema:"pagination offset"`
}
```

### Step 6.2: Modify `internal/mcp/tools.go`

Add tool registrations in `registerTools()`:

```go
mcp.AddTool(server, &mcp.Tool{Name: "list_strategy_types", Description: "...", Input: ...}, handleListStrategyTypes)
mcp.AddTool(server, &mcp.Tool{Name: "list_strategies", Description: "...", Input: ...}, handleListStrategies)
mcp.AddTool(server, &mcp.Tool{Name: "create_strategy", Description: "...", Input: ...}, handleCreateStrategy)
mcp.AddTool(server, &mcp.Tool{Name: "generate_signals", Description: "...", Input: ...}, handleGenerateSignals)
mcp.AddTool(server, &mcp.Tool{Name: "run_backtest", Description: "...", Input: ...}, handleRunBacktest)
mcp.AddTool(server, &mcp.Tool{Name: "query_signals", Description: "...", Input: ...}, handleQuerySignals)
mcp.AddTool(server, &mcp.Tool{Name: "query_backtest_results", Description: "...", Input: ...}, handleQueryBacktestResults)
```

### Step 6.3: Write MCP tool tests

- `internal/mcp/strategy_tools_test.go`
- `internal/mcp/signal_tools_test.go`
- `internal/mcp/backtest_tools_test.go`

Each test:
1. Starts the MCP server with test tools registered
2. Sends JSON-RPC `tools/call` via HTTP
3. Verifies SSE-formatted response (`event: message\ndata: {...}`)
4. Parses the result and validates fields

**Verification:** `go test ./internal/mcp/...` passes (existing 12 tests + new tests).

---

## Phase 7: Wire Up Startup + Deploy

### Step 7.1: Modify `cmd/server/main.go`

Add database migration call before starting the server:

```go
import "vercel-go-starter/internal/database"

func main() {
    databaseURL := os.Getenv("DATABASE_URL")
    if databaseURL != "" {
        if err := database.RunMigrations(databaseURL); err != nil {
            slog.Error("Migration failed", "error", err)
            os.Exit(1)
        }
    }
    // ... existing startup code
}
```

### Step 7.2: Modify `api/index.go`

Same migration call for Vercel serverless:

```go
import "vercel-go-starter/internal/database"

func init() {
    databaseURL := os.Getenv("DATABASE_URL")
    if databaseURL != "" {
        if err := database.RunMigrations(databaseURL); err != nil {
            slog.Error("Migration failed", "error", err)
        }
    }
}
```

### Step 7.3: Build + Test

```bash
go build ./...
go test ./...       # All existing + new tests pass
go vet ./...
```

### Step 7.4: Deploy to Vercel

```bash
vercel --prod
```

### Step 7.5: Verify

```bash
# List strategy types
curl https://tech-indicator.vercel.app/api/strategies/types

# Create a strategy
curl -X POST https://tech-indicator.vercel.app/api/strategies \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test RSI","strategy_type":"rsi_strategy","underlying":"AAPL","lookback_days":365,"parameters":{"period":14}}'

# Generate signals
curl -X POST https://tech-indicator.vercel.app/api/strategies/1/signals \
  -H "Authorization: Bearer $KEY"

# Run backtest
curl -X POST https://tech-indicator.vercel.app/api/strategies/1/backtest \
  -H "Authorization: Bearer $KEY"

# Query signals
curl "https://tech-indicator.vercel.app/api/signals?strategy_id=1"

# Query backtest results
curl "https://tech-indicator.vercel.app/api/backtest-results?strategy_id=1"
```

---

## Dependency Graph

```
Phase 1 (Models + Migrations)
    ↓
Phase 2 (Strategy Catalog)
    ↓
Phase 3 (Signal Generator + Repository) ── depends on Phase 1, 2
    ↓
Phase 4 (Backtest Runner + Repository)  ── depends on Phase 1, 2
    ↓
Phase 5 (REST Handlers) ── depends on Phase 3, 4
    ↓
Phase 6 (MCP Tools) ── depends on Phase 5
    ↓
Phase 7 (Wire Up + Deploy) ── depends on Phase 5, 6
```

Phases 3 and 4 can be implemented in parallel since they share only Phase 1 and 2 as dependencies.
