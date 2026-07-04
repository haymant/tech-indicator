---
title: High-Performance Batch Indicator Engine
feature_id: F-003
artifact: requirements
status: draft
version: 1.0.0
owner_agent: BA
parent_feature: F-002
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: BA
    description: Initial requirements for high-performance batch indicator computation engine.
---

# Requirements — High-Performance Batch Indicator Engine (F-003)

## 1. Business Context

The current indicator calculation (F-002) uses the `github.com/cinar/indicator/v2` library's **channel-based streaming API**. Each indicator spawns goroutines, creates channels, and processes values one-at-a-time. While architecturally elegant for real-time streaming, this approach is **~1,000x slower than direct slice computation** for batch workloads.

A Python/Cython reference implementation (QLib/featureHandler) computes **158 features for 12 years of data in ~2 seconds**. Our Go implementation computes **17 indicators for 60 data points in minutes** — an **~8,000x performance gap**.

The bottleneck is not Go itself but the **channel abstraction overhead**. Direct `[]float64` slice operations in Go can match vectorized performance for most technical indicators (SMA, EMA, RSI, etc.).

## 2. Constraints

| # | Constraint | Rationale |
|---|-----------|-----------|
| C-01 | **The `indicator/` folder is an external Go dependency. Do not modify it.** | Same as F-001/F-002. Upstream library at `github.com/cinar/indicator/v2`. |
| C-02 | **The existing channel-based compute path must remain functional.** | Used by other code paths and streaming use cases. No regression. |
| C-03 | **All existing tests must pass unchanged.** | Backward compatibility guarantee. |
| C-04 | **New batch engine must cover all 80+ indicators** from the upstream library. | Parity with the library's full capabilities. |
| C-05 | **Results must match the channel-based output** (within numeric tolerance). | Deterministic correctness. |
| C-06 | **No CGo or external native code.** | Pure Go — same constraint that drove the pgx switch (Vercel compatibility). |
| C-07 | **Same API surface** — handlers, routes, response types unchanged. | No client changes needed. |

## 3. Functional Requirements

### FR-01: Direct Slice Computation Engine

A new `internal/engine` package that computes indicators directly from `[]float64` slices instead of channels.

```
// Current (channel-based, ~5ms for SMA on 3000 values):
ch := helper.SliceToChan(close)
result := sma.Compute(ch)
values := helper.ChanToSlice(result)

// Proposed (slice-based, ~5μs for SMA on 3000 values):
values := computeSMA(close, 50)
```

The engine must support all indicator types:

| Type | Example | Inputs | Outputs |
|------|---------|--------|---------|
| Single-input, single-output | SMA, EMA, RSI | 1 slice | 1 slice |
| Multi-input, single-output | ATR, OBV | 2-4 slices | 1 slice |
| Multi-input, multi-output | MACD, BB, Stochastic | 1-4 slices | 2-3 slices |
| Parameterized | SMA(20), SMA(50), MACD(12,26,9) | Varies | Varies |

### FR-02: All 80+ Indicators

Full coverage across all 5 categories:

| Category | Count | Examples |
|----------|-------|---------|
| Trend | ~40 | SMA, EMA, MACD, RMA, HMA, KAMA, DEMA, TEMA, TRIMA, WMA, T3, TRIX, TSI, KST, KDJ, STC, CCI, Aroon, BoP, CFO, DPO, Envelope, KAMA, Mass Index, McGinley Dynamic, MLS, MLR, Moving Max/Min/Sum, Pivot Point, ROC, Slope, Slow Stochastic, SMMA, Typical Price, VWMA, Weighted Close |
| Momentum | ~20 | RSI, Stochastic, Williams %R, Awesome Oscillator, Chaikin Oscillator, Connors RSI, Coppock Curve, Elder-Ray, Fisher, IBS, Ichimoku Cloud, PPO, PVO, Qstick, RVI, Stochastic RSI, TD Sequential, Ultimate Oscillator, Pring's Special K |
| Volatility | ~17 | ATR, Bollinger Bands, True Range, Acceleration Bands, AHV, Bollinger Band Width, Chandelier Exit, CHOP, Donchian Channel, HV, Keltner Channel, Moving Std, Percent B, PO, Super Trend, Ulcer Index, Z-Score |
| Volume | ~13 | OBV, A/D, CMF, EMV, Force Index, KVO, MFI, MFM, MFV, NVI, VPT, VWAP |
| Valuation | 3 | FV, NPV, PV |
| **Total** | **~93** | |

### FR-03: Benchmark Suite

A `_test.go` benchmark that measures:

- Per-indicator throughput (values/second) for both channel and slice paths
- Batch throughput (all indicators for N data points)
- Comparison table in test output

### FR-04: Correctness Verification

Each indicator implementation must produce values that match the channel-based library output within `1e-9` relative tolerance. A regression test compares both paths for identical random inputs.

### FR-05: Batch SQL Writer

Replace the per-value UPSERT loop with a single bulk INSERT statement.

```
// Current: 1 round trip per value → 1824 round trips for 1824 values
// Proposed: 1 round trip for all values
INSERT INTO indicators (name, date, indicator, value) VALUES 
($1,$2,$3,$4), ($5,$6,$7,$8), ... 
ON CONFLICT (name, date, indicator) DO UPDATE SET value = EXCLUDED.value
```

## 4. Non-Functional Requirements

| Aspect | Target | Measurement |
|--------|--------|-------------|
| Throughput | >100,000 values/sec | Benchmark with 3000 data points × 80 indicators |
| Latency | <100ms for 60 data points × 80 indicators | `time` command |
| Memory | No per-value allocations in hot path | `go test -benchmem` |
| SQL write | <10ms for 1000 values | Batch INSERT timing |
| Regression | 100% test pass rate | `go test ./...` |
| Code coverage | >85% new code | `go test -cover` |

## 5. Acceptance Criteria

| ID | Criterion | Verification |
|----|-----------|-------------|
| AC-01 | All 93 indicators computable via slice engine | Unit test for each indicator |
| AC-02 | Batch engine output matches channel output within 1e-9 | Regression test |
| AC-03 | Benchmarks show >100x speedup over channel path | `go test -bench=.` output |
| AC-04 | Existing 27 tests still pass | `go test ./internal/...` |
| AC-05 | No new dependencies introduced | `go.mod` unchanged |
| AC-06 | POST /api/indicators/calculate returns same format | SIT test |
| AC-07 | Batch SQL writer reduces round trips by >100x | Benchmark comparison |
