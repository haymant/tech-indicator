package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"vercel-go-starter/internal/engine"
	"vercel-go-starter/internal/indicator"
	"vercel-go-starter/internal/model"
	"vercel-go-starter/internal/repository"

	"github.com/cinar/indicator/v2/asset"
	"github.com/cinar/indicator/v2/helper"
	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ─── Input/Output Types ────────────────────────────────────────────────────

type SyncAssetInput struct {
	Assets  []string `json:"assets"  jsonschema:"required,ticker symbols to sync, e.g. AAPL,MSFT"`
	Days    int      `json:"days"    jsonschema:"number of lookback days, default 365"`
	Workers int      `json:"workers" jsonschema:"concurrent workers, default 1"`
}

type CalculateIndicatorsInput struct {
	Assets     []string `json:"assets"     jsonschema:"ticker symbols to calculate for, e.g. AAPL,MSFT; defaults to all assets"`
	Indicators []string `json:"indicators" jsonschema:"specific indicator keys, e.g. rsi_14,sma_20; defaults to all 89"`
	Days       int      `json:"days"       jsonschema:"lookback days of snapshots to use, defaults to all available"`
}

type QueryIndicatorValuesInput struct {
	Symbols    string `json:"symbols"    jsonschema:"required,comma-separated ticker symbols, e.g. AAPL,MSFT"`
	Indicators string `json:"indicators" jsonschema:"comma-separated indicator names, e.g. rsi_14,sma_20"`
	DateFrom   string `json:"date_from"  jsonschema:"start date in ISO format, e.g. 2025-01-01"`
	DateTo     string `json:"date_to"    jsonschema:"end date in ISO format, e.g. 2026-07-04"`
}

// ─── Tool Registration ─────────────────────────────────────────────────────

func registerTools(server *mcp.Server) {
	// sync_asset_data
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sync_asset_data",
		Description: "Sync historical OHLCV market data from Tiingo into MotherDuck for one or more ticker symbols.",
	}, handleSyncAssetData)

	// list_indicators
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_indicators",
		Description: "List all available technical indicators with category, description, inputs, and default parameters.",
	}, handleListIndicators)

	// calculate_indicators
	mcp.AddTool(server, &mcp.Tool{
		Name:        "calculate_indicators",
		Description: "Compute technical indicators for specified assets using OHLCV data already synced to MotherDuck. Results are stored in the indicators table with idempotent upsert.",
	}, handleCalculateIndicators)

	// query_indicator_values
	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_indicator_values",
		Description: "Fetch computed indicator values for given symbols and indicators. Returns time-series data points with dates.",
	}, handleQueryIndicatorValues)
}

// ─── Tool Handlers ──────────────────────────────────────────────────────────

func handleSyncAssetData(ctx context.Context, req *mcp.CallToolRequest, input SyncAssetInput) (*mcp.CallToolResult, any, error) {
	tiingoKey := os.Getenv("TIINGO_API_KEY")
	if tiingoKey == "" {
		return errorResult("TIINGO_API_KEY not set"), nil, nil
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errorResult("DATABASE_URL not set"), nil, nil
	}

	if input.Days <= 0 {
		input.Days = 365
	}
	if input.Workers <= 0 {
		input.Workers = 1
	}

	// Normalize asset names to uppercase.
	for i, a := range input.Assets {
		input.Assets[i] = strings.ToUpper(a)
	}

	syncReq := model.SyncRequest{
		Assets:  input.Assets,
		Days:    input.Days,
		Workers: input.Workers,
	}

	repository.RegisterMotherDuck()

	source := asset.NewTiingoRepository(tiingoKey)
	target, err := asset.NewRepository(repository.MotherDuckRepositoryName, databaseURL)
	if err != nil {
		return errorResult("Failed to connect: " + err.Error()), nil, nil
	}

	s := asset.NewSync()
	s.Workers = syncReq.Workers
	s.Assets = syncReq.Assets
	s.Logger = slog.Default()
	startDate := time.Now().AddDate(0, 0, -syncReq.Days)

	if err := s.Run(source, target, startDate); err != nil {
		return errorResult("Sync failed: " + err.Error()), nil, nil
	}

	type result struct {
		Status  string   `json:"status"`
		Message string   `json:"message"`
		Assets  []string `json:"assets"`
		Days    int      `json:"days"`
	}

	return textResult(result{
		Status:  "ok",
		Message: "Sync completed",
		Assets:  input.Assets,
		Days:    input.Days,
	}), nil, nil
}

func handleListIndicators(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
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

	type catalogResult struct {
		Count      int                              `json:"count"`
		Indicators []model.IndicatorEntry           `json:"indicators"`
		Categories map[string]model.CatalogCategory `json:"categories"`
	}

	return textResult(catalogResult{
		Count:      len(entries),
		Indicators: entries,
		Categories: categories,
	}), nil, nil
}

func handleCalculateIndicators(ctx context.Context, req *mcp.CallToolRequest, input CalculateIndicatorsInput) (*mcp.CallToolResult, any, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errorResult("DATABASE_URL not set"), nil, nil
	}

	if input.Days <= 0 {
		input.Days = 365
	}

	// Determine indicators.
	if len(input.Indicators) == 0 {
		input.Indicators = allIndicatorKeys()
	}

	// Normalize asset names to uppercase.
	for i, a := range input.Assets {
		input.Assets[i] = strings.ToUpper(a)
	}

	// Determine assets.
	if len(input.Assets) == 0 {
		repo, err := asset.NewRepository("motherduck", databaseURL)
		if err != nil {
			return errorResult("Failed to connect: " + err.Error()), nil, nil
		}
		assets, err := repo.Assets()
		if err != nil {
			return errorResult("Failed to list assets: " + err.Error()), nil, nil
		}
		input.Assets = assets
	}

	processed := 0
	var errors []string

	for _, assetName := range input.Assets {
		if err := calculateForAsset(ctx, databaseURL, assetName, input.Indicators, input.Days); err != nil {
			slog.Error("Calculation failed for asset", "asset", assetName, "error", err)
			errors = append(errors, assetName+": "+err.Error())
			continue
		}
		processed++
	}

	type calcResult struct {
		Status     string   `json:"status"`
		Message    string   `json:"message"`
		Assets     []string `json:"assets"`
		Indicators int      `json:"indicators"`
		Processed  int      `json:"processed"`
		Errors     []string `json:"errors,omitempty"`
	}

	return textResult(calcResult{
		Status:     "ok",
		Message:    "Indicator calculation completed",
		Assets:     input.Assets,
		Indicators: len(input.Indicators),
		Processed:  processed,
		Errors:     errors,
	}), nil, nil
}

func handleQueryIndicatorValues(ctx context.Context, req *mcp.CallToolRequest, input QueryIndicatorValuesInput) (*mcp.CallToolResult, any, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errorResult("DATABASE_URL not set"), nil, nil
	}

	if input.Symbols == "" {
		return errorResult("symbols is required"), nil, nil
	}

	symbols := splitComma(input.Symbols)
	for i, s := range symbols {
		symbols[i] = strings.ToUpper(s)
	}
	var indicatorsFilter []string
	if input.Indicators != "" {
		indicatorsFilter = splitComma(input.Indicators)
	}

	// Build query.
	query := `SELECT name, indicator, date::text, value FROM indicators WHERE`
	args := []any{}
	argIdx := 1

	query += ` LOWER(name) IN (`
	for i, s := range symbols {
		if i > 0 {
			query += ","
		}
		query += "LOWER($" + itoa(argIdx) + ")"
		args = append(args, s)
		argIdx++
	}
	query += `)`

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

	if input.DateFrom != "" {
		query += ` AND date >= $` + itoa(argIdx)
		args = append(args, input.DateFrom)
		argIdx++
	}
	if input.DateTo != "" {
		query += ` AND date <= $` + itoa(argIdx)
		args = append(args, input.DateTo)
		argIdx++
	}
	query += ` ORDER BY name, indicator, date`

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return errorResult("Database connection failed: " + err.Error()), nil, nil
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return errorResult("Query failed: " + err.Error()), nil, nil
	}
	defer rows.Close()

	data := make(map[string]map[string][]model.DataPoint)
	total := 0
	for rows.Next() {
		var symbol, indicatorName, date string
		var value float64
		if err := rows.Scan(&symbol, &indicatorName, &date, &value); err != nil {
			continue
		}
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

	type queryResult struct {
		Symbols    string                                  `json:"symbols"`
		Indicators []string                                `json:"indicators"`
		Total      int                                     `json:"total"`
		Data       map[string]map[string][]model.DataPoint `json:"data"`
	}

	return textResult(queryResult{
		Symbols:    input.Symbols,
		Indicators: indicatorsFilter,
		Total:      total,
		Data:       data,
	}), nil, nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func textResult(v any) *mcp.CallToolResult {
	b, _ := json.Marshal(v)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(b)},
		},
	}
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}

func allIndicatorKeys() []string {
	return engine.AllIndicatorKeys()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := s[start:i]
			for len(part) > 0 && part[0] == ' ' {
				part = part[1:]
			}
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

// calculateForAsset reuses the logic from the handler package but accesses DB directly.
func calculateForAsset(ctx context.Context, databaseURL, assetName string, indicatorKeys []string, days int) error {
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

	dates := make([]time.Time, len(snapshotSlice))
	for i, s := range snapshotSlice {
		dates[i] = s.Date
	}

	// Check existing indicators for caching.
	existing, err := engine.ExistingIndicators(ctx, databaseURL, assetName)
	if err != nil {
		slog.Warn("Could not check existing indicators", "asset", assetName, "error", err)
		existing = nil
	}

	// Filter out cached.
	needed := indicatorKeys[:0]
	for _, k := range indicatorKeys {
		if existing != nil && existing[k] {
			continue
		}
		if existing != nil {
			found := false
			for ek := range existing {
				if len(ek) >= len(k) && ek[:len(k)] == k {
					found = true
					break
				}
			}
			if found {
				continue
			}
		}
		needed = append(needed, k)
	}

	if len(needed) == 0 {
		slog.Info("All indicators already cached", "asset", assetName)
		return nil
	}

	slog.Info("Computing indicators", "asset", assetName, "new", len(needed))

	input := engine.SnapshotsToInput(snapshotSlice)
	results := engine.ComputeIndicators(ctx, input, needed)
	if len(results) == 0 {
		return nil
	}

	return engine.WriteIndicators(ctx, databaseURL, assetName, dates, results)
}
