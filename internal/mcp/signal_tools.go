package mcp

import (
	"context"
	"log/slog"
	"os"
	"time"

	"vercel-go-starter/internal/signal"
	"vercel-go-starter/internal/strategy"

	"github.com/cinar/indicator/v2/asset"
	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ─── Input Types ───────────────────────────────────────────────────────────

type GenerateSignalsInput struct {
	StrategyID int  `json:"strategy_id" jsonschema:"required,strategy ID"`
	Force      bool `json:"force"       jsonschema:"force regeneration, default false"`
}

type QuerySignalsInput struct {
	StrategyID   int    `json:"strategy_id"   jsonschema:"filter by strategy ID"`
	StrategyType string `json:"strategy_type" jsonschema:"filter by strategy type"`
	Underlying   string `json:"underlying"    jsonschema:"filter by ticker symbol"`
	DateFrom     string `json:"date_from"     jsonschema:"start date ISO format, e.g. 2026-01-01"`
	DateTo       string `json:"date_to"       jsonschema:"end date ISO format"`
	Action       string `json:"action"        jsonschema:"filter by action: buy, sell, hold"`
	Limit        int    `json:"limit"         jsonschema:"max results, default 1000"`
	Offset       int    `json:"offset"        jsonschema:"pagination offset"`
}

// ─── Tool Handlers ─────────────────────────────────────────────────────────

func handleGenerateSignals(ctx context.Context, req *mcp.CallToolRequest, input GenerateSignalsInput) (*mcp.CallToolResult, any, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errorResult("DATABASE_URL not set"), nil, nil
	}
	if input.StrategyID <= 0 {
		return errorResult("strategy_id is required"), nil, nil
	}

	// Load strategy from DB.
	conn, err := getConn(dbURL)
	if err != nil {
		return errorResult("Connection failed: " + err.Error()), nil, nil
	}
	defer conn.Close(ctx)

	var stratName, strategyType, underlying, timeframe string
	var lookbackDays int
	var paramsJSON []byte
	err = conn.QueryRow(ctx,
		`SELECT name, strategy_type, underlying, timeframe, lookback_days, parameters FROM strategies WHERE id=$1`,
		input.StrategyID,
	).Scan(&stratName, &strategyType, &underlying, &timeframe, &lookbackDays, &paramsJSON)
	if err != nil {
		return errorResult("Strategy not found: " + err.Error()), nil, nil
	}

	startDate := time.Now().AddDate(0, 0, -lookbackDays)
	endDate := time.Now()

	signalRepo := signal.NewRepository(dbURL)

	// Check existing.
	exists, _ := signalRepo.ExistingSignals(ctx, input.StrategyID, underlying, startDate, endDate)
	if exists && !input.Force {
		filter := signal.SignalFilter{StrategyID: input.StrategyID, Limit: 10000}
		records, total, err := signalRepo.GetSignals(ctx, filter)
		if err != nil {
			return errorResult("Query failed: " + err.Error()), nil, nil
		}
		type sigEntry struct {
			Date   string  `json:"date"`
			Action string  `json:"action"`
			Price  float64 `json:"price"`
		}
		var entries []sigEntry
		for _, r := range records {
			entries = append(entries, sigEntry{
				Date:   r.SignalDate.Format("2006-01-02"),
				Action: r.Action,
				Price:  r.Price,
			})
		}
		return textResult(map[string]any{
			"strategy_id":   input.StrategyID,
			"strategy_name": stratName,
			"underlying":    underlying,
			"signal_count":  total,
			"signals":       entries,
			"cached":        true,
		}), nil, nil
	}

	if input.Force {
		_ = signalRepo.DeleteSignals(ctx, input.StrategyID, underlying, startDate, endDate)
	}

	// Fetch snapshots.
	assetRepo, err := asset.NewRepository("motherduck", dbURL)
	if err != nil {
		return errorResult("Failed to connect to asset repository: " + err.Error()), nil, nil
	}
	snapshotChan, err := assetRepo.GetSince(underlying, startDate)
	if err != nil {
		return errorResult("Failed to fetch snapshots: " + err.Error()), nil, nil
	}
	var snapshots []*asset.Snapshot
	for s := range snapshotChan {
		snapshots = append(snapshots, s)
	}

	if len(snapshots) == 0 {
		return textResult(map[string]any{
			"strategy_id":  input.StrategyID,
			"underlying":   underlying,
			"signal_count": 0,
			"signals":      []any{},
			"cached":       false,
		}), nil, nil
	}

	// Instantiate strategy.
	st, err := strategy.Instantiate(strategyType, nil) // params handled by constructor defaults
	if err != nil {
		return errorResult("Failed to instantiate strategy: " + err.Error()), nil, nil
	}

	// Generate.
	records, err := signal.Generate(ctx, st, snapshots, input.StrategyID, strategyType, underlying)
	if err != nil {
		return errorResult("Signal generation failed: " + err.Error()), nil, nil
	}

	if err := signalRepo.InsertSignals(ctx, records); err != nil {
		slog.Error("Failed to insert signals", "error", err)
	}

	type sigEntry struct {
		Date   string  `json:"date"`
		Action string  `json:"action"`
		Price  float64 `json:"price"`
	}
	var entries []sigEntry
	for _, r := range records {
		entries = append(entries, sigEntry{
			Date:   r.SignalDate.Format("2006-01-02"),
			Action: r.Action,
			Price:  r.Price,
		})
	}

	return textResult(map[string]any{
		"strategy_id":   input.StrategyID,
		"strategy_name": stratName,
		"underlying":    underlying,
		"signal_count":  len(entries),
		"signals":       entries,
		"cached":        false,
	}), nil, nil
}

func handleQuerySignals(ctx context.Context, req *mcp.CallToolRequest, input QuerySignalsInput) (*mcp.CallToolResult, any, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errorResult("DATABASE_URL not set"), nil, nil
	}

	filter := signal.SignalFilter{
		StrategyID:   input.StrategyID,
		StrategyType: input.StrategyType,
		Underlying:   input.Underlying,
		DateFrom:     input.DateFrom,
		DateTo:       input.DateTo,
		Action:       input.Action,
		Limit:        input.Limit,
		Offset:       input.Offset,
	}

	repo := signal.NewRepository(dbURL)
	records, total, err := repo.GetSignals(ctx, filter)
	if err != nil {
		return errorResult("Query failed: " + err.Error()), nil, nil
	}

	type sigEntry struct {
		Date   string  `json:"date"`
		Action string  `json:"action"`
		Price  float64 `json:"price"`
	}
	var entries []sigEntry
	for _, r := range records {
		entries = append(entries, sigEntry{
			Date:   r.SignalDate.Format("2006-01-02"),
			Action: r.Action,
			Price:  r.Price,
		})
	}

	return textResult(map[string]any{
		"signals": entries,
		"count":   len(entries),
		"total":   total,
	}), nil, nil
}

func getConn(dbURL string) (*pgx.Conn, error) {
	return pgx.Connect(context.Background(), dbURL)
}
