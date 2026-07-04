package indicator

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cinar/indicator/v2/asset"
	"github.com/cinar/indicator/v2/helper"
)

// OHLCVStreams holds typed channels for each OHLCV component.
type OHLCVStreams struct {
	Open   <-chan float64
	High   <-chan float64
	Low    <-chan float64
	Close  <-chan float64
	Volume <-chan float64
}

// IndicatorResult is a single computed indicator time series.
type IndicatorResult struct {
	SubIndicator string // "" for single-output, "line"/"signal"/"histogram" etc.
	Values       []float64
}

// ComputeFunc computes an indicator given OHLCV streams.
type ComputeFunc func(ctx context.Context, streams *OHLCVStreams) []IndicatorResult

// ComputeMap maps indicator keys to their compute functions.
var ComputeMap = map[string]ComputeFunc{}

// ParseIndicatorKey splits an indicator key into name and int parameters.
// E.g. "rsi_14" → "rsi", [14]; "macd_12_26_9" → "macd", [12,26,9].
func ParseIndicatorKey(key string) (name string, params []int, err error) {
	parts := strings.Split(key, "_")
	if len(parts) < 2 {
		return "", nil, fmt.Errorf("invalid indicator key: %s", key)
	}
	name = parts[0]
	for _, p := range parts[1:] {
		var n int
		if _, e := fmt.Sscanf(p, "%d", &n); e != nil {
			return "", nil, fmt.Errorf("invalid parameter in key %s: %s", key, p)
		}
		params = append(params, n)
	}
	return name, params, nil
}

// intParam returns the i-th parameter or a default value.
func intParam(params []int, i, def int) int {
	if i < len(params) {
		return params[i]
	}
	return def
}

// ComputeIndicators computes the requested indicators for the given asset data.
// Each indicator gets its own set of fresh channels from the source slices.
func ComputeIndicators(ctx context.Context, snapshots []*asset.Snapshot, indicatorKeys []string) map[string][]IndicatorResult {
	if len(snapshots) == 0 {
		return nil
	}

	// Pre-extract OHLCV slices from snapshots (kept as source for each indicator).
	n := len(snapshots)
	open := make([]float64, n)
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	vol := make([]float64, n)
	for i, s := range snapshots {
		open[i] = s.Open
		high[i] = s.High
		low[i] = s.Low
		close[i] = s.Close
		vol[i] = s.Volume
	}

	results := make(map[string][]IndicatorResult)
	for _, key := range indicatorKeys {
		fn, ok := ComputeMap[key]
		if !ok {
			continue
		}
		// Create fresh channels from source slices for each compute call.
		streams := &OHLCVStreams{
			Open:   helper.SliceToChanWithContext(ctx, open),
			High:   helper.SliceToChanWithContext(ctx, high),
			Low:    helper.SliceToChanWithContext(ctx, low),
			Close:  helper.SliceToChanWithContext(ctx, close),
			Volume: helper.SliceToChanWithContext(ctx, vol),
		}
		res := fn(ctx, streams)
		if res != nil {
			results[key] = res
		}
	}
	return results
}

// ─── SQL Helpers ───────────────────────────────────────────────────────────

// EnsureIndicatorsTable creates the indicators table and index if they don't exist.
func EnsureIndicatorsTable(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("unable to open database: %w", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS indicators (
		name      TEXT NOT NULL,
		date      DATE NOT NULL,
		indicator TEXT NOT NULL,
		value     DOUBLE PRECISION NOT NULL,
		PRIMARY KEY (name, date, indicator)
	)`)
	if err != nil {
		return fmt.Errorf("unable to create indicators table: %w", err)
	}
	if err != nil {
		return fmt.Errorf("unable to create indicators table: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_indicators_lookup ON indicators (name, indicator, date)`)
	if err != nil {
		slog.Warn("Indicators index creation skipped (non-fatal)", "error", err)
	}

	return nil
}

// BatchUpsertIndicators inserts indicator values using ON CONFLICT for idempotency.
// Values are grouped by (name, date, indicator_key) with one value per row.
func BatchUpsertIndicators(dsn string, name string, dates []time.Time, results map[string][]IndicatorResult) error {
	if len(results) == 0 {
		return nil
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("unable to open database: %w", err)
	}
	defer db.Close()

	// We need to align indicator results with dates.
	// Each IndicatorResult.Values[i] corresponds to the i-th date in the range.
	// The first IdlePeriod values may be zero/NaN, but we store them anyway
	// for alignment. We need the actual date range from the snapshots.

	slog.Info("Batch upserting indicators", "asset", name, "indicatorCount", len(results))

	// For batch upsert, prepare a statement and iterate.
	stmt, err := db.Prepare(`INSERT INTO indicators (name, date, indicator, value)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name, date, indicator)
		DO UPDATE SET value = EXCLUDED.value`)
	if err != nil {
		return fmt.Errorf("unable to prepare upsert: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for indicatorKey, resultList := range results {
		for _, res := range resultList {
			var fullKey string
			if res.SubIndicator == "" {
				fullKey = indicatorKey
			} else {
				fullKey = indicatorKey + "_" + res.SubIndicator
			}
			for i, v := range res.Values {
				if i >= len(dates) {
					break
				}
				_, err := stmt.Exec(name, dates[i], fullKey, v)
				if err != nil {
					slog.Error("Upsert failed", "indicator", fullKey, "date", dates[i], "error", err)
				} else {
					inserted++
				}
			}
		}
	}

	slog.Info("Batch upsert complete", "asset", name, "inserted", inserted)
	return nil
}
