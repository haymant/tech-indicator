package engine

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/cinar/indicator/v2/asset"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Input bundles OHLCV data for one asset.
type Input struct {
	Dates  []time.Time
	Open   []float64
	High   []float64
	Low    []float64
	Close  []float64
	Volume []float64
}

// Result is a single computed indicator series.
type Result struct {
	SubIndicator string // "" for single, "line"/"signal"/"histogram" for multi-output
	Values       []float64
}

// writeRow is a flattened indicator value ready for DB insertion.
type writeRow struct {
	date  time.Time
	key   string
	value float64
}

// indicatorFunc computes one or more output series from OHLCV input.
type indicatorFunc func(ctx context.Context, input *Input, params []int) ([]Result, error)

// batchRegistry maps base indicator names to their compute functions.
var batchRegistry = map[string]indicatorFunc{}

// RegisteredKeys returns all registered base indicator names.
func RegisteredKeys() []string {
	keys := make([]string, 0, len(batchRegistry))
	for k := range batchRegistry {
		keys = append(keys, k)
	}
	return keys
}

// ComputeIndicators computes the requested indicators from OHLCV slices.
// keys are fully qualified indicator names like "sma_20", "macd_12_26_9".
// Returns map[indicatorKey][]Result — same shape as indicator.ComputeIndicators.
func ComputeIndicators(ctx context.Context, input *Input, keys []string) map[string][]Result {
	if input == nil || len(input.Close) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	results := make(map[string][]Result, len(keys))

	for _, key := range keys {
		select {
		case <-ctx.Done():
			return results
		default:
		}

		fn, params := lookupIndicator(key)
		if fn == nil {
			continue
		}

		res, err := fn(ctx, input, params)
		if err != nil {
			continue
		}
		if res != nil {
			results[key] = res
		}
	}

	return results
}

// lookupIndicator finds the compute function and int params for a key.
// Uses progressive matching: tries the full key first, then strips trailing
// params until a match is found in the registry.
// This handles both "sma_20" → ("sma", [20]) and names with underscores
// like "williams_r_14" → ("williams_r", [14]) and "tr" → ("tr", []).
func lookupIndicator(key string) (indicatorFunc, []int) {
	parts := splitString(key, "_")
	for i := len(parts); i >= 1; i-- {
		name := joinString(parts[:i], "_")
		if fn, ok := batchRegistry[name]; ok {
			params := make([]int, 0, len(parts)-i)
			for _, p := range parts[i:] {
				if n, err := parseInt(p); err == nil {
					params = append(params, n)
				}
			}
			return fn, params
		}
	}
	return nil, nil
}

// joinString joins string parts with a separator.
func joinString(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}

// splitString splits a string by a separator (simple, no allocations).
func splitString(s string, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}

// parseInt parses an int from a string.
func parseInt(s string) (int, error) {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, nil
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, nil
}

// SnapshotsToInput converts []*asset.Snapshot to *Input for the engine.
func SnapshotsToInput(snapshots []*asset.Snapshot) *Input {
	n := len(snapshots)
	in := &Input{
		Dates:  make([]time.Time, n),
		Open:   make([]float64, n),
		High:   make([]float64, n),
		Low:    make([]float64, n),
		Close:  make([]float64, n),
		Volume: make([]float64, n),
	}
	for i, s := range snapshots {
		in.Dates[i] = s.Date
		in.Open[i] = s.Open
		in.High[i] = s.High
		in.Low[i] = s.Low
		in.Close[i] = s.Close
		in.Volume[i] = s.Volume
	}
	return in
}

// WriteIndicators bulk-writes computed indicator results to MotherDuck using
// a single multi-row INSERT statement with ON CONFLICT upsert.
// MotherDuck does not support pgx CopyFrom (binary COPY protocol) or
// the extended query protocol used by database/sql prepared statements,
// so we build SQL with positional parameters ($1, $2, ...) directly.
//
// Strategy: send ALL rows in one statement. MotherDuck and PostgreSQL can
// handle multi-million-row INSERT statements as long as the SQL text fits
// within the max statement size (~1 GB default).
func WriteIndicators(ctx context.Context, dsn string, name string, dates []time.Time, results map[string][]Result) error {
	if len(results) == 0 || len(dates) == 0 {
		return nil
	}

	// Flatten results into writeRow tuples.
	rows := make([]writeRow, 0, estimateRowCount(results, len(dates)))
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
				rows = append(rows, writeRow{date: dates[i], key: fullKey, value: v})
			}
		}
	}

	if len(rows) == 0 {
		return nil
	}

	// Open a database/sql connection (works with MotherDuck's PG wire protocol).
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("unable to open database: %w", err)
	}
	defer db.Close()

	// Try one single INSERT for all rows.
	// Build: INSERT INTO indicators (name, date, indicator, value) VALUES ($1,$2,$3,$4),...,($N,$N+1,$N+2,$N+3) ON CONFLICT ...
	var queryBuilder []byte
	queryBuilder = append(queryBuilder, "INSERT INTO indicators (name, date, indicator, value) VALUES "...)
	args := make([]any, 0, len(rows)*4)
	argIdx := 1

	for j, r := range rows {
		if j > 0 {
			queryBuilder = append(queryBuilder, ", "...)
		}
		queryBuilder = append(queryBuilder, '(')
		queryBuilder = append(queryBuilder, fmt.Sprintf("$%d,$%d,$%d,$%d", argIdx, argIdx+1, argIdx+2, argIdx+3)...)
		queryBuilder = append(queryBuilder, ')')
		args = append(args, name, r.date, r.key, r.value)
		argIdx += 4
	}
	queryBuilder = append(queryBuilder, " ON CONFLICT (name, date, indicator) DO UPDATE SET value = EXCLUDED.value"...)

	if _, err := db.ExecContext(ctx, string(queryBuilder), args...); err != nil {
		slog.Warn("Single-batch INSERT failed, trying chunked fallback", "rows", len(rows), "error", err)
		return writeIndicatorsChunked(ctx, db, rows, name)
	}

	slog.Info("Batch write complete (single statement)", "asset", name, "rows", len(rows))
	return nil
}

// writeIndicatorsChunked is the fallback: writes rows in larger chunks.
func writeIndicatorsChunked(ctx context.Context, db *sql.DB, rows []writeRow, name string) error {
	const chunkSize = 5000
	written := 0

	for i := 0; i < len(rows); i += chunkSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		end := i + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[i:end]

		var queryBuilder []byte
		queryBuilder = append(queryBuilder, "INSERT INTO indicators (name, date, indicator, value) VALUES "...)
		args := make([]any, 0, len(chunk)*4)
		argIdx := 1

		for j, r := range chunk {
			if j > 0 {
				queryBuilder = append(queryBuilder, ", "...)
			}
			queryBuilder = append(queryBuilder, '(')
			queryBuilder = append(queryBuilder, fmt.Sprintf("$%d,$%d,$%d,$%d", argIdx, argIdx+1, argIdx+2, argIdx+3)...)
			queryBuilder = append(queryBuilder, ')')
			args = append(args, name, r.date, r.key, r.value)
			argIdx += 4
		}
		queryBuilder = append(queryBuilder, " ON CONFLICT (name, date, indicator) DO UPDATE SET value = EXCLUDED.value"...)

		if _, err := db.ExecContext(ctx, string(queryBuilder), args...); err != nil {
			return fmt.Errorf("chunk upsert failed at offset %d: %w", i, err)
		}
		written += len(chunk)
	}

	slog.Info("Batch write complete (chunked)", "asset", name, "rows", written, "chunks", (written+chunkSize-1)/chunkSize)
	return nil
}

// estimateRowCount pre-calculates total row count for capacity.
func estimateRowCount(results map[string][]Result, maxDates int) int {
	n := 0
	for _, resultList := range results {
		for _, res := range resultList {
			c := len(res.Values)
			if c > maxDates {
				c = maxDates
			}
			n += c
		}
	}
	return n
}

// ExistingIndicators queries the database for which indicator keys already have
// data for a given asset, so we can skip recomputing them.
func ExistingIndicators(ctx context.Context, dsn string, name string) (map[string]bool, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	existing := make(map[string]bool)
	rows, err := db.QueryContext(ctx,
		`SELECT DISTINCT indicator FROM indicators WHERE name = $1`, name)
	if err != nil {
		return existing, err
	}
	defer rows.Close()

	for rows.Next() {
		var indicator string
		if err := rows.Scan(&indicator); err != nil {
			continue
		}
		existing[indicator] = true
	}
	return existing, nil
}

// AllIndicatorKeys returns all possible indicator keys from the registry
// with default parameters.
func AllIndicatorKeys() []string {
	// Map from base name to default param keys
	// These match the registered indicator names with their natural defaults.
	return []string{
		// Trend
		"sma_20", "sma_50", "ema_20", "macd_12_26_9", "vwma_20",
		"apo_14_30", "roc_9", "aroon_25", "bop", "cci_20",
		"cfo_14", "dema_20", "dpo_20", "envelope_20_5",
		"hma_20", "kama_30", "kdj_9", "kst", "mass_index_25",
		"mcginley_20", "mlr_20", "mls_20", "moving_max_20",
		"moving_min_20", "moving_sum_20", "pivot_point",
		"rma_20", "slow_stochastic_14_3", "smma_20",
		"stc_23_2_5", "t3_20", "tema_20", "trima_20", "trix_20",
		"tsi", "typical_price", "weighted_close", "wma_20",
		// Momentum
		"rsi_14", "stoch_14_3", "williams_r_14",
		"awesome_oscillator", "ibs", "chaikin_oscillator_3_10",
		"connors_rsi_14", "coppock_curve", "elder_ray_13",
		"fisher_9", "ichimoku_cloud", "ppo_12_26_9",
		"prings_special_k", "pvo_12_26_9", "qstick_14",
		"rvi_14", "stochastic_rsi_14_3_3", "td_sequential",
		"ultimate_oscillator",
		// Volatility
		"bb_20_2", "atr_14", "tr", "acceleration_bands_20_4",
		"annualized_historical_volatility_20", "bollinger_band_width_20",
		"chandelier_exit_22_3", "chop_14", "donchian_channel_20",
		"historical_volatility_20", "keltner_channel_20_2",
		"moving_std_20", "percent_b_20", "po_14",
		"super_trend_10_3", "ulcer_index_14", "z_score_20",
		// Volume
		"obv", "ad", "cmf_20", "emv_14", "fi_13",
		"kvo_34_55", "mfi_14", "mfm", "mfv", "nvi",
		"vpt", "vwap",
		// Valuation
		"fv_10_5", "npv_10", "pv_10_5",
	}
}
