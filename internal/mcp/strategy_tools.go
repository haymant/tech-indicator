package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"

	"vercel-go-starter/internal/database"

	"vercel-go-starter/internal/model"
	"vercel-go-starter/internal/strategy"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ─── Input Types ───────────────────────────────────────────────────────────

type CreateStrategyInput struct {
	Name         string         `json:"name"         jsonschema:"required,strategy name, must be unique"`
	StrategyType string         `json:"strategy_type" jsonschema:"required,strategy type key from list_strategy_types"`
	Underlying   string         `json:"underlying"   jsonschema:"required,ticker symbol, e.g. AAPL"`
	Timeframe    string         `json:"timeframe"    jsonschema:"data interval, default 1d"`
	LookbackDays int            `json:"lookback_days" jsonschema:"lookback days, default 365"`
	Parameters   map[string]any `json:"parameters"   jsonschema:"strategy-specific parameters as JSON object"`
	Force        bool           `json:"force"        jsonschema:"overwrite existing strategy with same name if true, default false"`
}

type ListStrategiesInput struct {
	StrategyType string `json:"strategy_type" jsonschema:"filter by strategy type"`
	Underlying   string `json:"underlying"    jsonschema:"filter by ticker symbol"`
	Name         string `json:"name"          jsonschema:"search by name (partial match)"`
}

// ─── Tool Handlers ─────────────────────────────────────────────────────────

func handleListStrategyTypes(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
	entries := strategy.ListTypes()
	cats := strategy.Categories()

	return textResult(map[string]any{
		"strategies": entries,
		"count":      len(entries),
		"categories": cats,
	}), nil, nil
}

func handleListStrategies(ctx context.Context, req *mcp.CallToolRequest, input ListStrategiesInput) (*mcp.CallToolResult, any, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errorResult("DATABASE_URL not set"), nil, nil
	}

	// Build filter query.
	where := ""
	args := []any{}
	argIdx := 1

	if input.StrategyType != "" {
		where += " AND strategy_type=$1"
		args = append(args, input.StrategyType)
		argIdx = 2
	}
	if input.Underlying != "" {
		where += " AND LOWER(underlying)=LOWER($" + itoa(argIdx) + ")"
		args = append(args, input.Underlying)
		argIdx++
	}
	if input.Name != "" {
		where += " AND LOWER(name) LIKE LOWER($" + itoa(argIdx) + ")"
		args = append(args, "%"+input.Name+"%")
		argIdx++
	}

	type strategyResult struct {
		ID           int            `json:"id"`
		Name         string         `json:"name"`
		StrategyType string         `json:"strategy_type"`
		Underlying   string         `json:"underlying"`
		Timeframe    string         `json:"timeframe"`
		LookbackDays int            `json:"lookback_days"`
		Parameters   map[string]any `json:"parameters"`
		CreatedAt    string         `json:"created_at"`
	}

	results, err := queryStrategiesRaw(dbURL, where, args)
	if err != nil {
		slog.Error("Failed to list strategies", "error", err)
		return errorResult("Query failed: " + err.Error()), nil, nil
	}

	return textResult(map[string]any{
		"strategies": results,
		"count":      len(results),
	}), nil, nil
}

func handleCreateStrategy(ctx context.Context, req *mcp.CallToolRequest, input CreateStrategyInput) (*mcp.CallToolResult, any, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errorResult("DATABASE_URL not set"), nil, nil
	}

	if input.Name == "" {
		return errorResult("name is required"), nil, nil
	}
	if !strategy.IsValid(input.StrategyType) {
		return errorResult("unknown strategy_type: " + input.StrategyType), nil, nil
	}
	if input.Underlying == "" {
		return errorResult("underlying is required"), nil, nil
	}
	if input.Timeframe == "" {
		input.Timeframe = "1d"
	}
	if input.LookbackDays <= 0 {
		input.LookbackDays = 365
	}

	input.Underlying = strings.ToUpper(input.Underlying)

	// Ensure tables exist.
	if err := database.RunMigrations(dbURL); err != nil {
		slog.Error("Migration failed", "error", err)
		return errorResult("Database setup failed: " + err.Error()), nil, nil
	}

	// Build DB insert.
	conn, err := getConn(dbURL)
	if err != nil {
		return errorResult("Connection failed: " + err.Error()), nil, nil
	}
	defer conn.Close(ctx)

	paramsJSON, _ := json.Marshal(input.Parameters)

	var id int
	err = conn.QueryRow(ctx,
		`INSERT INTO strategies (id, name, strategy_type, underlying, timeframe, lookback_days, parameters) VALUES ((SELECT COALESCE(MAX(id), 0) + 1 FROM strategies), $1,$2,$3,$4,$5,$6) RETURNING id`,
		input.Name, input.StrategyType, input.Underlying, input.Timeframe, input.LookbackDays, paramsJSON,
	).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "uq_strategies_name") || strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "Duplicate") {
			return errorResult("Strategy name already exists"), nil, nil
		}
		return errorResult("Failed to create: " + err.Error()), nil, nil
	}

	return textResult(model.StrategyResponse{
		ID:           id,
		Name:         input.Name,
		StrategyType: input.StrategyType,
		Underlying:   input.Underlying,
		Timeframe:    input.Timeframe,
		LookbackDays: input.LookbackDays,
		Parameters:   input.Parameters,
		CreatedAt:    nowStr(),
		UpdatedAt:    nowStr(),
	}), nil, nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func queryStrategiesRaw(dbURL, where string, args []any) ([]map[string]any, error) {
	conn, err := getConn(dbURL)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())

	if len(where) > 0 {
		where = " WHERE" + where[4:]
	}

	rows, err := conn.Query(context.Background(),
		`SELECT id, name, strategy_type, underlying, timeframe, lookback_days, parameters, created_at::text, updated_at::text FROM strategies`+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id int
		var name, strategyType, underlying, timeframe, createdAt, updatedAt string
		var lookbackDays int
		var paramsJSON []byte

		if err := rows.Scan(&id, &name, &strategyType, &underlying, &timeframe, &lookbackDays, &paramsJSON, &createdAt, &updatedAt); err != nil {
			continue
		}

		params := make(map[string]any)
		if len(paramsJSON) > 0 {
			json.Unmarshal(paramsJSON, &params)
		}

		results = append(results, map[string]any{
			"id":            id,
			"name":          name,
			"strategy_type": strategyType,
			"underlying":    underlying,
			"timeframe":     timeframe,
			"lookback_days": lookbackDays,
			"parameters":    params,
			"created_at":    createdAt,
			"updated_at":    updatedAt,
		})
	}
	return results, nil
}

func nowStr() string {
	return "just now" // simplified for MCP output
}
