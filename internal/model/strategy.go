package model

// ─── Strategy Types (FR-01) ────────────────────────────────────────────────

// StrategyTypeEntry describes a single strategy type from the library catalog.
type StrategyTypeEntry struct {
	Type          string         `json:"type"`
	Name          string         `json:"name"`
	Category      string         `json:"category"`
	Description   string         `json:"description"`
	DefaultParams map[string]any `json:"default_parameters"`
}

// CategorySummary groups strategy types by category.
type CategorySummary struct {
	Count       int    `json:"count"`
	Description string `json:"description"`
}

// StrategyTypesResponse is the response for GET /api/strategies/types.
type StrategyTypesResponse struct {
	Strategies []StrategyTypeEntry        `json:"strategies"`
	Count      int                        `json:"count"`
	Categories map[string]CategorySummary `json:"categories"`
}

// ─── Strategy CRUD (FR-02, FR-03) ──────────────────────────────────────────

// StrategyCreateRequest is the JSON body for POST /api/strategies.
type StrategyCreateRequest struct {
	Name         string         `json:"name"`
	StrategyType string         `json:"strategy_type"`
	Underlying   string         `json:"underlying"`
	Timeframe    string         `json:"timeframe"`
	LookbackDays int            `json:"lookback_days"`
	Parameters   map[string]any `json:"parameters"`
	Force        bool           `json:"force,omitempty"`
}

// StrategyResponse is a single strategy returned by the API.
type StrategyResponse struct {
	ID           int            `json:"id"`
	Name         string         `json:"name"`
	StrategyType string         `json:"strategy_type"`
	Underlying   string         `json:"underlying"`
	Timeframe    string         `json:"timeframe"`
	LookbackDays int            `json:"lookback_days"`
	Parameters   map[string]any `json:"parameters"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
}

// StrategyListResponse is the response for GET /api/strategies.
type StrategyListResponse struct {
	Strategies []StrategyResponse `json:"strategies"`
	Count      int                `json:"count"`
}
