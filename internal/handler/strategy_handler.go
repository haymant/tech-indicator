package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"vercel-go-starter/internal/backtest"
	"vercel-go-starter/internal/database"
	"vercel-go-starter/internal/model"
	"vercel-go-starter/internal/signal"
	"vercel-go-starter/internal/strategy"

	"github.com/cinar/indicator/v2/asset"
	"github.com/jackc/pgx/v5"
)

// ─── GET /api/strategies/types ─────────────────────────────────────────────

func (h *Handler) handleListStrategyTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Status: "error", Message: "Method not allowed", Timestamp: now()})
		return
	}

	entries := strategy.ListTypes()
	cats := strategy.Categories()

	writeJSON(w, http.StatusOK, model.StrategyTypesResponse{
		Strategies: entries,
		Count:      len(entries),
		Categories: cats,
	})
}

// ─── GET /api/strategies ───────────────────────────────────────────────────

func (h *Handler) handleListStrategies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Status: "error", Message: "Method not allowed", Timestamp: now()})
		return
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "DATABASE_URL not set", Timestamp: now()})
		return
	}

	q := r.URL.Query()
	strategyType := q.Get("strategy_type")
	underlying := q.Get("underlying")
	name := q.Get("name")

	strategies, total, err := queryStrategies(dbURL, strategyType, underlying, name)
	if err != nil {
		slog.Error("Failed to query strategies", "error", err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Query failed", Timestamp: now()})
		return
	}

	resp := model.StrategyListResponse{
		Strategies: strategies,
		Count:      total,
	}
	if resp.Strategies == nil {
		resp.Strategies = []model.StrategyResponse{}
	}

	writeJSON(w, http.StatusOK, resp)
}

// ─── POST /api/strategies ──────────────────────────────────────────────────

func (h *Handler) handleCreateStrategy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Status: "error", Message: "Method not allowed", Timestamp: now()})
		return
	}
	if !requireBearerAuth(r) {
		writeJSON(w, http.StatusUnauthorized, model.ErrorResponse{Status: "error", Message: "Unauthorized", Timestamp: now()})
		return
	}

	var req model.StrategyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Status: "error", Message: "Invalid JSON", Timestamp: now()})
		return
	}

	// Validate.
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Status: "error", Message: "name is required", Timestamp: now()})
		return
	}
	if !strategy.IsValid(req.StrategyType) {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Status: "error", Message: "unknown strategy_type: " + req.StrategyType, Timestamp: now()})
		return
	}
	if req.Underlying == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Status: "error", Message: "underlying is required", Timestamp: now()})
		return
	}
	if req.Timeframe == "" {
		req.Timeframe = "1d"
	}
	if req.LookbackDays <= 0 {
		req.LookbackDays = 365
	}

	// Normalize underlying to uppercase.
	req.Underlying = strings.ToUpper(req.Underlying)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "DATABASE_URL not set", Timestamp: now()})
		return
	}

	// Ensure tables exist.
	if err := database.RunMigrations(dbURL); err != nil {
		slog.Error("Migration failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Database setup failed", Timestamp: now()})
		return
	}

	id, err := insertStrategy(dbURL, req)
	if err != nil {
		if strings.Contains(err.Error(), "uq_strategies_name") || strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "Duplicate") {
			// If force is true, delete the existing strategy and recreate.
			if req.Force {
				if delErr := deleteStrategyByName(dbURL, req.Name); delErr != nil {
					slog.Error("Failed to delete existing strategy for force recreate", "error", delErr)
					writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Force recreate failed", Timestamp: now()})
					return
				}
				// Retry insert.
				id, err = insertStrategy(dbURL, req)
				if err != nil {
					slog.Error("Failed to create strategy after force delete", "error", err)
					writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Failed to create strategy", Timestamp: now()})
					return
				}
			} else {
				writeJSON(w, http.StatusConflict, model.ErrorResponse{Status: "error", Message: "Strategy name already exists", Timestamp: now()})
				return
			}
		} else {
			slog.Error("Failed to create strategy", "error", err)
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Failed to create strategy", Timestamp: now()})
			return
		}
	}

	writeJSON(w, http.StatusCreated, model.StrategyResponse{
		ID:           id,
		Name:         req.Name,
		StrategyType: req.StrategyType,
		Underlying:   req.Underlying,
		Timeframe:    req.Timeframe,
		LookbackDays: req.LookbackDays,
		Parameters:   req.Parameters,
		CreatedAt:    now(),
		UpdatedAt:    now(),
	})
}

// ─── POST /api/strategies/{id}/signals and /api/strategies/{id}/backtest ────

func (h *Handler) handleStrategyByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Status: "error", Message: "Method not allowed", Timestamp: now()})
		return
	}
	if !requireBearerAuth(r) {
		writeJSON(w, http.StatusUnauthorized, model.ErrorResponse{Status: "error", Message: "Unauthorized", Timestamp: now()})
		return
	}

	// Parse /api/strategies/{id}/...
	path := strings.TrimPrefix(r.URL.Path, "/api/strategies/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Status: "error", Message: "Invalid path", Timestamp: now()})
		return
	}

	idStr := parts[0]
	action := parts[1]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Status: "error", Message: "Invalid strategy ID", Timestamp: now()})
		return
	}

	switch action {
	case "signals":
		h.handleGenerateSignals(w, r, id)
	case "backtest":
		h.handleRunBacktest(w, r, id)
	default:
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{Status: "error", Message: "Unknown action: " + action, Timestamp: now()})
	}
}

// ─── POST /api/strategies/{id}/signals ─────────────────────────────────────

func (h *Handler) handleGenerateSignals(w http.ResponseWriter, r *http.Request, strategyID int) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "DATABASE_URL not set", Timestamp: now()})
		return
	}

	// Parse optional force flag.
	force := false
	if r.Body != nil {
		var req model.SignalGenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			force = req.Force
		}
	}

	// Load strategy from DB.
	ctx := context.Background()
	strat, err := loadStrategyByID(ctx, dbURL, strategyID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{Status: "error", Message: "Strategy not found", Timestamp: now()})
		return
	}

	startDate := time.Now().AddDate(0, 0, -strat.LookbackDays)
	endDate := time.Now()

	signalRepo := signal.NewRepository(dbURL)

	// Check existing signals.
	exists, err := signalRepo.ExistingSignals(ctx, strategyID, strat.Underlying, startDate, endDate)
	if err != nil {
		slog.Error("Failed to check existing signals", "error", err)
	}
	if exists && !force {
		// Return cached signals.
		filter := signal.SignalFilter{StrategyID: strategyID, Limit: 10000}
		records, total, err := signalRepo.GetSignals(ctx, filter)
		if err != nil {
			slog.Error("Failed to query cached signals", "error", err)
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Query failed", Timestamp: now()})
			return
		}
		signalRecords := toModelSignalRecords(records)
		writeJSON(w, http.StatusOK, model.SignalGenerateResponse{
			StrategyID:   strategyID,
			StrategyName: strat.Name,
			Underlying:   strat.Underlying,
			SignalCount:  total,
			Signals:      signalRecords,
			Cached:       true,
			GeneratedAt:  now(),
		})
		return
	}

	if force {
		_ = signalRepo.DeleteSignals(ctx, strategyID, strat.Underlying, startDate, endDate)
	}

	// Fetch OHLCV snapshots.
	assetRepo, err := asset.NewRepository("motherduck", dbURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Failed to connect: " + err.Error(), Timestamp: now()})
		return
	}

	snapshotChan, err := assetRepo.GetSince(strat.Underlying, startDate)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Failed to fetch snapshots: " + err.Error(), Timestamp: now()})
		return
	}
	snapshots := helperChanToSlice(snapshotChan)

	if len(snapshots) == 0 {
		writeJSON(w, http.StatusOK, model.SignalGenerateResponse{
			StrategyID:  strategyID,
			Underlying:  strat.Underlying,
			SignalCount: 0,
			Signals:     []model.SignalRecord{},
			Cached:      false,
			GeneratedAt: now(),
		})
		return
	}

	// Instantiate strategy.
	st, err := strategy.Instantiate(strat.StrategyType, strat.Parameters)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Failed to instantiate strategy: " + err.Error(), Timestamp: now()})
		return
	}

	// Generate signals.
	records, err := signal.Generate(ctx, st, snapshots, strategyID, strat.StrategyType, strat.Underlying)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Signal generation failed: " + err.Error(), Timestamp: now()})
		return
	}

	// Batch insert.
	if err := signalRepo.InsertSignals(ctx, records); err != nil {
		slog.Error("Failed to insert signals", "error", err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Failed to persist signals: " + err.Error(), Timestamp: now()})
		return
	}

	signalRecords := toModelSignalRecords(records)

	writeJSON(w, http.StatusCreated, model.SignalGenerateResponse{
		StrategyID:   strategyID,
		StrategyName: strat.Name,
		Underlying:   strat.Underlying,
		SignalCount:  len(signalRecords),
		Signals:      signalRecords,
		Cached:       false,
		GeneratedAt:  now(),
	})
}

// ─── POST /api/strategies/{id}/backtest ────────────────────────────────────

func (h *Handler) handleRunBacktest(w http.ResponseWriter, r *http.Request, strategyID int) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "DATABASE_URL not set", Timestamp: now()})
		return
	}

	force := false
	if r.Body != nil {
		var req model.BacktestRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			force = req.Force
		}
	}

	ctx := context.Background()
	strat, err := loadStrategyByID(ctx, dbURL, strategyID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{Status: "error", Message: "Strategy not found", Timestamp: now()})
		return
	}

	startDate := time.Now().AddDate(0, 0, -strat.LookbackDays)
	endDate := time.Now()

	btRepo := backtest.NewRepository(dbURL)

	// Check existing result.
	existing, err := btRepo.ExistingResult(ctx, strategyID, strat.Underlying, startDate, endDate)
	if err == nil && existing != nil && !force {
		resp := toBacktestRunResponse(strat, existing, true)
		writeJSON(w, http.StatusOK, resp)
		return
	}

	if force && existing != nil {
		_ = btRepo.DeleteResult(ctx, strategyID, strat.Underlying, startDate, endDate)
	}

	// Fetch snapshots.
	assetRepo, err := asset.NewRepository("motherduck", dbURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Failed to connect: " + err.Error(), Timestamp: now()})
		return
	}

	snapshotChan, err := assetRepo.GetSince(strat.Underlying, startDate)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Failed to fetch snapshots: " + err.Error(), Timestamp: now()})
		return
	}
	snapshots := helperChanToSlice(snapshotChan)

	if len(snapshots) == 0 {
		writeJSON(w, http.StatusOK, model.BacktestRunResponse{
			BacktestResultResponse: model.BacktestResultResponse{
				StrategyID:  strategyID,
				Underlying:  strat.Underlying,
				FinalAction: "hold",
				Cached:      false,
				GeneratedAt: now(),
			},
		})
		return
	}

	// Instantiate strategy.
	st, err := strategy.Instantiate(strat.StrategyType, strat.Parameters)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Failed to instantiate strategy: " + err.Error(), Timestamp: now()})
		return
	}

	// Run backtest.
	btResult, err := backtest.Run(ctx, st, snapshots, strategyID, strat.StrategyType, strat.Underlying, strat.Parameters)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Backtest failed: " + err.Error(), Timestamp: now()})
		return
	}

	// Store result.
	if btResult != nil {
		if err := btRepo.InsertResult(ctx, btResult); err != nil {
			slog.Error("Failed to persist backtest result", "error", err)
		}
	}

	resp := toBacktestRunResponse(strat, btResult, false)
	writeJSON(w, http.StatusCreated, resp)
}

// ─── Helper Types and Functions ────────────────────────────────────────────

type dbStrategy struct {
	ID           int
	Name         string
	StrategyType string
	Underlying   string
	Timeframe    string
	LookbackDays int
	Parameters   map[string]any
}

func loadStrategyByID(ctx context.Context, dbURL string, id int) (*dbStrategy, error) {
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var s dbStrategy
	var paramsJSON []byte
	err = conn.QueryRow(ctx,
		`SELECT id, name, strategy_type, underlying, timeframe, lookback_days, parameters FROM strategies WHERE id=$1`, id,
	).Scan(&s.ID, &s.Name, &s.StrategyType, &s.Underlying, &s.Timeframe, &s.LookbackDays, &paramsJSON)
	if err != nil {
		return nil, err
	}
	if len(paramsJSON) > 0 {
		json.Unmarshal(paramsJSON, &s.Parameters)
	}
	return &s, nil
}

func queryStrategies(dbURL, strategyType, underlying, name string) ([]model.StrategyResponse, int, error) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close(ctx)

	where := ""
	args := []any{}
	argIdx := 1

	if strategyType != "" {
		where += fmt.Sprintf(" AND strategy_type=$%d", argIdx)
		args = append(args, strategyType)
		argIdx++
	}
	if underlying != "" {
		where += fmt.Sprintf(" AND LOWER(underlying)=LOWER($%d)", argIdx)
		args = append(args, underlying)
		argIdx++
	}
	if name != "" {
		where += fmt.Sprintf(" AND LOWER(name) LIKE LOWER($%d)", argIdx)
		args = append(args, "%"+name+"%")
		argIdx++
	}
	if len(where) > 0 {
		where = " WHERE" + where[4:]
	}

	var total int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM strategies`+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := conn.Query(ctx,
		`SELECT id, name, strategy_type, underlying, timeframe, lookback_days, parameters, created_at::text, updated_at::text FROM strategies`+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []model.StrategyResponse
	for rows.Next() {
		var res model.StrategyResponse
		var paramsJSON []byte
		var createdAt, updatedAt string
		if err := rows.Scan(&res.ID, &res.Name, &res.StrategyType, &res.Underlying, &res.Timeframe, &res.LookbackDays, &paramsJSON, &createdAt, &updatedAt); err != nil {
			continue
		}
		res.CreatedAt = createdAt
		res.UpdatedAt = updatedAt
		if len(paramsJSON) > 0 {
			json.Unmarshal(paramsJSON, &res.Parameters)
		}
		results = append(results, res)
	}
	if results == nil {
		results = []model.StrategyResponse{}
	}
	return results, total, nil
}

func insertStrategy(dbURL string, req model.StrategyCreateRequest) (int, error) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return 0, err
	}
	defer conn.Close(ctx)

	paramsJSON, _ := json.Marshal(req.Parameters)

	var id int
	err = conn.QueryRow(ctx,
		`INSERT INTO strategies (id, name, strategy_type, underlying, timeframe, lookback_days, parameters) VALUES ((SELECT COALESCE(MAX(id), 0) + 1 FROM strategies), $1,$2,$3,$4,$5,$6) RETURNING id`,
		req.Name, req.StrategyType, req.Underlying, req.Timeframe, req.LookbackDays, paramsJSON,
	).Scan(&id)
	return id, err
}

func deleteStrategyByName(dbURL, name string) error {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `DELETE FROM strategies WHERE name = $1`, name)
	return err
}

func toModelSignalRecords(records []signal.SignalRecord) []model.SignalRecord {
	result := make([]model.SignalRecord, 0, len(records))
	for _, r := range records {
		result = append(result, model.SignalRecord{
			ID:           0,
			StrategyID:   r.StrategyID,
			StrategyType: r.StrategyType,
			StrategyName: "",
			Underlying:   r.Underlying,
			SignalDate:   r.SignalDate.Format("2006-01-02"),
			Action:       r.Action,
			Price:        r.Price,
		})
	}
	return result
}

func toBacktestRunResponse(strat *dbStrategy, btResult *backtest.Result, cached bool) model.BacktestRunResponse {
	tr, mr, sr, wr, fo := 0.0, 0.0, 0.0, 0.0, 0.0
	nt := 0
	fa := "hold"
	sd := ""
	ed := ""

	if btResult != nil {
		tr = btResult.TotalReturn
		mr = btResult.MaxDrawdown
		sr = btResult.SharpeRatio
		wr = btResult.WinRate
		nt = btResult.NumTransactions
		fo = btResult.FinalOutcome
		fa = btResult.FinalAction
		sd = btResult.StartDate.Format("2006-01-02")
		ed = btResult.EndDate.Format("2006-01-02")
	}

	return model.BacktestRunResponse{
		BacktestResultResponse: model.BacktestResultResponse{
			ID:              0,
			StrategyID:      strat.ID,
			StrategyName:    strat.Name,
			StrategyType:    strat.StrategyType,
			Underlying:      strat.Underlying,
			StartDate:       sd,
			EndDate:         ed,
			TotalReturn:     &tr,
			MaxDrawdown:     &mr,
			SharpeRatio:     &sr,
			WinRate:         &wr,
			NumTransactions: nt,
			FinalOutcome:    &fo,
			FinalAction:     fa,
			Cached:          cached,
			GeneratedAt:     now(),
		},
	}
}
