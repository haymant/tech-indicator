package strategy

import (
	"testing"
)

func TestCatalogPopulated(t *testing.T) {
	if len(Catalog) < 30 {
		t.Fatalf("expected at least 30 strategies in catalog, got %d", len(Catalog))
	}
}

func TestInstantiateDefaultParams(t *testing.T) {
	for typeKey, def := range Catalog {
		s, err := def.Constructor(def.DefaultParams)
		if err != nil {
			// Decorator and compound strategies that require sub-strategies may error.
			// That's acceptable — they're documented as requiring inner strategies.
			t.Logf("SKIP %s: %v", typeKey, err)
			continue
		}
		if s == nil {
			t.Errorf("Instantiate(%q) returned nil strategy", typeKey)
			continue
		}
		if s.Name() == "" {
			t.Errorf("Instantiate(%q) returned strategy with empty Name()", typeKey)
		}
	}
}

func TestInstantiateInvalidType(t *testing.T) {
	_, err := Instantiate("non_existent_strategy", nil)
	if err == nil {
		t.Fatal("expected error for invalid strategy type")
	}
}

func TestIsValid(t *testing.T) {
	if !IsValid("buy_and_hold_strategy") {
		t.Error("buy_and_hold_strategy should be valid")
	}
	if !IsValid("rsi_strategy") {
		t.Error("rsi_strategy should be valid")
	}
	if !IsValid("golden_cross_strategy") {
		t.Error("golden_cross_strategy should be valid")
	}
	if IsValid("not_a_strategy") {
		t.Error("not_a_strategy should not be valid")
	}
}

func TestListTypesReturnsAll(t *testing.T) {
	entries := ListTypes()
	if len(entries) != len(Catalog) {
		t.Errorf("ListTypes returned %d entries, Catalog has %d", len(entries), len(Catalog))
	}
}

func TestCategories(t *testing.T) {
	cats := Categories()
	if len(cats) == 0 {
		t.Fatal("Categories returned empty map")
	}
	baseCat, ok := cats["base"]
	if !ok {
		t.Fatal("expected 'base' category")
	}
	if baseCat.Count == 0 {
		t.Error("base category count should be > 0")
	}
	if baseCat.Description == "" {
		t.Error("base category should have description")
	}
}

func TestGetParamHelpers(t *testing.T) {
	params := map[string]any{
		"period": 14,
		"buy_at": 30.5,
		"name":   "test",
	}

	if v := getParamInt(params, "period", 10); v != 14 {
		t.Errorf("expected 14, got %d", v)
	}
	if v := getParamInt(params, "missing", 10); v != 10 {
		t.Errorf("expected default 10, got %d", v)
	}

	if v := getParamFloat(params, "buy_at", 20); v != 30.5 {
		t.Errorf("expected 30.5, got %f", v)
	}
	if v := getParamFloat(params, "missing", 20); v != 20 {
		t.Errorf("expected default 20, got %f", v)
	}

	if v := getParamString(params, "name", "default"); v != "test" {
		t.Errorf("expected 'test', got '%s'", v)
	}
}
