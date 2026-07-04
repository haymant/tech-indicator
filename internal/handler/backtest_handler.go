package handler

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"vercel-go-starter/internal/backtest"
	"vercel-go-starter/internal/model"
)

// handleQueryBacktestResults handles GET /api/backtest-results
func (h *Handler) handleQueryBacktestResults(w http.ResponseWriter, r *http.Request) {
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

	filter := backtest.BacktestFilter{
		StrategyType: q.Get("strategy_type"),
		Underlying:   q.Get("underlying"),
		DateFrom:     q.Get("date_from"),
		DateTo:       q.Get("date_to"),
	}

	if sid := q.Get("strategy_id"); sid != "" {
		filter.StrategyID, _ = strconv.Atoi(sid)
	}
	if mr := q.Get("min_return"); mr != "" {
		filter.MinReturn, _ = strconv.ParseFloat(mr, 64)
	}
	if limit := q.Get("limit"); limit != "" {
		filter.Limit, _ = strconv.Atoi(limit)
	}
	if offset := q.Get("offset"); offset != "" {
		filter.Offset, _ = strconv.Atoi(offset)
	}

	repo := backtest.NewRepository(dbURL)
	results, total, err := repo.GetResults(r.Context(), filter)
	if err != nil {
		slog.Error("Failed to query backtest results", "error", err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Query failed", Timestamp: now()})
		return
	}

	resp := model.BacktestResultsQueryResponse{
		Results: toModelBacktestResults(results),
		Count:   total,
	}
	if resp.Results == nil {
		resp.Results = []model.BacktestResultResponse{}
	}

	writeJSON(w, http.StatusOK, resp)
}

func toModelBacktestResults(results []backtest.Result) []model.BacktestResultResponse {
	out := make([]model.BacktestResultResponse, 0, len(results))
	for _, r := range results {
		tr, mr, sr, wr, fo := r.TotalReturn, r.MaxDrawdown, r.SharpeRatio, r.WinRate, r.FinalOutcome
		resp := model.BacktestResultResponse{
			StrategyID:      r.StrategyID,
			StrategyType:    r.StrategyType,
			Underlying:      r.Underlying,
			StartDate:       r.StartDate.Format("2006-01-02"),
			EndDate:         r.EndDate.Format("2006-01-02"),
			TotalReturn:     &tr,
			MaxDrawdown:     &mr,
			SharpeRatio:     &sr,
			WinRate:         &wr,
			NumTransactions: r.NumTransactions,
			FinalOutcome:    &fo,
			FinalAction:     r.FinalAction,
		}
		out = append(out, resp)
	}
	return out
}
