---
title: Strategy Management, Signal Generation & Backtesting — Testing Plan
feature_id: F-005
artifact: testing-plan
status: draft
version: 1.0.0
owner_agent: QA
parent_feature: F-003
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: Architect
    description: Initial testing plan for F-005 with unit, integration, and e2e tests.
---

# Testing Plan — Strategy Management, Signal Generation & Backtesting (F-005)

## 1. Test Strategy

| Level | Focus | Tools | Environment |
|-------|-------|-------|-------------|
| **Unit** | Individual packages (strategy factory, signal generator, backtest runner, repositories) | Go `testing` package, table-driven tests | Local |
| **Integration** | REST handlers + MCP tools with real MotherDuck | `httptest`, `pgx`, MCP SSE parsing | Local + MotherDuck |
| **E2E** | Full workflow via deployed endpoints | `curl`, manual verification | Vercel prod |

## 2. Test Matrix

### 2.1 Strategy Catalog (Phase 2)

| Test ID | Description | Type | Acceptance Criteria |
|---------|------------|------|-------------------|
| TC-STRAT-01 | All strategy types are registered in the catalog | Unit | `len(strategy.Catalog) >= 30` |
| TC-STRAT-02 | Each strategy type can be instantiated with default params | Unit | `strategy.Instantiate(type, nil)` returns no error for all types |
| TC-STRAT-03 | Invalid strategy type returns error | Unit | `strategy.Instantiate("unknown", nil)` returns error |
| TC-STRAT-04 | Invalid parameter returns error | Unit | Wrong parameter types produce clear error messages |
| TC-STRAT-05 | `list_strategy_types` returns categories with correct counts | Unit | Each strategy maps to the correct category; counts match |

### 2.2 Signal Generator (Phase 3)

| Test ID | Description | Type | Acceptance Criteria |
|---------|------------|------|-------------------|
| TC-SIG-01 | BuyAndHoldStrategy generates buy on first date, hold for rest | Unit | First signal = `buy`, remaining = `hold` |
| TC-SIG-02 | Signal count matches snapshot count | Unit | `len(signals) == len(snapshots)` |
| TC-SIG-03 | Signal prices match snapshot closing prices | Unit | `signal.Price == snapshot.Close` for each date |
| TC-SIG-04 | RSI Strategy generates buy/sell at correct thresholds | Unit | With known RSI values, signals match expected pattern |
| TC-SIG-05 | Empty snapshots return empty signals | Unit | No error, empty slice |
| TC-SIG-06 | Signal dates are in ascending order | Unit | Each signal date >= previous |

### 2.3 Signal Repository (Phase 3)

| Test ID | Description | Type | Acceptance Criteria |
|---------|------------|------|-------------------|
| TC-SIGREPO-01 | Insert signals batch-inserts correctly | Integration | Rows exist in `signals` table |
| TC-SIGREPO-02 | Duplicate (strategy_id, underlying, date) is rejected | Integration | `ON CONFLICT DO NOTHING` prevents duplicates |
| TC-SIGREPO-03 | ExistingSignals returns true when signals exist | Integration | After insert, `ExistingSignals()` returns true |
| TC-SIGREPO-04 | ExistingSignals returns false when none exist | Integration | Before insert, `ExistingSignals()` returns false |
| TC-SIGREPO-05 | GetSignals returns correct count with filters | Integration | Each filter (strategy_id, type, underlying, date range, action) narrows results correctly |
| TC-SIGREPO-06 | DeleteSignals removes only matching rows | Integration | After delete, `ExistingSignals()` returns false |

### 2.4 Backtest Runner (Phase 4)

| Test ID | Description | Type | Acceptance Criteria |
|---------|------------|------|-------------------|
| TC-BT-01 | BuyAndHoldStrategy returns positive total return for an uptrend | Unit | Simulated uptrend → `TotalReturn > 0` |
| TC-BT-02 | BuyAndHoldStrategy returns negative return for a downtrend | Unit | Simulated downtrend → `TotalReturn < 0` |
| TC-BT-03 | MaxDrawdown is between 0 and -1 | Unit | `MaxDrawdown <= 0 && MaxDrawdown >= -1` |
| TC-BT-04 | FinalAction matches last action | Unit | `FinalAction` string matches last strategy action |
| TC-BT-05 | NumTransactions counts Buy + Sell actions | Unit | `NumTransactions > 0` for BuyAndHold (1 buy) |

### 2.5 Backtest Repository (Phase 4)

| Test ID | Description | Type | Acceptance Criteria |
|---------|------------|------|-------------------|
| TC-BTREPO-01 | InsertResult stores correctly | Integration | Row exists in `backtest_results` |
| TC-BTREPO-02 | Duplicate strategy+underlying+dates is rejected | Integration | `ON CONFLICT DO NOTHING` prevents duplicates |
| TC-BTREPO-03 | GetResults filters by strategy_id | Integration | Only matching results returned |
| TC-BTREPO-04 | GetResults filters by min_return | Integration | `WHERE total_return >= $1` |

### 2.6 REST Handlers (Phase 5)

| Test ID | Description | Type | Acceptance Criteria |
|---------|------------|------|-------------------|
| TC-REST-01 | GET /api/strategies/types returns 200 with JSON body | Integration | Non-empty strategies array, valid JSON |
| TC-REST-02 | GET /api/strategies returns empty list initially | Integration | `count: 0` |
| TC-REST-03 | POST /api/strategies creates and returns 201 | Integration | Response includes `id` |
| TC-REST-04 | POST /api/strategies with duplicate name returns 409 | Integration | Error response with conflict message |
| TC-REST-05 | POST /api/strategies without auth returns 401 | Integration | Error response |
| TC-REST-06 | POST /api/strategies/{id}/signals generates signals | Integration | Response has `cached: false`, non-empty signals |
| TC-REST-07 | Same call again returns cached signals | Integration | Response has `cached: true` |
| TC-REST-08 | POST with force:true regenerates signals | Integration | Response has `cached: false` again |
| TC-REST-09 | POST /api/strategies/{id}/backtest returns metrics | Integration | All metric fields present and non-nil |
| TC-REST-10 | Same backtest call returns cached result | Integration | `cached: true` |
| TC-REST-11 | GET /api/signals filters by action | Integration | Only matching actions returned |
| TC-REST-12 | GET /api/backtest-results filters by min_return | Integration | Only results with >= min_return |

### 2.7 MCP Tools (Phase 6)

| Test ID | Description | Type | Acceptance Criteria |
|---------|------------|------|-------------------|
| TC-MCP-01 | `list_strategy_types` tool returns SSE response | Integration | Valid SSE format, non-empty result |
| TC-MCP-02 | `create_strategy` tool returns strategy ID | Integration | Result includes `id` |
| TC-MCP-03 | `generate_signals` tool returns cached: false on first call | Integration | Result has signals array |
| TC-MCP-04 | `generate_signals` tool returns cached: true on second call | Integration | `cached: true` |
| TC-MCP-05 | `run_backtest` tool returns performance metrics | Integration | All metric fields present |
| TC-MCP-06 | `query_signals` tool filters correctly | Integration | Filtered results match | 
| TC-MCP-07 | `query_backtest_results` tool filters correctly | Integration | Filtered results match |

### 2.8 Regression

| Test ID | Description | Type | Acceptance Criteria |
|---------|------------|------|-------------------|
| TC-REG-01 | All F-001 tests pass | Regression | No regression in sync functionality |
| TC-REG-02 | All F-002 tests pass | Regression | No regression in indicator calculation |
| TC-REG-03 | All F-003 tests pass | Regression | No regression in batch engine |
| TC-REG-04 | All F-004 MCP tests pass | Regression | 12 existing MCP tests pass |
| TC-REG-05 | `go build ./...` compiles | Regression | No compile errors |

## 3. Test Data

### 3.1 Synthetic Snapshots for Unit Tests

```go
func makeTestSnapshots() []*asset.Snapshot {
    // 10 trading days of AAPL-like data
    prices := []float64{150, 152, 148, 155, 157, 153, 158, 160, 162, 159}
    var snapshots []*asset.Snapshot
    for i, p := range prices {
        snapshots = append(snapshots, &asset.Snapshot{
            Date:  time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
            Close: p,
            High:  p * 1.02,
            Low:   p * 0.98,
            Open:  p * 0.99,
            Volume: 1000000,
        })
    }
    return snapshots
}
```

### 3.2 Uptrend / Downtrend Fixtures

```go
func uptrendSnapshots(n int) []*asset.Snapshot {
    start := 100.0
    ...
}
func downtrendSnapshots(n int) []*asset.Snapshot {
    start := 100.0
    ...
}
```

## 4. Expected Test Counts

| Package | Test Files | Expected Tests |
|---------|-----------|---------------|
| `internal/strategy` | `catalog_test.go` | 5 |
| `internal/signal` | `generator_test.go`, `repository_test.go` | 11 |
| `internal/backtest` | `runner_test.go`, `repository_test.go` | 9 |
| `internal/handler` | 3 handler test files | 12 |
| `internal/mcp` | 3 MCP tool test files | 7 |
| **Total new tests** | | **44** |
| Existing tests (regression) | F-001 through F-004 | ~90 |

## 5. Testing Commands

```bash
# All tests
go test -v -count=1 ./...

# Specific packages
go test -v -count=1 ./internal/strategy/...
go test -v -count=1 ./internal/signal/...
go test -v -count=1 ./internal/backtest/...
go test -v -count=1 ./internal/handler/...
go test -v -count=1 ./internal/mcp/...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## 6. Risks

| Risk | Mitigation |
|------|-----------|
| MotherDuck connection required for repository tests | Repository tests use pgx directly; skip if `DATABASE_URL` is not set using `testing.Short()` |
| Strategy constructor differences across library versions | Pin `github.com/cinar/indicator/v2 v2.1.33` in `go.mod`; update catalog if library upgrades |
| Signal date alignment off-by-one (timezone) | All dates stored as `DATE` type (no time component); snapshot dates truncated to UTC midnight |
| Large signal sets cause slow repository tests | Limit test data to 100-252 rows per test |
