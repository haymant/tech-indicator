package model

// ─── Backtest (FR-05) ──────────────────────────────────────────────────────

// BacktestRunRequest is the optional JSON body for POST /api/strategies/{id}/backtest.
type BacktestRunRequest struct {
	Force bool `json:"force"`
}

// BacktestResultResponse is a single backtest performance/risk evaluation.
type BacktestResultResponse struct {
	ID              int            `json:"id"`
	StrategyID      int            `json:"strategy_id"`
	StrategyName    string         `json:"strategy_name,omitempty"`
	StrategyType    string         `json:"strategy_type"`
	Underlying      string         `json:"underlying"`
	StartDate       string         `json:"start_date"`
	EndDate         string         `json:"end_date"`
	TotalReturn     *float64       `json:"total_return"`
	MaxDrawdown     *float64       `json:"max_drawdown"`
	SharpeRatio     *float64       `json:"sharpe_ratio"`
	WinRate         *float64       `json:"win_rate"`
	NumTransactions int            `json:"num_transactions"`
	FinalOutcome    *float64       `json:"final_outcome"`
	FinalAction     string         `json:"final_action"`
	Parameters      map[string]any `json:"parameters,omitempty"`
	Cached          bool           `json:"cached"`
	GeneratedAt     string         `json:"generated_at"`
}

// BacktestRunResponse is returned after running a backtest.
type BacktestRunResponse struct {
	BacktestResultResponse
}

// BacktestResultsQueryResponse is the response for GET /api/backtest-results.
type BacktestResultsQueryResponse struct {
	Results []BacktestResultResponse `json:"results"`
	Count   int                      `json:"count"`
}
