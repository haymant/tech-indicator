---
title: Indicator Calculation — Testing Plan
feature_id: F-002
artifact: testing-plan
status: complete
version: 2.0.0
owner_agent: QA
parent_feature: F-002
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: QA
    description: Initial testing plan with unit and SIT tests.
---

# Testing Plan — Indicator Calculation

## Scope

- Unit tests for indicator registry and compute logic
- Unit tests for HTTP handlers
- SIT tests via curl against local server with real MotherDuck

## Unit Tests

### TC-01: Registry contains all Phase 1 indicators

| Field | Value |
|-------|-------|
| **Source** | `internal/indicator/registry_test.go` |
| **Expected** | Registry has ≥16 entries across 5 categories |

### TC-02: Registry keys are parseable

| Field | Value |
|-------|-------|
| **Source** | `registry_test.go` |
| **Expected** | Each key matches format `{name}_{params}` |

### TC-03: GET /api/indicators returns 200

| Field | Value |
|-------|-------|
| **Source** | `internal/handler/indicator_handler_test.go` |
| **Expected** | Status 200; body contains `indicators` array and `categories` |

### TC-04: GET /api/indicators counts match categories

| Field | Value |
|-------|-------|
| **Source** | `indicator_handler_test.go` |
| **Expected** | Category counts sum to total indicator count |

### TC-05: POST /api/indicators/calculate valid body → 200

| Field | Value |
|-------|-------|
| **Source** | `indicator_handler_test.go` |
| **Expected** | Status 200 with results summary |

### TC-06: POST with unknown indicator → 400

| Field | Value |
|-------|-------|
| **Source** | `indicator_handler_test.go` |
| **Expected** | Status 400 with error message |

### TC-07: POST without auth → 401

| Field | Value |
|-------|-------|
| **Source** | `indicator_handler_test.go` |
| **Expected** | Status 401 |

## SIT Tests (Curl)

### SIT-01: GET /api/indicators

```bash
curl http://localhost:3000/api/indicators
# Expected: 200, JSON with indicators array
```

### SIT-02: POST /api/indicators/calculate — RSI for TSLA

```bash
curl -X POST http://localhost:3000/api/indicators/calculate \
  -H "Authorization: Bearer $TECH_INDICATOR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"assets":["tsla"],"indicators":["rsi_14"]}'
# Expected: 200, "Indicator calculation completed"
```

### SIT-03: Verify indicators table in MotherDuck

```bash
# Query MotherDuck for RSI values
```

### SIT-04: Unknown indicator → 400

```bash
curl -X POST http://localhost:3000/api/indicators/calculate \
  -H "Authorization: Bearer $TECH_INDICATOR_API_KEY" \
  -d '{"indicators":["fake_indicator"]}'
# Expected: 400
```
