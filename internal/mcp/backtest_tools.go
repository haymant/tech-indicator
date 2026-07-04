package mcp

import (
	"context"
	"log/slog"
	"os"
	"time"

	"vercel-go-starter/internal/backtest"
	"vercel-go-starter/internal/strategy"

	"github.com/cinar/indicator/v2/asset"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ─── Input Types ───────────────────────────────────────────────────────────

type RunBacktestInput struct {
	StrategyID int  `json:"strategy_id" jsonschema:"required,strategy ID"`
	Force      bool `json:"force"       jsonschema:"force rerun, default false"`
}

type QueryBacktestResultsInput struct {
	StrategyID   int     `json:"strategy_id"   jsonschema:"filter by strategy ID"`
	StrategyType string  `json:"strategy_type" jsonschema:"filter by strategy type"`
	Underlying   string  `json:"underlying"    jsonschema:"filter by ticker symbol"`
	DateFrom     string  `json:"date_from"     jsonschema:"min backtest end date, ISO format"`
	DateTo       string  `json:"date_to"       jsonschema:"max backtest end date, ISO format"`
	MinReturn    float64 `json:"min_return"    jsonschema:"minimum total return filter"`
	Limit        int     `json:"limit"         jsonschema:"max results, default 100"`
	Offset       int     `json:"offset"        jsonschema:"pagination offset"`
}

// ─── Tool Handlers ─────────────────────────────────────────────────────────

func handleRunBacktest(ctx context.Context, req *mcp.CallToolRequest, input RunBacktestInput) (*mcp.CallToolResult, any, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errorResult("DATABASE_URL not set"), nil, nil
	}
	if input.StrategyID <= 0 {
		return errorResult("strategy_id is required"), nil, nil
	}

	// Load strategy.
	conn, err := getConn(dbURL)
	if err != nil {
		return errorResult("Connection failed: " + err.Error()), nil, nil
	}
	defer conn.Close(ctx)

	var stratName, strategyType, underlying string
	var lookbackDays int
	var paramsJSON []byte
	err = conn.QueryRow(ctx,
		`SELECT name, strategy_type, underlying, lookback_days, parameters FROM strategies WHERE id=$1`,
		input.StrategyID,
	).Scan(&stratName, &strategyType, &underlying, &lookbackDays, &paramsJSON)
	if err != nil {
		return errorResult("Strategy not found: " + err.Error()), nil, nil
	}

	startDate := time.Now().AddDate(0, 0, -lookbackDays)
	endDate := time.Now()

	btRepo := backtest.NewRepository(dbURL)

	// Check existing.
	existing, _ := btRepo.ExistingResult(ctx, input.StrategyID, underlying, startDate, endDate)
	if existing != nil && !input.Force {
		return textResult(map[string]any{
			"strategy_id":      input.StrategyID,
			"strategy_name":    stratName,
			"underlying":       underlying,
			"total_return":     existing.TotalReturn,
			"max_drawdown":     existing.MaxDrawdown,
			"sharpe_ratio":     existing.SharpeRatio,
			"win_rate":         existing.WinRate,
			"num_transactions": existing.NumTransactions,
			"final_outcome":    existing.FinalOutcome,
			"final_action":     existing.FinalAction,
			"cached":           true,
		}), nil, nil
	}

	if input.Force && existing != nil {
		_ = btRepo.DeleteResult(ctx, input.StrategyID, underlying, startDate, endDate)
	}

	// Fetch snapshots.
	assetRepo, err := asset.NewRepository("motherduck", dbURL)
	if err != nil {
		return errorResult("Failed to connect: " + err.Error()), nil, nil
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
			"strategy_id": input.StrategyID,
			"underlying":  underlying,
			"message":     "no snapshots available",
		}), nil, nil
	}

	// Instantiate strategy.
	st, err := strategy.Instantiate(strategyType, nil)
	if err != nil {
		return errorResult("Failed to instantiate strategy: " + err.Error()), nil, nil
	}

	// Run backtest.
	result, err := backtest.Run(ctx, st, snapshots, input.StrategyID, strategyType, underlying, nil)
	if err != nil {
		return errorResult("Backtest failed: " + err.Error()), nil, nil
	}

	if result != nil {
		_ = btRepo.InsertResult(ctx, result)
	}

	return textResult(map[string]any{
		"strategy_id":      input.StrategyID,
		"strategy_name":    stratName,
		"underlying":       underlying,
		"total_return":     result.TotalReturn,
		"max_drawdown":     result.MaxDrawdown,
		"sharpe_ratio":     result.SharpeRatio,
		"win_rate":         result.WinRate,
		"num_transactions": result.NumTransactions,
		"final_outcome":    result.FinalOutcome,
		"final_action":     result.FinalAction,
		"cached":           false,
	}), nil, nil
}

func handleQueryBacktestResults(ctx context.Context, req *mcp.CallToolRequest, input QueryBacktestResultsInput) (*mcp.CallToolResult, any, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errorResult("DATABASE_URL not set"), nil, nil
	}

	filter := backtest.BacktestFilter{
		StrategyID:   input.StrategyID,
		StrategyType: input.StrategyType,
		Underlying:   input.Underlying,
		DateFrom:     input.DateFrom,
		DateTo:       input.DateTo,
		MinReturn:    input.MinReturn,
		Limit:        input.Limit,
		Offset:       input.Offset,
	}

	repo := backtest.NewRepository(dbURL)
	results, total, err := repo.GetResults(ctx, filter)
	if err != nil {
		slog.Error("Failed to query backtest results", "error", err)
		return errorResult("Query failed: " + err.Error()), nil, nil
	}

	type resultEntry struct {
		StrategyID      int     `json:"strategy_id"`
		StrategyType    string  `json:"strategy_type"`
		Underlying      string  `json:"underlying"`
		TotalReturn     float64 `json:"total_return"`
		MaxDrawdown     float64 `json:"max_drawdown"`
		SharpeRatio     float64 `json:"sharpe_ratio"`
		WinRate         float64 `json:"win_rate"`
		NumTransactions int     `json:"num_transactions"`
		FinalOutcome    float64 `json:"final_outcome"`
		FinalAction     string  `json:"final_action"`
	}

	var entries []resultEntry
	for _, r := range results {
		entries = append(entries, resultEntry{
			StrategyID:      r.StrategyID,
			StrategyType:    r.StrategyType,
			Underlying:      r.Underlying,
			TotalReturn:     r.TotalReturn,
			MaxDrawdown:     r.MaxDrawdown,
			SharpeRatio:     r.SharpeRatio,
			WinRate:         r.WinRate,
			NumTransactions: r.NumTransactions,
			FinalOutcome:    r.FinalOutcome,
			FinalAction:     r.FinalAction,
		})
	}

	return textResult(map[string]any{
		"results": entries,
		"count":   total,
	}), nil, nil
}
