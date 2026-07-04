---
title: Indicator Calculation — Technical Design
feature_id: F-002
artifact: design
status: draft
version: 1.0.0
owner_agent: Architect
parent_feature: F-002
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: Architect
    description: Initial design based on F-002 requirements.
---

# Design — Indicator Calculation & Storage

## 1. Design Summary

Add two endpoints to the existing Go server:

1. **GET `/api/indicators`** — returns a static catalog of all supported technical indicators with category, description, inputs, and parameters. No DB needed.
2. **POST `/api/indicators/calculate`** — reads OHLCV snapshots from MotherDuck, computes requested indicators via the indicator library's channel-based API, and stores results in a new `indicators` table with UPSERT semantics.

The indicator registry is a static Go map populated at init time. Indicators are computed synchronously (same pattern as F-001 sync — Vercel serverless kills goroutines).

## 2. Components & Interfaces

### 2.1 New/Modified Packages

```
internal/
├── indicator/
│   ├── registry.go          # IndicatorDef type, global registry, init registration
│   ├── registry_test.go
│   └── compute.go           # Compute function helpers, OHLCVStreams type
├── handler/
│   ├── handler.go           # ← modified: add routes
│   ├── indicator_handler.go # handleListIndicators, handleCalculateIndicators
│   └── indicator_handler_test.go
└── model/
    └── sync.go              # ← modified: add IndicatorRequest/Response types
```

### 2.2 IndicatorDef — Registry Entry

```go
type IndicatorDef struct {
    Key           string            // "rsi_14"
    Category      string            // "momentum"
    DisplayName   string            // "Relative Strength Index"
    Description   string            // Human-readable
    WhenToUse     string            // Usage guidance
    Inputs        []string          // ["close"]
    SubIndicators []string          // ["line","signal","histogram"] for multi-output
    DefaultParams map[string]int    // {"period": 14}
}
```

The registry is a `map[string]IndicatorDef` populated at `init()` time. This is purely metadata — no function pointers. Computations are dispatched by indicator key via a separate `computeMap`.

### 2.3 Compute Registry

```go
// ComputeFunc computes an indicator from OHLCV channels and returns channel slices.
type ComputeFunc func(ctx context.Context, ohlcv *OHLCVStreams) []IndicatorResult

type IndicatorResult struct {
    SubIndicator string    // "" for single, "line"/"signal"/"histogram" for multi
    Values       []float64 // computed values, aligned by date
}
```

Each indicator has a registered ComputeFunc that takes OHLCV channels and returns named result slices.

### 2.4 OHLCVStreams

```go
type OHLCVStreams struct {
    Open    <-chan float64
    High    <-chan float64
    Low     <-chan float64
    Close   <-chan float64
    Volume  <-chan float64
}
```

### 2.5 Indicators Table DDL

```sql
CREATE TABLE IF NOT EXISTS indicators (
    name      TEXT NOT NULL,
    date      DATE NOT NULL,
    indicator TEXT NOT NULL,
    value     DOUBLE PRECISION NOT NULL
);
```

Index: `(name, indicator, date)` for targeted queries. UPSERT via `ON CONFLICT (name, date, indicator) DO UPDATE SET value = EXCLUDED.value`.

Note: PostgreSQL syntax `DOUBLE PRECISION` and `ON CONFLICT` are compatible with MotherDuck's PG endpoint.

## 3. Data Flow

```
Client GET /api/indicators
  → handler reads static registry map (no DB)
  → returns JSON catalog

Client POST /api/indicators/calculate
  → handler validates auth, parses body
  → validates indicator names against registry
  → reads snapshots from MotherDuck via asset.SQLRepository.GetSince
  → converts snapshots → slices → channel streams
  → for each requested indicator:
      1. Duplicate input channels as needed
      2. Call ComputeWithContext on indicator struct
      3. Collect outputs via ChanToSlice
      4. Batch upsert into indicators table
  → returns 200 OK with results summary
```

## 4. Indicator Naming Convention

```
{sma|ema|rsi|atr|obv}_{param1}[_{param2}...][_{channel}]
```

Examples: `rsi_14`, `sma_50`, `macd_12_26_9_line`, `bb_upper_20_2`, `stoch_k_14_3_3`

## 5. Indicators Implemented (Phase 1)

Phase 1 covers the most widely used indicators across all 5 categories, proving the pattern:

### Trend (6)
| Key | Struct | Inputs | Outputs |
|-----|--------|--------|---------|
| `sma_{period}` | `trend.Sma` | close | 1 |
| `ema_{period}` | `trend.Ema` | close | 1 |
| `macd_{fast}_{slow}_{signal}` | `trend.Macd` | close | 3 (line, signal, histogram) |
| `vwma_{period}` | `trend.Vwma` | close, volume | 1 |
| `apo_{fast}_{slow}` | `trend.Apo` | close | 1 |
| `roc_{period}` | `trend.Roc` | close | 1 |

### Momentum (5)
| Key | Struct | Inputs | Outputs |
|-----|--------|--------|---------|
| `rsi_{period}` | `momentum.Rsi` | close | 1 |
| `stoch_k_{period}_{sma}` / `stoch_d_{period}_{sma}` | `momentum.StochasticOscillator` | high, low, close | 2 |
| `williams_r_{period}` | `momentum.WilliamsR` | high, low, close | 1 |
| `awesome_oscillator` | `momentum.AwesomeOscillator` | high, low | 1 |
| `ibs` | `momentum.InternalBarStrength` | high, low, close | 1 |

### Volatility (3)
| Key | Struct | Inputs | Outputs |
|-----|--------|--------|---------|
| `bb_upper_{period}_{stdev}` / `bb_middle_{period}_{stdev}` / `bb_lower_{period}_{stdev}` | `volatility.BollingerBands` | close | 3 |
| `atr_{period}` | `volatility.Atr` | high, low, close | 1 |
| `tr` | `volatility.TrueRange` | high, low, close | 1 |

### Volume (2)
| Key | Struct | Inputs | Outputs |
|-----|--------|--------|---------|
| `obv` | `volume.Obv` | close, volume | 1 |
| `ad` | `volume.Ad` | high, low, close, volume | 1 |

Total: **16 indicator keys** → ~24 stored indicator channels (multi-output ones expand).

## 6. Implementation Sequence

1. Create `internal/indicator/registry.go` — `IndicatorDef` type, registry map, catalog of all indicators with metadata.
2. Create `internal/indicator/compute.go` — `OHLCVStreams`, `ComputeFunc`, `IndicatorResult`, compute dispatcher.
3. Add indicator calculation logic for Phase 1 indicators in `internal/indicator/compute.go`.
4. Create `internal/model/sync.go` additions — `IndicatorRequest`, `IndicatorResponse`, `IndicatorCatalogResponse`, `IndicatorEntry`.
5. Create `internal/handler/indicator_handler.go` — two handlers.
6. Modify `internal/handler/handler.go` — register new routes.
7. Create SQL helpers for the `indicators` table (DDL + upsert).
8. Unit tests + SIT curl tests.

## 7. Failure Modes

| Failure | Response |
|---------|----------|
| Unknown indicator name | 400 with message listing unknown names |
| No snapshots for asset | 200 with warning in message |
| Empty indicator list | Calculate ALL registered indicators |
| Database connection error | 500 |
| Invalid JSON body | 400 |
| Missing/invalid auth | 401 |

## 8. Non-Functional

| Aspect | Decision |
|--------|----------|
| Synchronous execution | Same as F-001 — Vercel serverless compatibility |
| Registry static at compile time | No DB read for catalog |
| UPSERT for idempotency | `ON CONFLICT ... DO UPDATE SET value = EXCLUDED.value` |
| Extensible | Add new indicator by adding to registry + compute map |
