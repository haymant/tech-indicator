---
title: Indicator Calculation — Implementation Plan
feature_id: F-002
artifact: implementation-plan
status: complete
version: 2.0.0
owner_agent: Architect
parent_feature: F-002
last_updated: 2026-07-04
change_log:
  - date: 2026-07-04
    author: Architect
    description: Initial implementation plan.
---

# Implementation Plan — Indicator Calculation

## Phase 1: Indicator Registry & Compute

**Files:**
- `internal/indicator/registry.go` — IndicatorDef type, global metadata catalog
- `internal/indicator/compute.go` — OHLCVStreams, ComputeFunc dispatcher, SQL helpers

**Steps:**
1. Define `IndicatorDef`, `RegisterIndicator()`, global `Registry` map.
2. Register all Phase 1 indicators (16 keys) with metadata.
3. Define `OHLCVStreams` struct and `IndicatorResult` type.
4. Implement compute dispatcher: create channels from snapshot slices, call indicator ComputeWithContext, collect results.
5. Implement `ensureIndicatorsTable()` DDL and batch upsert.

**Verification:** `go test ./internal/indicator/...` passes — registry + compute unit tests.

## Phase 2: Models

**File:**
- `internal/model/sync.go` — append indicator request/response types

**Steps:**
1. Add `IndicatorCalculateRequest`, `IndicatorCalculateResponse`, `IndicatorCatalogResponse`, `IndicatorEntry`, `CatalogCategory`.

**Verification:** Compiles.

## Phase 3: Handlers

**Files:**
- `internal/handler/indicator_handler.go`
- `internal/handler/indicator_handler_test.go`
- `internal/handler/handler.go` — register routes

**Steps:**
1. Implement `handleListIndicators` (GET) — reads static registry, returns JSON.
2. Implement `handleCalculateIndicators` (POST) — auth, validate, compute, store.
3. Register `/api/indicators` and `/api/indicators/calculate` routes.
4. Unit tests for both handlers with mock compute.

**Verification:** `go test ./internal/handler/...` passes.

## Phase 4: SIT Tests

**Steps:**
1. Run server locally with DATABASE_URL.
2. Run curl tests against localhost:3000 for both endpoints.
3. Verify `indicators` table created and populated in MotherDuck.

**Verification:** SIT tests pass.

## Dependency Graph

```
Phase 1 (registry+compute) ──► Phase 3 (handlers)
                                      │
Phase 2 (models) ─────────────────────┘
                                      │
                                Phase 4 (SIT)
```
