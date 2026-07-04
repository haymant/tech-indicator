---
title: High-Performance Batch Indicator Engine — Testing Plan
feature_id: F-003
artifact: testing-plan
status: draft
version: 1.0.0
owner_agent: QA
parent_feature: F-002
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: QA
    description: Initial testing plan covering regression, full coverage, benchmarks, and integration.
---

# Testing Plan — High-Performance Batch Indicator Engine

## Scope

- **Unit tests** for each of the ~93 indicator functions
- **Regression tests** comparing engine output vs channel-based output (deterministic correctness)
- **Benchmark tests** measuring throughput and improvement ratio
- **Integration tests** verifying handler compatibility
- **SIT tests** via curl against local server (smoke test)

## Test Layers

```
Layer 1: Unit (each indicator)    ← 93 tests, deterministic synthetic data
Layer 2: Regression (vs channel)  ← 93 tests, random data, tolerance check
Layer 3: Full Coverage             ← 1 test, all indicators on 500 points
Layer 4: Benchmark                 ← 4 sizes × 2 paths × 5 runs
Layer 5: Integration (handler)     ← Existing 10 handler tests unchanged
Layer 6: SIT (curl)                ← Manual smoke test against localhost
```

---

## Layer 1: Unit Tests — Per-Indicator Verification

Each indicator function is tested in isolation with known synthetic data and expected values.

### TC-001 to TC-093: Individual indicator correctness

| Field | Value |
|-------|-------|
| **Source** | `internal/engine/trend_test.go`, `momentum_test.go`, `volatility_test.go`, `volume_test.go`, `valuation_test.go` |
| **Pattern** | One `TestCompute{Name}` per indicator |
| **Input** | Monotonically increasing OHLCV (prices 100..600, volume 1M) |
| **Expected** | Non-empty result, no NaN in active period, correct output count |

Example for SMA:

```go
func TestComputeSMA(t *testing.T) {
    input := makeTestInput(100)
    results := computeSMA(context.Background(), input, []int{20})
    if len(results) != 1 {
        t.Fatalf("expected 1 result, got %d", len(results))
    }
    vals := results[0].Values
    if len(vals) != 100 {
        t.Fatalf("expected 100 values, got %d", len(vals))
    }
    // First 19 values should be zero (idle period)
    for i := 0; i < 19; i++ {
        if vals[i] != 0 {
            t.Errorf("expected 0 at position %d, got %f", i, vals[i])
        }
    }
    // Value at index 19 should be SMA of first 20 values
    expected := 0.0
    for i := 0; i < 20; i++ {
        expected += 100 + float64(i)
    }
    expected /= 20.0
    if diff := math.Abs(vals[19] - expected); diff > 1e-9 {
        t.Errorf("expected %f, got %f (diff=%e)", expected, vals[19], diff)
    }
}
```

### TC-XXX: NaN handling

| Field | Value |
|-------|-------|
| **Source** | `internal/engine/engine_test.go` |
| **Input** | OHLCV with some NaN values |
| **Expected** | Indicator handles NaN gracefully (matches library behavior) |

### TC-XXX: Empty input

| Field | Value |
|-------|-------|
| **Source** | `internal/engine/engine_test.go` |
| **Input** | Empty slices |
| **Expected** | Returns nil or empty results (no panic) |

### TC-XXX: Insufficient data (fewer points than period)

| Field | Value |
|-------|-------|
| **Source** | `internal/engine/engine_test.go` |
| **Input** | 5 data points for SMA(50) |
| **Expected** | Returns nil values (no panic, no out-of-bounds) |

---

## Layer 2: Regression Tests — Channel vs Slice Consistency

### TC-REGRESSION-001 to TC-REGRESSION-093: Value match

| Field | Value |
|-------|-------|
| **Source** | `internal/engine/regression_test.go` |
| **Data** | 500 random OHLCV points (seeded RNG for determinism) |
| **Expected** | Each indicator's slice output matches channel output within `1e-9` |

```go
func TestRegressionAllIndicators(t *testing.T) {
    rng := rand.New(rand.NewSource(42))
    data := randomOHLCV(rng, 500)
    snapshots := toSnapshots(data)
    keys := allRegisteredKeys()
    
    channelResults := indicator.ComputeIndicators(context.Background(), snapshots, keys)
    batchResults := engine.ComputeIndicators(context.Background(), toInput(data), keys)
    
    for _, key := range keys {
        t.Run(key, func(t *testing.T) {
            chanOut := channelResults[key]
            batchOut := batchResults[key]
            
            if len(chanOut) != len(batchOut) {
                t.Fatalf("output count mismatch: %d vs %d", len(chanOut), len(batchOut))
            }
            
            for i := range chanOut {
                if len(chanOut[i].Values) != len(batchOut[i].Values) {
                    t.Fatalf("value count mismatch for sub-indicator %d", i)
                }
                for j := range chanOut[i].Values {
                    expected := chanOut[i].Values[j]
                    actual := batchOut[i].Values[j]
                    if math.IsNaN(expected) && math.IsNaN(actual) {
                        continue
                    }
                    if diff := math.Abs(expected - actual); diff > 1e-9 {
                        t.Errorf("mismatch at position %d: expected %f, got %f (diff=%e)",
                            j, expected, actual, diff)
                    }
                }
            }
        })
    }
}
```

---

## Layer 3: Full Coverage Test

### TC-COVERAGE-001: All registered indicators produce results

| Field | Value |
|-------|-------|
| **Source** | `internal/engine/coverage_test.go` |
| **Input** | 500 synthetic data points |
| **Expected** | Every registered indicator key has a non-empty result in the output map |

```go
func TestFullCoverage(t *testing.T) {
    data := makeTestInput(500)
    keys := engine.RegisteredKeys()
    
    results := engine.ComputeIndicators(context.Background(), data, keys)
    
    var missing []string
    for _, key := range keys {
        if _, ok := results[key]; !ok {
            missing = append(missing, key)
        }
    }
    if len(missing) > 0 {
        t.Errorf("Missing results for %d indicators: %v", len(missing), missing)
    }
    
    t.Logf("Coverage: %d/%d indicators produce results", len(results), len(keys))
}
```

---

## Layer 4: Benchmarks

### BM-SLICE: Batch engine throughput

| Field | Value |
|-------|-------|
| **Source** | `internal/engine/benchmark_test.go` |
| **Sizes** | 60, 252, 500, 1000, 3000 data points |
| **Indicators** | All 93 |
| **Measured** | ns/op, B/op, allocs/op |

### BM-CHANNEL: Channel path throughput (subset)

| Field | Value |
|-------|-------|
| **Source** | `internal/engine/benchmark_test.go` |
| **Sizes** | 60, 252 (channel path is too slow for larger sizes) |
| **Indicators** | First 5 (SMA_20, EMA_20, RSI_14, MACD_12_26_9, BB_20_2) |
| **Measured** | ns/op, B/op, allocs/op |

### BM-COMPARISON: Speedup ratio

| Field | Value |
|-------|-------|
| **Source** | `internal/engine/benchmark_test.go` |
| **Output** | Ratio table: `channel_ns / slice_ns` |

### BM-SQL: Batch SQL writer throughput

| Field | Value |
|-------|-------|
| **Source** | `internal/engine/sql_test.go` |
| **Sizes** | 100, 1000, 10000 rows |
| **Measured** | Time to write rows via CopyFrom vs individual INSERT |

---

## Layer 5: Integration Tests

### TC-INT-001: Existing handler tests pass

```bash
go test ./internal/handler/... -v -count=1
# Expected: 10/10 PASS
```

### TC-INT-002: Existing indicator tests pass

```bash
go test ./internal/indicator/... -v -count=1
# Expected: 10/10 PASS
```

### TC-INT-003: Engine tests pass

```bash
go test ./internal/engine/... -v -count=1
# Expected: 93+ tests PASS
```

### TC-INT-004: Full workspace

```bash
go test ./... -count=1 -timeout 60s
# Expected: All pass
```

---

## Layer 6: SIT Tests (Manual)

### SIT-01: Build compiles

```bash
go build ./...
```

### SIT-02: POST /api/indicators/calculate returns 200 (batch engine active)

```bash
curl -X POST http://localhost:3000/api/indicators/calculate \
  -H "Authorization: Bearer $TECH_INDICATOR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"assets":["aapl"],"indicators":["sma_20","rsi_14","macd_12_26_9"],"days":100}'
# Expected: 200 OK
```

### SIT-03: Output matches channel path

Compare values from SIT-02 with values from a run using the old channel path (via version control rollback if needed).

### SIT-04: Query values via GET

```bash
curl "http://localhost:3000/api/indicators/values?symbols=aapl&indicators=sma_20,rsi_14"
# Expected: 200 OK with indicator values
```

---

## Test Data Strategy

| Data Type | Size | Purpose | Deterministic |
|-----------|------|---------|---------------|
| `makeTestInput(n)` | `n` points | Unit tests — linear prices 100..100+n | ✅ Yes (no random) |
| `randomOHLCV(rng, n)` | `n` points | Regression — random prices around 100±50 | ✅ Yes (seeded RNG) |
| Real Tiingo data | Variable | SIT — real-world prices from MotherDuck | ❌ No |

---

## Acceptance Criteria Verification

| ID | Criterion | Test | Verification |
|----|-----------|------|-------------|
| AC-01 | All 93 indicators computable | TC-COVERAGE-001 | All keys produce results |
| AC-02 | Results match channel output | TC-REGRESSION-* | Diff < 1e-9 per indicator |
| AC-03 | >100x speedup | BM-COMPARISON | Benchmark ratio output |
| AC-04 | 27 existing tests pass | TC-INT-001, TC-INT-002 | 27/27 PASS |
| AC-05 | No new dependencies | `go.mod` diff | Only `internal/engine/` added |
| AC-06 | Same API format | SIT-02, SIT-04 | Same JSON response shape |
| AC-07 | SQL batch >100x fewer round trips | BM-SQL | 1 COPY for N values |
