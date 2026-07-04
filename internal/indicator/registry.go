package indicator

// IndicatorDef describes a single technical indicator in the registry.
type IndicatorDef struct {
	Key           string         `json:"name"`                     // "rsi_14"
	Category      string         `json:"category"`                 // "momentum"
	DisplayName   string         `json:"display_name"`             // "Relative Strength Index"
	Description   string         `json:"description"`              // Human-readable
	WhenToUse     string         `json:"when_to_use"`              // Usage guidance
	Inputs        []string       `json:"inputs"`                   // ["close"]
	Outputs       int            `json:"outputs"`                  // number of output channels
	SubIndicators []string       `json:"sub_indicators,omitempty"` // for multi-output
	DefaultParams map[string]int `json:"default_parameters"`       // {"period": 14}
}

// Registry returns the global catalog of all supported indicators.
var Registry = map[string]IndicatorDef{}
