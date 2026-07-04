---
title: Strategy Management, Signal Generation & Backtesting — Design
feature_id: F-005
artifact: design
status: draft
version: 1.0.0
owner_agent: Architect
parent_feature: F-003
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: Architect
    description: Initial technical design for F-005 strategy/signal/backtest subsystem.
---

# Design — Strategy Management, Signal Generation & Backtesting (F-005)

## 1. Architecture Overview

### 1.1 New Packages

```
internal/
├── strategy/              # NEW — Strategy registry, catalog, instantiation
│   ├── catalog.go         #   Strategy catalog from cinar/indicator library
│   ├── factory.go         #   Instantiate strategy by type + parameters
│   └── catalog_test.go
├── signal/                # NEW — Signal generation and persistence
│   ├── generator.go       #   Run strategy → produce signals
│   ├── generator_test.go
│   ├── repository.go      #   CRUD for signals table
│   └── repository_test.go
├── backtest/              # NEW — Backtesting wrapper and persistence
│   ├── runner.go          #   Wrap cinar/indicator backtest → extract metrics
│   ├── runner_test.go
│   ├── repository.go      #   CRUD for backtest_results table
│   └── repository_test.go
├── handler/
│   ├── handler.go         #   MODIFIED — Register new routes
│   ├── strategy_handler.go #  NEW — GET/POST /api/strategies, GET /api/strategies/types
│   ├── signal_handler.go  #   NEW — POST /api/strategies/{id}/signals, GET /api/signals
│   └── backtest_handler.go #  NEW — POST /api/strategies/{id}/backtest, GET /api/backtest-results
├── mcp/
│   ├── tools.go           #   MODIFIED — Register 7 new MCP tools
│   ├── strategy_tools.go  #   NEW — Strategy MCP tool handlers
│   ├── signal_tools.go    #   NEW — Signal MCP tool handlers
│   └── backtest_tools.go  #   NEW — Backtest MCP tool handlers
├── model/
│   ├── model.go           #   MODIFIED — Add strategy/signal/backtest types
│   ├── strategy.go        #   NEW — Strategy request/response types
│   ├── signal.go          #   NEW — Signal request/response types
│   └── backtest.go        #   NEW — Backtest request/response types
└── database/
    └── migrations.go      #   NEW — DDL for strategies, signals, backtest_results tables
```

### 1.2 Data Flow Diagrams

#### Signal Generation Flow

```
Client
  │ POST /api/strategies/{id}/signals
  ▼
StrategyHandler.generateSignals()
  │
  ├─► Load strategy from `strategies` table (by id)
  ├─► Check `signals` table for existing signals
  │     └─► If found and !force → return cached signals (early exit)
  ├─► Fetch OHLCV snapshots from `snapshots` table
  ├─► Instantiate strategy via strategy.Factory
  ├─► Run strategy.Compute() → channel of Actions
  ├─► Collect actions + prices into signal records
  ├─► Batch INSERT into `signals` table
  └─► Return generated signals
```

#### Backtest Flow

```
Client
  │ POST /api/strategies/{id}/backtest
  ▼
BacktestHandler.runBacktest()
  │
  ├─► Load strategy from `strategies` table
  ├─► Check `backtest_results` for existing result
  │     └─► If found and !force → return cached metrics
  ├─► Fetch OHLCV snapshots
  ├─► Instantiate strategy
  ├─► Run backtest.NewBacktest + backtest.DataReport
  ├─► Extract metrics (total_return, max_drawdown, etc.)
  ├─► INSERT into `backtest_results` table
  └─► Return performance/risk metrics
```

### 1.3 Strategy Catalog Population

The strategy catalog is populated at package init time by scanning the cinar/indicator library's known strategies. A `Catalog` struct maps strategy type keys to their metadata (name, category, description, default parameters, constructor function). No database calls needed — it's a static, compile-time map.

```go
// internal/strategy/catalog.go
type StrategyDef struct {
    Type             string         `json:"type"`
    Name             string         `json:"name"`
    Category         string         `json:"category"`
    Description      string         `json:"description"`
    DefaultParams    map[string]any `json:"default_parameters"`
    Constructor      func(params map[string]any) (strategy.Strategy, error)
}

var Catalog = map[string]StrategyDef{
    "buy_and_hold_strategy": {
        Type: "buy_and_hold_strategy",
        Name: "Buy and Hold",
        Category: "base",
        Description: "Buy on first signal, hold until end.",
        DefaultParams: nil,
        Constructor: func(params map[string]any) (strategy.Strategy, error) {
            return strategy.NewBuyAndHoldStrategy(), nil
        },
    },
    "rsi_strategy": {
        Type: "rsi_strategy",
        Name: "RSI Strategy",
        Category: "momentum",
        Description: "Buy when RSI crosses below oversold threshold, sell when RSI crosses above overbought threshold.",
        DefaultParams: map[string]any{"period": 14, "oversold": 30, "overbought": 70},
        Constructor: func(params map[string]any) (strategy.Strategy, error) {
            // Parse params and construct
            ...
        },
    },
    // ... all 30+ strategies
}
```

The catalog is defined in one place and used by both REST handlers and MCP tools.

## 2. Route Registration

### 2.1 New REST Routes

Added to `Handler.RegisterRoutes()`:

```go
mux.HandleFunc("/api/strategies/types",       h.handleListStrategyTypes)      // GET
mux.HandleFunc("/api/strategies",              h.handleListStrategies)          // GET
mux.HandleFunc("/api/strategies",              h.handleCreateStrategy)          // POST
mux.HandleFunc("/api/strategies/",             h.handleStrategyByID)            // POST sub-routes
mux.HandleFunc("/api/signals",                 h.handleQuerySignals)            // GET
mux.HandleFunc("/api/backtest-results",        h.handleQueryBacktestResults)    // GET
```

The `/api/strategies/` prefix handler dispatches to sub-handlers based on the path suffix:
- `POST /api/strategies/{id}/signals` → `generateSignals`
- `POST /api/strategies/{id}/backtest` → `runBacktest`

### 2.2 New MCP Tools

Registered in `registerTools()`:

```go
mcp.AddTool(server, &mcp.Tool{Name: "list_strategy_types", ...}, handleListStrategyTypes)
mcp.AddTool(server, &mcp.Tool{Name: "list_strategies", ...}, handleListStrategies)
mcp.AddTool(server, &mcp.Tool{Name: "create_strategy", ...}, handleCreateStrategy)
mcp.AddTool(server, &mcp.Tool{Name: "generate_signals", ...}, handleGenerateSignals)
mcp.AddTool(server, &mcp.Tool{Name: "run_backtest", ...}, handleRunBacktest)
mcp.AddTool(server, &mcp.Tool{Name: "query_signals", ...}, handleQuerySignals)
mcp.AddTool(server, &mcp.Tool{Name: "query_backtest_results", ...}, handleQueryBacktestResults)
```

## 3. Database Schema (DDL)

### 3.1 `strategies` Table

```sql
CREATE TABLE IF NOT EXISTS strategies (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    strategy_type   VARCHAR(100) NOT NULL,
    underlying      VARCHAR(20) NOT NULL,
    timeframe       VARCHAR(10) NOT NULL DEFAULT '1d',
    lookback_days   INTEGER NOT NULL DEFAULT 365,
    parameters      JSONB,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_strategies_name UNIQUE (name)
);

CREATE INDEX idx_strategies_type ON strategies(strategy_type);
CREATE INDEX idx_strategies_underlying ON strategies(underlying);
```

### 3.2 `signals` Table

```sql
CREATE TABLE IF NOT EXISTS signals (
    id              SERIAL PRIMARY KEY,
    strategy_id     INTEGER NOT NULL REFERENCES strategies(id) ON DELETE CASCADE,
    strategy_type   VARCHAR(100) NOT NULL,
    underlying      VARCHAR(20) NOT NULL,
    signal_date     DATE NOT NULL,
    action          VARCHAR(10) NOT NULL,
    price           DOUBLE PRECISION,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_signals_strategy_date UNIQUE (strategy_id, underlying, signal_date)
);

CREATE INDEX idx_signals_lookup ON signals(strategy_id, underlying, signal_date);
CREATE INDEX idx_signals_underlying ON signals(underlying);
CREATE INDEX idx_signals_type ON signals(strategy_type);
CREATE INDEX idx_signals_action ON signals(action);
CREATE INDEX idx_signals_date ON signals(signal_date);
```

### 3.3 `backtest_results` Table

```sql
CREATE TABLE IF NOT EXISTS backtest_results (
    id                  SERIAL PRIMARY KEY,
    strategy_id         INTEGER NOT NULL REFERENCES strategies(id) ON DELETE CASCADE,
    strategy_type       VARCHAR(100) NOT NULL,
    underlying          VARCHAR(20) NOT NULL,
    start_date          DATE NOT NULL,
    end_date            DATE NOT NULL,
    total_return        DOUBLE PRECISION,
    max_drawdown        DOUBLE PRECISION,
    sharpe_ratio        DOUBLE PRECISION,
    win_rate            DOUBLE PRECISION,
    num_transactions    INTEGER,
    final_outcome       DOUBLE PRECISION,
    final_action        VARCHAR(10),
    parameters_snapshot JSONB,
    created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_backtest_strategy_run UNIQUE (strategy_id, underlying, start_date, end_date)
);

CREATE INDEX idx_backtest_lookup ON backtest_results(strategy_id, underlying);
CREATE INDEX idx_backtest_underlying ON backtest_results(underlying);
CREATE INDEX idx_backtest_return ON backtest_results(total_return);
```

### 3.4 Migration Strategy

All `CREATE TABLE IF NOT EXISTS` statements are idempotent and safe to run on every server startup. A new `database/migrations.go` file provides:

```go
package database

var Migrations = []string{
    strategiesDDL,
    signalsDDL,
    backtestResultsDDL,
}

func RunMigrations(databaseURL string) error {
    // Execute each migration in sequence
}
```

This is called once during server startup (in `cmd/server/main.go` and `api/index.go`), ensuring tables exist before any handler runs. This avoids separate migration tooling.

## 4. Component Interfaces

### 4.1 Strategy Factory

```go
// internal/strategy/factory.go
package strategy

// Instantiate creates a strategy.Strategy from a type key and parameters map.
func Instantiate(strategyType string, params map[string]any) (strategy.Strategy, error)
```

### 4.2 Signal Repository

```go
// internal/signal/repository.go
package signal

type Repository struct {
    databaseURL string
}

func NewRepository(databaseURL string) *Repository

// ExistingSignals checks if signals already exist for a strategy + date range.
func (r *Repository) ExistingSignals(ctx context.Context, strategyID int, underlying string, startDate, endDate time.Time) (bool, error)

// GetSignals retrieves signals with filters.
func (r *Repository) GetSignals(ctx context.Context, filter SignalFilter) ([]SignalRecord, int, error)

// InsertSignals batch-inserts signal records.
func (r *Repository) InsertSignals(ctx context.Context, records []SignalRecord) error

// DeleteSignals removes signals for a strategy + date range (used when force=true).
func (r *Repository) DeleteSignals(ctx context.Context, strategyID int, underlying string, startDate, endDate time.Time) error
```

### 4.3 Backtest Repository

```go
// internal/backtest/repository.go
package backtest

type Repository struct {
    databaseURL string
}

func NewRepository(databaseURL string) *Repository

// ExistingResult checks if a backtest result already exists.
func (r *Repository) ExistingResult(ctx context.Context, strategyID int, underlying string, startDate, endDate time.Time) (*BacktestResult, error)

// InsertResult stores a backtest result.
func (r *Repository) InsertResult(ctx context.Context, result BacktestResult) error

// DeleteResult removes a backtest result (used when force=true).
func (r *Repository) DeleteResult(ctx context.Context, strategyID int, underlying string, startDate, endDate time.Time) error

// GetResults retrieves backtest results with filters.
func (r *Repository) GetResults(ctx context.Context, filter BacktestFilter) ([]BacktestResult, int, error)
```

### 4.4 Signal Generator

```go
// internal/signal/generator.go
package signal

// Generate runs a strategy against OHLCV data and produces signal records.
func Generate(
    ctx context.Context,
    st strategy.Strategy,
    snapshots []asset.Snapshot,
) ([]SignalRecord, error)
```

This function:
1. Converts snapshots to a channel via `helper.SliceToChan`
2. Duplicates the channel for strategy computation
3. Calls `strategy.ComputeWithOutcome()` to get actions and outcomes
4. Iterates through actions, matching each to a date and closing price
5. Returns `[]SignalRecord`

### 4.5 Backtest Runner

```go
// internal/backtest/runner.go
package backtest

// Run executes a backtest and returns performance/risk metrics.
func Run(
    ctx context.Context,
    st strategy.Strategy,
    snapshots []asset.Snapshot,
    params map[string]any,
) (*BacktestResult, error)
```

This function:
1. Uses `backtest.NewDataReport()` to capture results programmatically
2. Runs the backtest with the given strategy and snapshots
3. Extracts aggregate metrics from `DataStrategyResult`
4. Computes derived metrics (Sharpe ratio, win rate, etc.) from the outcome stream if not directly provided

## 5. Model Types

### 5.1 Request/Response Types

```go
// internal/model/strategy.go

type StrategyTypeEntry struct {
    Type             string         `json:"type"`
    Name             string         `json:"name"`
    Category         string         `json:"category"`
    Description      string         `json:"description"`
    DefaultParams    map[string]any `json:"default_parameters"`
}

type StrategyTypesResponse struct {
    Strategies []StrategyTypeEntry        `json:"strategies"`
    Count      int                        `json:"count"`
    Categories map[string]CategorySummary `json:"categories"`
}

type StrategyCreateRequest struct {
    Name         string         `json:"name"`
    StrategyType string         `json:"strategy_type"`
    Underlying   string         `json:"underlying"`
    Timeframe    string         `json:"timeframe"`
    LookbackDays int            `json:"lookback_days"`
    Parameters   map[string]any `json:"parameters"`
}

type StrategyResponse struct {
    ID           int            `json:"id"`
    Name         string         `json:"name"`
    StrategyType string         `json:"strategy_type"`
    Underlying   string         `json:"underlying"`
    Timeframe    string         `json:"timeframe"`
    LookbackDays int            `json:"lookback_days"`
    Parameters   map[string]any `json:"parameters"`
    CreatedAt    string         `json:"created_at"`
    UpdatedAt    string         `json:"updated_at"`
}

type StrategyListResponse struct {
    Strategies []StrategyResponse `json:"strategies"`
    Count      int                `json:"count"`
}
```

```go
// internal/model/signal.go

type SignalGenerateRequest struct {
    Force bool `json:"force"`
}

type SignalRecord struct {
    ID           int     `json:"id,omitempty"`
    StrategyID   int     `json:"strategy_id"`
    StrategyType string  `json:"strategy_type"`
    StrategyName string  `json:"strategy_name,omitempty"`
    Underlying   string  `json:"underlying"`
    SignalDate   string  `json:"signal_date"`
    Action       string  `json:"action"`
    Price        float64 `json:"price"`
}

type SignalGenerateResponse struct {
    StrategyID   int            `json:"strategy_id"`
    StrategyName string         `json:"strategy_name"`
    Underlying   string         `json:"underlying"`
    SignalCount  int            `json:"signal_count"`
    Signals      []SignalRecord `json:"signals"`
    Cached       bool           `json:"cached"`
    GeneratedAt  string         `json:"generated_at"`
}

type SignalQueryResponse struct {
    Signals []SignalRecord `json:"signals"`
    Count   int            `json:"count"`
    Total   int            `json:"total"`
}
```

```go
// internal/model/backtest.go

type BacktestRunRequest struct {
    Force bool `json:"force"`
}

type BacktestResultResponse struct {
    ID              int            `json:"id"`
    StrategyID      int            `json:"strategy_id"`
    StrategyName    string         `json:"strategy_name,omitempty"`
    StrategyType    string         `json:"strategy_type"`
    Underlying      string         `json:"underlying"`
    StartDate       string         `json:"start_date"`
    EndDate         string         `json:"end_date"`
    TotalReturn     float64        `json:"total_return"`
    MaxDrawdown     float64        `json:"max_drawdown"`
    SharpeRatio     float64        `json:"sharpe_ratio"`
    WinRate         float64        `json:"win_rate"`
    NumTransactions int            `json:"num_transactions"`
    FinalOutcome    float64        `json:"final_outcome"`
    FinalAction     string         `json:"final_action"`
    Cached          bool           `json:"cached"`
    GeneratedAt     string         `json:"generated_at"`
}

type BacktestResultsQueryResponse struct {
    Results []BacktestResultResponse `json:"results"`
    Count   int                      `json:"count"`
}
```

## 6. Signal Idempotency Logic

```go
func (h *Handler) generateSignals(w http.ResponseWriter, r *http.Request, strategyID int) {
    // 1. Load strategy from DB.
    strat := loadStrategy(strategyID)

    // 2. Determine date range.
    startDate := time.Now().AddDate(0, 0, -strat.LookbackDays)
    endDate := time.Now()

    // 3. Check existing signals.
    signalRepo := signal.NewRepository(databaseURL)
    exists, err := signalRepo.ExistingSignals(ctx, strategyID, strat.Underlying, startDate, endDate)

    if exists && !force {
        // Return cached signals.
        signals, total, _ := signalRepo.GetSignals(ctx, filter{strategyID, ...})
        writeJSON(w, 200, SignalGenerateResponse{Cached: true, Signals: signals, ...})
        return
    }

    if force {
        // Delete existing signals before regeneration.
        signalRepo.DeleteSignals(ctx, strategyID, strat.Underlying, startDate, endDate)
    }

    // 4. Fetch OHLCV snapshots.
    snapshots := fetchSnapshots(strat.Underlying, startDate)

    // 5. Instantiate strategy & generate signals.
    st := strategy.Instantiate(strat.StrategyType, strat.Parameters)
    records := signal.Generate(ctx, st, snapshots)

    // 6. Batch insert.
    signalRepo.InsertSignals(ctx, records)

    // 7. Return freshly generated signals.
    writeJSON(w, 201, SignalGenerateResponse{Cached: false, Signals: records, ...})
}
```

## 7. Strategy Constructor Patterns

Each strategy type from the cinar/indicator library has a specific constructor signature. The factory maps type keys to constructor wrappers.

### Parameterless Strategies

```go
func newBuyAndHold(params map[string]any) (strategy.Strategy, error) {
    return strategy.NewBuyAndHoldStrategy(), nil
}
```

### Single-Parameter Strategies

```go
func newRSIStrategy(params map[string]any) (strategy.Strategy, error) {
    period := getParamInt(params, "period", 14)
    oversold := getParamFloat(params, "oversold", 30)
    overbought := getParamFloat(params, "overbought", 70)

    s := strategy.NewRSIStrategy(period)
    s.Oversold = oversold
    s.Overbought = overbought
    return s, nil
}
```

### Multi-Parameter Strategies

```go
func newGoldenCrossStrategy(params map[string]any) (strategy.Strategy, error) {
    short := getParamInt(params, "short_period", 50)
    long := getParamInt(params, "long_period", 200)
    return strategy.NewGoldenCrossStrategy(short, long), nil
}
```

### Strategies Requiring Indicators

Some strategies require creating indicator instances first. The factory handles this internally:

```go
func newBollingerBandsStrategy(params map[string]any) (strategy.Strategy, error) {
    period := getParamInt(params, "period", 20)
    stddev := getParamFloat(params, "stddev", 2.0)
    return strategy.NewBollingerBandsStrategy(period, stddev), nil
}
```

## 8. Backtest Metrics Computation

The cinar/indicator `backtest.DataReport` provides `DataStrategyResult` with:
- `Outcome` (float64) — final P&L from a $1 initial investment
- `Action` — final action
- `Transactions` — all actions over time

Derived metrics are computed in the runner:

```go
// total_return = final outcome / initial investment
// (DataStrategyResult.Outcome already represents return since it starts at 0 = $1)

// max_drawdown = maximum peak-to-trough decline
// Computed from the outcome stream via a running maximum

// sharpe_ratio = mean(outcome_daily - risk_free_rate) / std(outcome_daily) * sqrt(252)
// Risk-free rate defaults to 0; uses outcome stream for daily returns

// win_rate = count(transactions ending in profit) / count(transactions)
// A trade is "won" if sell price > buy price for each Buy→Sell pair

// num_transactions = count of actions that are Buy or Sell (not Hold)
```

## 9. Risk Analysis

| Risk | Impact | Likelihood | Mitigation |
|------|--------|-----------|------------|
| Strategy constructor mismatch | Medium | Low | Factory unit tests for all 30+ strategies; add integration test with real snapshots |
| Signal time series misalignment | High | Medium | Test that signal dates exactly match snapshot dates; validate no gaps |
| Backtest metric calculation error | Medium | Low | Cross-validate with known strategy (BuyAndHold should return ~market return); add expected-value tests |
| DB query performance with large signal sets | Low | Medium | Indexes on all filter columns; pagination with LIMIT/OFFSET; `total` count via separate COUNT query |
| Strategy parameters stored as JSONB — type safety | Low | Medium | The factory validates parameter types at runtime; invalid params return clear error messages |
| Concurrent signal generation for same strategy | Low | Low | Unique constraint on (strategy_id, underlying, signal_date) prevents duplicates at DB level; use INSERT ... ON CONFLICT DO NOTHING |

## 10. Non-Functional Expectations

| Attribute | Expectation |
|-----------|-------------|
| **Performance** | Signal generation for a single strategy + 1 year of daily data should complete in <500ms. Backtesting for same should complete in <1s. |
| **Idempotency** | Duplicate signal generation requests return cached results (no redundant computation). Backtest results follow the same pattern. |
| **Data Integrity** | Unique constraints at DB level prevent duplicate signal records. Foreign keys ensure orphaned signals are impossible (CASCADE delete). |
| **Observability** | slog.Logger used throughout; each major operation logs start/completion with duration. |
| **Test Coverage** | ≥80% for new packages (strategy factory, signal generator, backtest runner, repositories). Handler tests via httptest. MCP tool tests using recorded SSE responses. |
