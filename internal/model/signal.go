package model

// ─── Signal Generation (FR-04) ─────────────────────────────────────────────

// SignalGenerateRequest is the optional JSON body for POST /api/strategies/{id}/signals.
type SignalGenerateRequest struct {
	Force bool `json:"force"`
}

// SignalRecord is a single buy/sell/hold signal.
type SignalRecord struct {
	ID           int     `json:"id,omitempty"`
	StrategyID   int     `json:"strategy_id"`
	StrategyType string  `json:"strategy_type"`
	StrategyName string  `json:"strategy_name,omitempty"`
	Underlying   string  `json:"underlying"`
	SignalDate   string  `json:"signal_date"`
	Action       string  `json:"action"`
	Price        float64 `json:"price"`
}

// SignalGenerateResponse is returned after signal generation.
type SignalGenerateResponse struct {
	StrategyID   int            `json:"strategy_id"`
	StrategyName string         `json:"strategy_name"`
	Underlying   string         `json:"underlying"`
	SignalCount  int            `json:"signal_count"`
	Signals      []SignalRecord `json:"signals"`
	Cached       bool           `json:"cached"`
	GeneratedAt  string         `json:"generated_at"`
}

// SignalQueryResponse is the response for GET /api/signals.
type SignalQueryResponse struct {
	Signals []SignalRecord `json:"signals"`
	Count   int            `json:"count"`
	Total   int            `json:"total"`
}
