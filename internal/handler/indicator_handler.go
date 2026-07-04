package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"time"

	"vercel-go-starter/internal/indicator"
	"vercel-go-starter/internal/model"

	"github.com/cinar/indicator/v2/asset"
	"github.com/cinar/indicator/v2/helper"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) handleListIndicators(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{
			Status: "error", Message: "Method not allowed", Timestamp: now(),
		})
		return
	}

	entries := make([]model.IndicatorEntry, 0, len(indicator.Registry))
	catCounts := make(map[string]int)
	catDescs := map[string]string{
		"trend":      "Identify the direction and strength of price trends",
		"momentum":   "Measure the speed and magnitude of price movements",
		"volatility": "Measure the rate and magnitude of price fluctuations",
		"volume":     "Analyze trading volume to confirm price movements",
	}

	keys := make([]string, 0, len(indicator.Registry))
	for k := range indicator.Registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		def := indicator.Registry[k]
		entries = append(entries, model.IndicatorEntry{
			Name:          def.Key,
			Category:      def.Category,
			DisplayName:   def.DisplayName,
			Description:   def.Description,
			WhenToUse:     def.WhenToUse,
			Inputs:        def.Inputs,
			Outputs:       def.Outputs,
			SubIndicators: def.SubIndicators,
			DefaultParams: def.DefaultParams,
		})
		catCounts[def.Category]++
	}

	categories := make(map[string]model.CatalogCategory)
	for cat, desc := range catDescs {
		categories[cat] = model.CatalogCategory{
			Count:       catCounts[cat],
			Description: desc,
		}
	}

	writeJSON(w, http.StatusOK, model.IndicatorCatalogResponse{
		Indicators: entries,
		Count:      len(entries),
		Categories: categories,
		Timestamp:  now(),
	})
}

func (h *Handler) handleCalculateIndicators(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{
			Status: "error", Message: "Method not allowed", Timestamp: now(),
		})
		return
	}

	if !requireBearerAuth(r) {
		writeJSON(w, http.StatusUnauthorized, model.ErrorResponse{
			Status: "error", Message: "Unauthorized", Timestamp: now(),
		})
		return
	}

	var req model.IndicatorCalculateRequest
	if r.Body != nil {
		data, err := io.ReadAll(r.Body)
		if err == nil && len(data) > 0 {
			if err := json.Unmarshal(data, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
					Status: "error", Message: "Invalid request body", Timestamp: now(),
				})
				return
			}
		}
	}

	// Determine indicators to compute.
	if len(req.Indicators) == 0 {
		req.Indicators = allIndicatorKeys()
	} else {
		for _, name := range req.Indicators {
			if _, ok := indicator.Registry[name]; !ok {
				writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
					Status: "error", Message: "Unknown indicator: " + name, Timestamp: now(),
				})
				return
			}
		}
	}

	if req.Days <= 0 {
		req.Days = 365
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Status: "error", Message: "DATABASE_URL not set", Timestamp: now(),
		})
		return
	}

	// Ensure indicators table exists.
	if err := indicator.EnsureIndicatorsTable(databaseURL); err != nil {
		slog.Error("Failed to create indicators table", "error", err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Status: "error", Message: "Database setup failed", Timestamp: now(),
		})
		return
	}

	// Determine assets.
	if len(req.Assets) == 0 {
		// Get all assets from snapshots table.
		repo, err := asset.NewRepository("motherduck", databaseURL)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Status: "error", Message: "Failed to connect to database", Timestamp: now(),
			})
			return
		}
		assets, err := repo.Assets()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Status: "error", Message: "Failed to list assets", Timestamp: now(),
			})
			return
		}
		req.Assets = assets
	}

	processedAssets := 0

	for _, assetName := range req.Assets {
		if err := calculateForAsset(databaseURL, assetName, req.Indicators, req.Days); err != nil {
			slog.Error("Calculation failed for asset", "asset", assetName, "error", err)
			continue
		}
		processedAssets++
	}

	writeJSON(w, http.StatusOK, model.IndicatorCalculateResponse{
		Status:     "ok",
		Message:    "Indicator calculation completed",
		Assets:     req.Assets,
		Indicators: len(req.Indicators),
		AssetCount: processedAssets,
		Timestamp:  now(),
	})
}

func calculateForAsset(databaseURL, assetName string, indicatorKeys []string, days int) error {
	ctx := context.Background()

	repo, err := asset.NewRepository("motherduck", databaseURL)
	if err != nil {
		return err
	}

	startDate := time.Now().AddDate(0, 0, -days)
	snapshots, err := repo.GetSince(assetName, startDate)
	if err != nil {
		return err
	}

	snapshotSlice := helper.ChanToSlice(snapshots)
	if len(snapshotSlice) == 0 {
		slog.Warn("No snapshots found", "asset", assetName)
		return nil
	}

	// Extract dates for alignment.
	dates := make([]time.Time, len(snapshotSlice))
	for i, s := range snapshotSlice {
		dates[i] = s.Date
	}

	results := indicator.ComputeIndicators(ctx, snapshotSlice, indicatorKeys)
	if len(results) == 0 {
		return nil
	}

	return indicator.BatchUpsertIndicators(databaseURL, assetName, dates, results)
}

func (h *Handler) handleGetIndicatorValues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{
			Status: "error", Message: "Method not allowed", Timestamp: now(),
		})
		return
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Status: "error", Message: "DATABASE_URL not set", Timestamp: now(),
		})
		return
	}

	q := r.URL.Query()
	symbolsParam := q.Get("symbols")
	if symbolsParam == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Status: "error", Message: "Missing 'symbols' query parameter", Timestamp: now(),
		})
		return
	}
	symbols := splitComma(symbolsParam)

	var indicatorsFilter []string
	if indParam := q.Get("indicators"); indParam != "" {
		indicatorsFilter = splitComma(indParam)
	}

	dateFrom := q.Get("date_from")
	dateTo := q.Get("date_to")

	// Build query with individual placeholders (MotherDuck PG doesn't support ANY with arrays).
	query := `SELECT name, indicator, date::text, value FROM indicators WHERE`
	args := []any{}
	argIdx := 1

	// Symbols IN clause
	query += ` name IN (`
	for i, s := range symbols {
		if i > 0 {
			query += ","
		}
		query += "$" + itoa(argIdx)
		args = append(args, s)
		argIdx++
	}
	query += `)`

	// Indicators IN clause
	if len(indicatorsFilter) > 0 {
		query += ` AND indicator IN (`
		for i, s := range indicatorsFilter {
			if i > 0 {
				query += ","
			}
			query += "$" + itoa(argIdx)
			args = append(args, s)
			argIdx++
		}
		query += `)`
	}

	if dateFrom != "" {
		query += ` AND date >= $` + itoa(argIdx)
		args = append(args, dateFrom)
		argIdx++
	}
	if dateTo != "" {
		query += ` AND date <= $` + itoa(argIdx)
		args = append(args, dateTo)
		argIdx++
	}
	query += ` ORDER BY name, indicator, date`

	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Status: "error", Message: "Database connection failed", Timestamp: now(),
		})
		return
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(context.Background(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Status: "error", Message: "Query failed: " + err.Error(), Timestamp: now(),
		})
		return
	}
	defer rows.Close()

	// Group results: data[symbol][indicator] = []DataPoint
	data := make(map[string]map[string][]model.DataPoint)
	total := 0
	for rows.Next() {
		var symbol, indicatorName, date string
		var value float64
		if err := rows.Scan(&symbol, &indicatorName, &date, &value); err != nil {
			continue
		}
		// Keep only YYYY-MM-DD from timestamp
		if len(date) > 10 {
			date = date[:10]
		}
		if data[symbol] == nil {
			data[symbol] = make(map[string][]model.DataPoint)
		}
		data[symbol][indicatorName] = append(data[symbol][indicatorName], model.DataPoint{
			Date:  date,
			Value: value,
		})
		total++
	}

	writeJSON(w, http.StatusOK, model.IndicatorValuesResponse{
		Symbols:    symbols,
		Indicators: indicatorsFilter,
		Data:       data,
		Total:      total,
		Timestamp:  now(),
	})
}

// splitComma splits a comma-separated string, trimming whitespace.
func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := s[start:i]
			// trim leading spaces
			for len(part) > 0 && part[0] == ' ' {
				part = part[1:]
			}
			// trim trailing spaces
			for len(part) > 0 && part[len(part)-1] == ' ' {
				part = part[:len(part)-1]
			}
			if part != "" {
				result = append(result, part)
			}
			start = i + 1
		}
	}
	return result
}

// itoa converts an int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func allIndicatorKeys() []string {
	keys := make([]string, 0, len(indicator.Registry))
	for k := range indicator.Registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
