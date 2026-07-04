package strategy

import (
	"fmt"
	"sort"

	"vercel-go-starter/internal/model"

	cindicator "github.com/cinar/indicator/v2/strategy"
)

// Instantiate creates a strategy.Strategy from a type key and parameters map.
func Instantiate(strategyType string, params map[string]any) (cindicator.Strategy, error) {
	def, ok := Catalog[strategyType]
	if !ok {
		return nil, fmt.Errorf("unknown strategy type: %s", strategyType)
	}
	return def.Constructor(params)
}

// IsValid returns true if the strategy type is registered in the catalog.
func IsValid(strategyType string) bool {
	_, ok := Catalog[strategyType]
	return ok
}

// ListTypes returns all strategy types as response entries.
func ListTypes() []model.StrategyTypeEntry {
	keys := make([]string, 0, len(Catalog))
	for k := range Catalog {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	entries := make([]model.StrategyTypeEntry, 0, len(keys))
	for _, k := range keys {
		def := Catalog[k]
		entries = append(entries, model.StrategyTypeEntry{
			Type:          def.Type,
			Name:          def.Name,
			Category:      def.Category,
			Description:   def.Description,
			DefaultParams: def.DefaultParams,
		})
	}
	return entries
}

// Categories returns category summaries with counts.
func Categories() map[string]model.CategorySummary {
	counts := make(map[string]int)
	for _, def := range Catalog {
		counts[def.Category]++
	}

	result := make(map[string]model.CategorySummary)
	for cat, desc := range CategoryDescriptions {
		result[cat] = model.CategorySummary{
			Count:       counts[cat],
			Description: desc,
		}
	}
	return result
}
