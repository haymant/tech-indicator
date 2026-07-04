---
title: High-Performance Batch Indicator Engine — Technical Design
feature_id: F-003
artifact: design
status: draft
version: 1.0.0
owner_agent: Architect
parent_feature: F-002
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: Architect
    description: Initial design for high-performance batch indicator computation.
---

# Design — High-Performance Batch Indicator Engine

## 1. Design Summary

Add a new `internal/engine` package that computes technical indicators directly from `[]float64` slices, bypassing the channel-based API. The new engine is a **drop-in replacement** for the compute path — the same HTTP handlers, models, and DB schema are reused.

**Key architectural decisions:**

1. **New package, not a rewrite** — `internal/indicator/` (channel path) is untouched.
2. **Function-per-indicator** — Each indicator gets its own exported function (e.g., `SMA(close []float64, period int) ([]float64, error)`) for clarity and testability.
3. **Auto-generated wrappers** — A registry of indicator constructors drives a generic `Compute()` dispatcher, avoiding repetitive boilerplate for 93 indicators.
4. **Parallel-friendly** — Multiple indicators for the same asset can run concurrently via goroutines (optional, caller decides).
5. **Result verification** — A regression harness compares slice-engine output vs channel-engine output for identical random inputs.

## 2. Package Structure

```
internal/engine/
├── engine.go              # Package entry: Compute() dispatcher, EngineConfig
├── engine_test.go         # Integration tests + benchmarks
├── compute.go             # ComputeIndicators() — public entry point
├── compute_test.go        # Correctness tests
├── benchmark_test.go      # Benchmarks (channel vs slice)
├── trends.go              # All trend indicators (SMA, EMA, MACD, etc.)
├── momentum.go            # All momentum indicators (RSI, Stochastic, etc.)
├── volatility.go          # All volatility indicators (ATR, BB, etc.)
├── volume.go              # All volume indicators (OBV, AD, etc.)
├── valuation.go           # All valuation indicators (FV, NPV, PV)
└── helpers.go             # Shared helpers: buildSMA, fillSMA, roundRobin etc.
```

### 2.1 Core Types

```go
// EngineConfig controls batch computation behavior.
type EngineConfig struct {
    // NumWorkers controls parallelism across indicators (0 = no parallelism).
    NumWorkers int
}

// EngineResult is a single computed series (matches existing IndicatorResult).
type EngineResult struct {
    SubIndicator string    // "" for single, "line"/"signal" for multi-output
    Values       []float64
}

// BatchInput bundles the OHLCV data for one asset.
type BatchInput struct {
    Dates  []time.Time
    Open   []float64
    High   []float64
    Low    []float64
    Close  []float64
    Volume []float64
}
```

### 2.2 Dispatcher API

```go
package engine

// ComputeIndicators computes the requested indicators using direct slice operations.
// Returns map[indicatorKey][]EngineResult, same shape as indicator.ComputeIndicators.
//
// If an indicator key is not in the engine registry, it falls back to the
// channel-based indicator.ComputeIndicators for that key (graceful degradation).
func ComputeIndicators(ctx context.Context, input *BatchInput, keys []string) map[string][]EngineResult
```

### 2.3 Registration Pattern

Rather than registering each of 93 indicators separately, indicators are registered via a table-driven approach:

```go
type indicatorFunc func(ctx context.Context, input *BatchInput, params []int) ([]EngineResult, error)

var batchRegistry = map[string]indicatorFunc{
    "sma":     computeSMA,
    "ema":     computeEMA,
    "rsi":     computeRSI,
    "macd":    computeMACD,
    "bb":      computeBollingerBands,
    // ... one entry per base indicator name
}
```

The `ParseIndicatorKey()` from `internal/indicator` is reused to split `"sma_20"` into `("sma", [20])`, then dispatched to `computeSMA(ctx, input, [20])`.

## 3. Indicator Computation Patterns

### 3.1 Simple Moving Average (SMA) — Slice Pattern

This is the canonical example showing the performance difference.

```go
func computeSMA(ctx context.Context, input *BatchInput, params []int) ([]EngineResult, error) {
    period := intParam(params, 0, 20)
    close := input.Close
    n := len(close)
    result := make([]float64, n)
    
    // Idle period: first (period-1) values are 0
    // First valid value at index period-1
    sum := 0.0
    for i := 0; i < n; i++ {
        sum += close[i]
        if i >= period-1 {
            if i >= period {
                sum -= close[i-period]
            }
            result[i] = sum / float64(period)
        }
    }
    
    return []EngineResult{{Values: result}}, nil
}
```

**Key optimization**: Single pass over data with O(1) sliding window update — no allocation per value, no goroutines, no channels.

### 3.2 Exponential Moving Average (EMA)

```go
func computeEMA(ctx context.Context, input *BatchInput, params []int) ([]EngineResult, error) {
    period := intParam(params, 0, 20)
    close := input.Close
    n := len(close)
    result := make([]float64, n)
    
    multiplier := 2.0 / float64(period+1)
    
    // SMA for first value
    sum := 0.0
    for i := 0; i < period; i++ {
        sum += close[i]
    }
    result[period-1] = sum / float64(period)
    
    // EMA for remaining
    for i := period; i < n; i++ {
        result[i] = (close[i]-result[i-1])*multiplier + result[i-1]
    }
    
    return []EngineResult{{Values: result}}, nil
}
```

### 3.3 Multi-Output Patterns (MACD, Bollinger Bands, Stochastic)

For multi-output indicators, all outputs are computed in a single pass to avoid redundant data traversal:

```go
func computeMACD(ctx context.Context, input *BatchInput, params []int) ([]EngineResult, error) {
    fastPeriod := intParam(params, 0, 12)
    slowPeriod := intParam(params, 1, 26)
    signalPeriod := intParam(params, 2, 9)
    close := input.Close
    n := len(close)
    
    // Compute fast EMA (12) and slow EMA (26) in one pass
    fastEMA := computeEMASlice(close, fastPeriod)
    slowEMA := computeEMASlice(close, slowPeriod)
    
    // MACD line = fastEMA - slowEMA
    macdLine := make([]float64, n)
    for i := 0; i < n; i++ {
        macdLine[i] = fastEMA[i] - slowEMA[i]
    }
    
    // Signal line = EMA of MACD line
    signalLine := computeEMASlice(macdLine, signalPeriod)
    
    // Histogram = MACD line - Signal line
    histogram := make([]float64, n)
    for i := 0; i < n; i++ {
        histogram[i] = macdLine[i] - signalLine[i]
    }
    
    return []EngineResult{
        {SubIndicator: "line", Values: macdLine},
        {SubIndicator: "signal", Values: signalLine},
        {SubIndicator: "histogram", Values: histogram},
    }, nil
}
```

### 3.4 RSI — Slice Pattern

```go
func computeRSI(ctx context.Context, input *BatchInput, params []int) ([]EngineResult, error) {
    period := intParam(params, 0, 14)
    close := input.Close
    n := len(close)
    result := make([]float64, n)
    
    // First, compute price changes
    gains := make([]float64, n)
    losses := make([]float64, n)
    for i := 1; i < n; i++ {
        diff := close[i] - close[i-1]
        if diff > 0 {
            gains[i] = diff
        } else {
            losses[i] = -diff
        }
    }
    
    // Average gains and losses over the period using Wilder's smoothing
    avgGain := 0.0
    avgLoss := 0.0
    for i := 1; i <= period; i++ {
        avgGain += gains[i]
        avgLoss += losses[i]
    }
    avgGain /= float64(period)
    avgLoss /= float64(period)
    
    result[period] = 100 - (100 / (1 + avgGain/avgLoss))
    
    for i := period + 1; i < n; i++ {
        avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
        avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)
        if avgLoss == 0 {
            result[i] = 100
        } else {
            result[i] = 100 - (100 / (1 + avgGain/avgLoss))
        }
    }
    
    return []EngineResult{{Values: result}}, nil
}
```

## 4. Indirect Indicators (Composed)

Some indicators in the upstream library are composed from simpler ones. The batch engine reuses slice helpers rather than calling the channel-based versions:

| Indicator | Composition |
|-----------|------------|
| Chaikin Oscillator | EMA(A/D line, short) - EMA(A/D line, long) |
| Stochastic RSI | RSI → Stochastic applied to RSI values |
| Elder-Ray Index | High - EMA(close, 13) |
| Connors RSI | RSI(close) + RSI(streak) + ROC(close) — uses multiple internal RSI calls |
| Fisher Transform | Normalized price → ln((1+x)/(1-x)) |
| KST | Sum of 4 ROC SMA values with weights |
| TSI | Double-smoothed price change ratio |
| VWAP | Cumulative(price × volume) / Cumulative(volume) |
| Bollinger Band Width | (Upper - Lower) / Middle |
| %B | (Close - Lower) / (Upper - Lower) |
| Chandelier Exit | High(max) - ATR × multiplier |

## 5. SQL Batch Writer

Replace the current per-value UPSERT with bulk INSERT. Using pgx's native `CopyFrom` for maximum performance:

```go
func BatchUpsertIndicators(ctx context.Context, conn *pgx.Conn, 
    name string, dates []time.Time, 
    results map[string][]EngineResult) error {
    
    // Collect all rows into a single CopyFromSource
    rows := make([][]any, 0, estimateRowCount(results))
    for indicatorKey, resultList := range results {
        for _, res := range resultList {
            fullKey := indicatorKey
            if res.SubIndicator != "" {
                fullKey += "_" + res.SubIndicator
            }
            for i, v := range res.Values {
                if i >= len(dates) {
                    break
                }
                rows = append(rows, []any{name, dates[i], fullKey, v})
            }
        }
    }
    
    _, err := conn.CopyFrom(
        ctx,
        pgx.Identifier{"indicators"},
        []string{"name", "date", "indicator", "value"},
        pgx.CopyFromRows(rows),
    )
    return err
}
```

`CopyFrom` uses the PostgreSQL binary COPY protocol — ~10x faster than individual INSERT statements. Falls back to prepared INSERT+ON CONFLICT if CopyFrom fails (MotherDuck compatibility).

## 6. Verification Harness

A regression test compares every batch indicator against its channel-based equivalent:

```go
func TestBatchVsChannelConsistency(t *testing.T) {
    data := generateRandomOHLCV(500)  // 500 data points
    
    for key := range indicator.Registry {
        t.Run(key, func(t *testing.T) {
            // Channel path
            channelResult := indicator.ComputeIndicators(ctx, 
                toSnapshots(data), []string{key})
            
            // Batch path
            batchResult := engine.ComputeIndicators(ctx, 
                toBatchInput(data), []string{key})
            
            // Compare
            for i := range channelResult[key] {
                for j := range channelResult[key][i].Values {
                    expected := channelResult[key][i].Values[j]
                    actual := batchResult[key][i].Values[j]
                    
                    if math.Abs(expected-actual) > 1e-9 {
                        t.Errorf("Mismatch at index %d: expected %f, got %f", 
                            j, expected, actual)
                    }
                }
            }
        })
    }
}
```

## 7. Benchmark Design

```go
func BenchmarkChannelVsSlice(b *testing.B) {
    sizes := []int{60, 252, 500, 1000, 3000}
    keys := allIndicatorKeys()
    
    for _, size := range sizes {
        data := generateRandomOHLCV(size)
        
        b.Run(fmt.Sprintf("Channel_%d_points_%d_indicators", size, len(keys)), func(b *testing.B) {
            for i := 0; i < b.N; i++ {
                indicator.ComputeIndicators(ctx, toSnapshots(data), keys)
            }
        })
        
        b.Run(fmt.Sprintf("Slice_%d_points_%d_indicators", size, len(keys)), func(b *testing.B) {
            for i := 0; i < b.N; i++ {
                engine.ComputeIndicators(ctx, toBatchInput(data), keys)
            }
        })
    }
}
```

## 8. Implementation Sequence

```
Phase 1: Infrastructure
├── Create internal/engine/ package skeleton
├── Implement helpers (SMA slice, EMA slice, NaN handling)
├── Implement compute dispatcher with ParseIndicatorKey
└── Implement BatchInput from snapshot slices

Phase 2: Core Indicators (15 most common)
├── SMA, EMA, RSI, MACD, Bollinger Bands
├── ATR, True Range, Stochastic, Williams %R
├── OBV, A/D, VWAP
├── VWMA, APO, ROC
└── Test: all 15 verified against channel path

Phase 3: Full Coverage (~78 remaining)
├── Remaining Trend indicators (25)
├── Remaining Momentum indicators (12)
├── Remaining Volatility indicators (12)
├── Remaining Volume indicators (9)
├── Valuation indicators (3)
└── Test: all 93 verified against channel path

Phase 4: Batch SQL Writer
├── Implement CopyFrom-based batch writer
├── Fallback to prepared statement for MotherDuck compat
├── Benchmark SQL write performance
└── Integration test

Phase 5: Handler Integration
├── Configure handler to use engine for batch compute
├── Keep channel path as fallback
└── Integration test

Phase 6: Documentation
├── Benchmark results in README
├── Performance comparison table
└── KB artifacts finalized
```

## 9. Performance Targets

| Metric | Channel Path (current) | Slice Path (target) | Improvement |
|--------|----------------------|-------------------|-------------|
| SMA (1000 values) | ~1-2ms | ~2-5μs | ~400x |
| RSI (1000 values) | ~1-2ms | ~3-7μs | ~300x |
| MACD (1000 values) | ~3-5ms | ~10-20μs | ~300x |
| All 93 indicators (252 values) | ~50-100ms | ~1-2ms | ~50x |
| SQL write (1000 values) | ~50-100ms | ~5-10ms | ~10x |
| **Total: 93 ind × 252 days** | **~100-200ms** | **~6-12ms** | **~15x** |

Note: The current bottleneck for real assets is **Tiingo API fetch** (seconds), not indicator computation (milliseconds). The slice engine primarily improves **batch recomputation scenarios** (backtest parameter sweeps, multi-asset recalculation).

## 10. Failure Modes

| Failure | Handling |
|---------|----------|
| Indicator not in batch registry | Fall back to channel-based `indicator.ComputeIndicators` for that key |
| NaN/Inf values in input | Propagate deterministically (same as library behavior) |
| Too few data points for period | Return zero/Nil at idle positions (same as channel path) |
| CopyFrom not supported (MotherDuck) | Fall back to prepared INSERT+ON CONFLICT |
| Context cancellation | Check ctx.Err() between indicators, return partial results |

## 11. Non-Functional

| Aspect | Decision |
|--------|----------|
| Package location | `internal/engine/` — new package, no existing code modified |
| Concurrency | Caller decides parallelism (goroutine pool) |
| Memory | Pre-allocate result slices, reuse scratch buffers |
| Error handling | Return errors, caller decides severity |
| Logging | `slog.Debug` for per-indicator timing |
| Zero-value handling | NaN values at idle positions — same as library |
