package model

// SyncRequest is the optional JSON body for POST /api/sync.
type SyncRequest struct {
	Assets  []string `json:"assets,omitempty"`
	Days    int      `json:"days,omitempty"`
	Workers int      `json:"workers,omitempty"`
	Delay   int      `json:"delay,omitempty"`
	Force   bool     `json:"force,omitempty"`
}

// SyncResponse is returned on 202 Accepted after starting a sync.
type SyncResponse struct {
	Status    string   `json:"status"`
	Message   string   `json:"message"`
	Assets    []string `json:"assets,omitempty"`
	Days      int      `json:"days,omitempty"`
	Workers   int      `json:"workers,omitempty"`
	Timestamp string   `json:"timestamp"`
}

// ErrorResponse is returned on 4xx and 5xx errors.
type ErrorResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// ─── Indicator Calculation ─────────────────────────────────────────────────

// IndicatorCalculateRequest is the JSON body for POST /api/indicators/calculate.
type IndicatorCalculateRequest struct {
	Assets     []string `json:"assets,omitempty"`
	Indicators []string `json:"indicators,omitempty"`
	Days       int      `json:"days,omitempty"`
	Force      bool     `json:"force,omitempty"`
}

// IndicatorCalculateResponse is returned after calculation completes.
type IndicatorCalculateResponse struct {
	Status     string   `json:"status"`
	Message    string   `json:"message"`
	Assets     []string `json:"assets,omitempty"`
	Indicators int      `json:"indicators"`
	AssetCount int      `json:"asset_count"`
	Timestamp  string   `json:"timestamp"`
}

// IndicatorCatalogResponse is the response for GET /api/indicators.
type IndicatorCatalogResponse struct {
	Indicators []IndicatorEntry           `json:"indicators"`
	Count      int                        `json:"count"`
	Categories map[string]CatalogCategory `json:"categories"`
	Timestamp  string                     `json:"timestamp"`
}

// IndicatorEntry describes a single indicator in the catalog.
type IndicatorEntry struct {
	Name          string         `json:"name"`
	Category      string         `json:"category"`
	DisplayName   string         `json:"display_name"`
	Description   string         `json:"description"`
	WhenToUse     string         `json:"when_to_use"`
	Inputs        []string       `json:"inputs"`
	Outputs       int            `json:"outputs"`
	SubIndicators []string       `json:"sub_indicators,omitempty"`
	DefaultParams map[string]int `json:"default_parameters"`
}

// CatalogCategory describes a category of indicators.
type CatalogCategory struct {
	Count       int    `json:"count"`
	Description string `json:"description"`
}
