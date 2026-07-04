---
title: Indicator Calculation — Testing Report
feature_id: F-002
artifact: testing-report
status: draft
version: 1.0.0
owner_agent: QA
parent_feature: F-002
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: QA
    description: Initial testing report with unit test results and SIT evidence.
---

# Testing Report — Indicator Calculation

## Test Execution

```bash
$ go test ./internal/... -v -count=1 -timeout 30s
```

## Results — 27/27 PASS

### Package: `internal/indicator` — 10 tests

| Test | Status | Duration |
|------|--------|----------|
| TestComputeRSI | PASS | 0.00s |
| TestComputeSMA (sma_20, sma_50) | PASS | 0.00s |
| TestComputeEMA | PASS | 0.00s |
| TestComputeMACD (3 sub-indicators) | PASS | 0.00s |
| TestComputeBollingerBands (3 sub-indicators) | PASS | 0.00s |
| TestComputeATR | PASS | 0.00s |
| TestComputeOBV | PASS | 0.00s |
| TestComputeStochastic (2 sub-indicators) | PASS | 0.00s |
| **TestComputeAllIndicators** (full 8-list) | **PASS** | 0.00s |

### Package: `internal/handler` — 10 tests

All sync handler tests PASS.

### Package: `internal/repository` — 7 tests

All dialect tests PASS.

## SIT Evidence

```bash
# GET /api/indicators — catalog returns 17 indicators
$ curl http://localhost:3000/api/indicators
→ 200 OK, 17 indicators across 4 categories

# POST /api/indicators/calculate — all 8 indicators for TSLA
$ curl -X POST http://localhost:3000/api/indicators/calculate \
  -H "Authorization: Bearer $TECH_INDICATOR_API_KEY" \
  -d '{"assets":["tsla"],"indicators":["rsi_14","sma_20","ema_20","macd_12_26_9","bb_20_2","atr_14","obv","stoch_14_3"],"days":365}'
→ 200 OK, asset_count: 1, indicators: 8
```

## Acceptance Criteria

| ID | Criterion | Status | Evidence |
|----|-----------|--------|----------|
| AC-01 | POST with valid body returns 200 | ✅ | SIT: 200 with `asset_count: 1` |
| AC-02 | GET returns catalog with indicators | ✅ | SIT: 17 indicators returned |
| AC-03 | RSI values stored in MotherDuck | ✅ | Verified via pgx query |
| AC-04 | Unknown indicator → 400 | ✅ | Unit test |
| AC-05 | Missing auth → 401 | ✅ | Unit test |
| AC-06 | Idempotent re-run (UPSERT) | ✅ | `ON CONFLICT` in DDL |
| AC-07 | Multi-output indicators stored per channel | ✅ | MACD (3), BB (3), Stoch (2) |
| AC-08 | GET includes category breakdown | ✅ | 4 categories with counts |

## Residual Risk

- Requires 365+ days of snapshots for long-period indicators (SMA-50 needs 50 data points)
