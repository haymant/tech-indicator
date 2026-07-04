package handler

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"vercel-go-starter/internal/model"
	"vercel-go-starter/internal/signal"
)

// handleQuerySignals handles GET /api/signals
func (h *Handler) handleQuerySignals(w http.ResponseWriter, r *http.Request) {
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

	filter := signal.SignalFilter{
		StrategyType: q.Get("strategy_type"),
		Underlying:   q.Get("underlying"),
		DateFrom:     q.Get("date_from"),
		DateTo:       q.Get("date_to"),
		Action:       q.Get("action"),
	}

	if sid := q.Get("strategy_id"); sid != "" {
		filter.StrategyID, _ = strconv.Atoi(sid)
	}
	if limit := q.Get("limit"); limit != "" {
		filter.Limit, _ = strconv.Atoi(limit)
	}
	if offset := q.Get("offset"); offset != "" {
		filter.Offset, _ = strconv.Atoi(offset)
	}

	repo := signal.NewRepository(dbURL)
	records, total, err := repo.GetSignals(r.Context(), filter)
	if err != nil {
		slog.Error("Failed to query signals", "error", err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Status: "error", Message: "Query failed", Timestamp: now()})
		return
	}

	resp := model.SignalQueryResponse{
		Signals: toModelSignalRecords(records),
		Count:   len(records),
		Total:   total,
	}
	if resp.Signals == nil {
		resp.Signals = []model.SignalRecord{}
	}

	writeJSON(w, http.StatusOK, resp)
}
