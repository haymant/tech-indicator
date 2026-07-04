---
title: Indicator Calculation & Storage Endpoints
feature_id: F-002
artifact: requirements
status: draft
version: 1.0.0
owner_agent: BA
parent_feature: null
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: BA
    description: Initial requirements draft for indicator calculation and storage.
---

# Requirements — Indicator Calculation & Storage

## 1. Business Context

The system currently syncs raw OHLCV snapshots from Tiingo into MotherDuck (`snapshots` table). These raw snapshots are the **input** for technical analysis indicators (RSI, MACD, SMA, Bollinger Bands, etc.), which must be calculated and stored in a separate table for querying, visualization, and downstream use (backtesting, strategy execution, API consumption).

The indicator library (`github.com/cinar/indicator/v2`) provides 80+ configurable indicators across 5 categories — all indicators are computed **on-the-fly from channel streams** and have no built-in persistence mechanism. The application layer must own indicator storage.

## 2. Constraints

| # | Constraint | Rationale |
|---|-----------|-----------|
| C-01 | **The `indicator/` folder is an external Go dependency. Do not modify it.** | Same as F-001. Upstream library at `github.com/cinar/indicator/v2`. |
| C-02 | Indicator values must be stored in a **new `indicators` table**, not in `snapshots`. | The `snapshots` schema is owned by the library (C-01). Indicators are 1-to-many per snapshot date — flattening them into columns would require frequent ALTER TABLE and violate library schema constraints. |
| C-03 | The indicator calculation must happen **server-side**, triggered via a POST endpoint. | Indicators depend on the full OHLCV history for a given ticker, which lives in MotherDuck. Calculation should run asynchronously. |
| C-04 | The endpoint must be authenticated with the same `TECH_INDICATOR_API_KEY` Bearer token as POST `/api/sync`. | Consistent auth across all state-mutating endpoints. |

## 3. Functional Requirements

### FR-01: POST /api/indicators/calculate — Trigger Indicator Calculation

**Description**

Triggers calculation of technical indicators for specified assets, using the snapshots data already in MotherDuck, and stores results in the `indicators` table.

**Request**

```
POST /api/indicators/calculate
Content-Type: application/json
Authorization: Bearer $TECH_INDICATOR_API_KEY
```

Optional JSON body:

```json
{
  "assets": ["aapl", "msft"],
  "indicators": ["rsi_14", "sma_50", "macd_12_26_9"],
  "days": 365
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `assets` | `[]string` | No | All assets in MotherDuck snapshot table | Ticker symbols to calculate indicators for. |
| `indicators` | `[]string` | No | **All supported indicators** | Specific indicators to calculate. See GET `/api/indicators` for available names. |
| `days` | `int` | No | All available data | Maximum number of lookback days of snapshots to use for calculation. |

**Response — 202 Accepted**

```json
{
  "status": "accepted",
  "message": "Indicator calculation started",
  "assets": ["aapl", "msft"],
  "indicators": 3,
  "timestamp": "2026-07-04T12:00:00Z"
}
```

**Response — 400 Bad Request** (invalid indicator name, unknown asset)

```json
{
  "status": "error",
  "message": "Unknown indicator: 'invalid_indicator_name'",
  "timestamp": "2026-07-04T12:00:00Z"
}
```

### FR-02: GET /api/indicators — List Available Indicators

**Description**

Returns the full catalog of supported indicators with their category, description, inputs, outputs, default parameters, and usage guidance.

**Request**

```
GET /api/indicators
```

No auth required (read-only, informational).

**Response — 200 OK**

```json
{
  "indicators": [
    {
      "name": "rsi_14",
      "category": "momentum",
      "display_name": "Relative Strength Index",
      "description": "Momentum oscillator measuring the speed and magnitude of recent price changes to identify overbought (>70) and oversold (<30) conditions.",
      "when_to_use": "Best for identifying trend reversals and overbought/oversold levels in ranging markets.",
      "inputs": ["close"],
      "outputs": 1,
      "default_parameters": { "period": 14 }
    },
    {
      "name": "macd_12_26_9",
      "category": "trend",
      "display_name": "Moving Average Convergence Divergence",
      "description": "Trend-following momentum indicator showing the relationship between two exponential moving averages.",
      "when_to_use": "Best for identifying trend direction, momentum, and potential crossover signals.",
      "inputs": ["close"],
      "outputs": 3,
      "sub_indicators": ["macd_12_26_9_line", "macd_12_26_9_signal", "macd_12_26_9_histogram"],
      "default_parameters": { "fast_period": 12, "slow_period": 26, "signal_period": 9 }
    }
  ],
  "count": 80,
  "categories": {
    "trend": { "count": 30, "description": "Identify the direction and strength of price trends" },
    "momentum": { "count": 19, "description": "Measure the speed and magnitude of price movements" },
    "volatility": { "count": 17, "description": "Measure the rate and magnitude of price fluctuations" },
    "volume": { "count": 12, "description": "Analyze trading volume to confirm price movements" },
    "valuation": { "count": 3, "description": "Calculate present and future value of assets" }
  },
  "timestamp": "2026-07-04T12:00:00Z"
}
```

### FR-03: Indicators Table Schema

A new `indicators` table in MotherDuck (same database `fin-lake` as `snapshots`):

```sql
CREATE TABLE IF NOT EXISTS indicators (
    name      TEXT NOT NULL,    -- asset ticker, e.g. 'aapl'
    date      DATE NOT NULL,   -- trading date
    indicator TEXT NOT NULL,    -- indicator key, e.g. 'rsi_14', 'macd_12_26_9_line'
    value     DOUBLE NOT NULL  -- computed indicator value
);
```

**Primary Key**: `(name, date, indicator)`

This composite PK is the natural key — for a given asset on a given date, each indicator (or sub-channel) appears at most once. The `indicator` column encodes both the indicator type, its parameters, and its output channel:

| Column | Example values | Cardinality |
|--------|---------------|-------------|
| `name` | `aapl`, `msft`, `googl` | ~thousands |
| `date` | `2026-04-06` ... `2026-07-04` | ~252/year |
| `indicator` | `rsi_14`, `sma_50`, `macd_12_26_9_line`, `bb_upper_20_2`, `atr_14` | ~200+ |

**Estimated row count per full calculation**:
- One asset × one trading day × ~200 indicator channels = ~200 rows/day/asset
- 3 assets × 90 days × 200 = ~54,000 rows per calculation run
- This is well within MotherDuck's capabilities even for hundreds of assets.

### FR-04: Indicator Naming Convention

Every stored indicator value uses a deterministic, parseable naming scheme:

```
{indicator_key}_{param1}_{param2}_{...}[_{channel}]
```

| Indicator | Stored Name(s) | Notes |
|-----------|---------------|-------|
| RSI(14) | `rsi_14` | Single output |
| SMA(50) | `sma_50` | Single output |
| MACD(12,26,9) | `macd_12_26_9_line`, `macd_12_26_9_signal`, `macd_12_26_9_histogram` | 3 sub-channels |
| Bollinger Bands(20,2) | `bb_upper_20_2`, `bb_middle_20_2`, `bb_lower_20_2` | 3 sub-channels |
| ATR(14) | `atr_14` | Single output |
| Stochastic(14,3,3) | `stoch_k_14_3_3`, `stoch_d_14_3_3` | 2 sub-channels |
| OBV | `obv` | No parameters, single output |
| Ichimoku Cloud | `ichimoku_conversion_9_26_52`, `ichimoku_base_9_26_52`, `ichimoku_leading_a_9_26_52`, `ichimoku_leading_b_9_26_52`, `ichimoku_lagging_9_26_52` | 5 sub-channels |

### FR-05: Indicator Registry (internal data structure)

The application must maintain a registry of all supported indicators. This is the source of truth for both the GET listing and the POST calculation logic. Each entry includes:

```go
type IndicatorDef struct {
    Key              string            // "rsi_14"
    Category         string            // "momentum"
    DisplayName      string            // "Relative Strength Index"
    Description      string            // Human-readable description
    WhenToUse        string            // Usage guidance
    Inputs           []string          // ["close"] or ["high","low","close"] etc.
    SubIndicators    []string          // For multi-output indicators: ["line","signal","histogram"]
    DefaultParams    map[string]int    // {"period": 14}
    ComputeFn        func(ohlcv *OHLCVStreams) []IndicatorResult  // Function pointer
}
```

This registry is populated at compile time and drives both the GET listing endpoint and the POST calculation worker.

### FR-06: Calculation Worker Logic

1. Read snapshots from MotherDuck for the requested assets and date range via `asset.SQLRepository.GetSince`.
2. Convert snapshots into Go slices, then into channel streams via `helper.SliceToChan`.
3. For each requested indicator:
   a. Instantiate the indicator struct with its parameters.
   b. Call `ComputeWithContext` with the appropriate input channels.
   c. Collect output values via `helper.ChanToSlice`.
   d. Batch-insert into the `indicators` table (doing an UPSERT to handle idempotent re-runs).
4. Runs in a background goroutine; returns 202 immediately.

### FR-07: UPSERT Semantics

The `indicators` table must support idempotent re-calculation. If a sync brings in new data for an existing date, re-running indicator calculation should update existing rows, not create duplicates.

```sql
INSERT INTO indicators (name, date, indicator, value)
VALUES ($1, $2, $3, $4)
ON CONFLICT (name, date, indicator)
DO UPDATE SET value = EXCLUDED.value
```

DuckDB supports `ON CONFLICT` for upsert semantics since version 0.8.

## 4. Non-Functional Requirements

| # | Requirement | Target |
|---|-------------|--------|
| NFR-01 | Response within 2 seconds (202 Accepted). Actual computation runs async. | ≤ 2s |
| NFR-02 | 80+ indicators must be computable in a single request. | Full catalog |
| NFR-03 | Indicator catalog (GET) must load in <100ms — static data, no DB needed. | < 100ms |
| NFR-04 | Re-running indicators is idempotent — no duplicate rows. | UPSERT |
| NFR-05 | Must support incremental calculation: only compute indicators for dates that have new snapshots since last calculation. | Optional optimization |

## 5. Acceptance Criteria

| ID | Criterion | How to verify |
|----|-----------|---------------|
| AC-01 | POST `/api/indicators/calculate` with valid body returns 202. | `curl -X POST -H "Authorization: Bearer $KEY" -d '{"assets":["aapl"],"indicators":["rsi_14"],"days":90}' localhost:3000/api/indicators/calculate` → 202 |
| AC-02 | GET `/api/indicators` returns 200 with indicator catalog. | `curl localhost:3000/api/indicators` → 200 with list of 80+ indicators |
| AC-03 | After calculation, RSI values exist in MotherDuck `indicators` table for AAPL. | `SELECT COUNT(*) FROM indicators WHERE name='aapl' AND indicator='rsi_14'` > 0 |
| AC-04 | Unknown indicator name in request returns 400. | `-d '{"indicators":["fake_indicator"]}'` → 400 |
| AC-05 | Missing auth token returns 401. | No `Authorization` header → 401 |
| AC-06 | Re-running calculation does not create duplicate rows. | Run twice → row count remains same (UPSERT) |
| AC-07 | Multi-output indicators (MACD, Bollinger Bands) store each sub-channel separately. | Select by indicator prefix → 3 rows per date for MACD |
| AC-08 | GET `/api/indicators` includes category breakdown with descriptions. | Response contains `categories` object. |

## 6. Open Questions

| # | Question | Status |
|---|----------|--------|
| OQ-01 | Should indicator parameters be configurable per-request (e.g., `{"rsi_14":{"period":14}}`) or fixed to the named key? | **Needs BA/Architect decision.** Fixed-key approach means `rsi_14` always computes RSI(14). If users need RSI(21), they'd request `rsi_21`. |
| OQ-02 | Should the valuation indicators (FV, NPV, PV) be included? They don't operate on OHLCV streams. | **Needs BA decision.** They're pure functions (not stream-based), so they don't fit the channel pipeline. Could be exposed as a separate endpoint. |
| OQ-03 | Should indicator calculation automatically trigger after a sync completes? | **Deferred.** Could be added as an option in a future phase. |

## 7. Key Architectural Decisions

### 7.1 Narrow `indicators` table (one value per row)

Chosen over wide-table (one row per date, one column per indicator) because:
- Adding new indicators doesn't require schema migration
- Query is straightforward: `SELECT value FROM indicators WHERE name='aapl' AND indicator='rsi_14' ORDER BY date`
- DuckDB's columnar compression handles repeated `(name, date)` values efficiently
- `ON CONFLICT` upsert is clean and idempotent

### 7.2 DuckDB Dialect Extensions

The existing `duckdb_dialect.go` will be extended with a new method or a separate `IndicatorsDialect` for managing the `indicators` table. However, since this table is **application-owned** (not library-owned), it won't use `asset.SQLRepositoryDialect`. Instead, direct `database/sql` queries will be used via the existing `MotherDuckDriverName`.

### 7.3 Static Indicator Registry

The indicator registry will be a Go `map[string]IndicatorDef` populated at init time. This avoids runtime reflection and keeps the GET endpoint fast (no DB round-trip). It also serves as the validator for incoming indicator names in the POST request.

### 7.4 One-to-Many Mapping

For each `(name, date)` pair, there are ~200 indicator values. The `indicator` column differentiates them. Query patterns:

```sql
-- All indicators for AAPL on a specific date
SELECT indicator, value FROM indicators WHERE name = 'aapl' AND date = '2026-07-01';

-- RSI time series for AAPL
SELECT date, value FROM indicators WHERE name = 'aapl' AND indicator = 'rsi_14' ORDER BY date;

-- All momentum indicators for multiple assets
SELECT * FROM indicators WHERE name IN ('aapl', 'msft') AND indicator LIKE 'rsi_%' ORDER BY name, date;
```

## 8. Out of Scope (for this feature)

- Real-time / streaming indicator calculation (WebSocket)
- Visualization of indicator values (frontend)
- Strategy backtesting using stored indicators (future F-003)
- Automatic trigger of indicator calculation after sync completion
- Valuation indicators (FV, NPV, PV) — they're pure functions, not OHLCV stream-based
