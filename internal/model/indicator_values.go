package model

// IndicatorValuesResponse is the response for GET /api/indicators/values.
type IndicatorValuesResponse struct {
	Symbols    []string                          `json:"symbols"`
	Indicators []string                          `json:"indicators"`
	Data       map[string]map[string][]DataPoint `json:"data"` // symbol → indicator → points
	Total      int                               `json:"total"`
	Timestamp  string                            `json:"timestamp"`
}

// DataPoint is a single (date, value) pair.
type DataPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}
