package engine

import (
	"context"
	"math"
	"testing"
	"time"
)

// makeTestInput creates n mock OHLCV data points with gradually increasing prices.
func makeTestInput(n int) *Input {
	now := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	in := &Input{
		Dates:  make([]time.Time, n),
		Open:   make([]float64, n),
		High:   make([]float64, n),
		Low:    make([]float64, n),
		Close:  make([]float64, n),
		Volume: make([]float64, n),
	}
	for i := 0; i < n; i++ {
		in.Dates[i] = now.AddDate(0, 0, i-n+1)
		in.Open[i] = 100 + float64(i)
		in.High[i] = 105 + float64(i)
		in.Low[i] = 95 + float64(i)
		in.Close[i] = 100 + float64(i)
		in.Volume[i] = 1000000
	}
	return in
}

// checkResult verifies an indicator produced non-empty values.
func checkResult(t *testing.T, results map[string][]Result, key string, minValues int) {
	t.Helper()
	res, ok := results[key]
	if !ok {
		t.Fatalf("%s not found in results", key)
	}
	if len(res) == 0 {
		t.Fatalf("%s result list is empty", key)
	}
	if len(res[0].Values) < minValues {
		t.Errorf("%s expected at least %d values, got %d", key, minValues, len(res[0].Values))
	}
}

// checkSubIndicators verifies the number of sub-indicators.
func checkSubIndicators(t *testing.T, results map[string][]Result, key string, expected int) {
	t.Helper()
	res, ok := results[key]
	if !ok {
		t.Fatalf("%s not found in results", key)
	}
	if len(res) != expected {
		t.Errorf("%s expected %d sub-indicators, got %d", key, expected, len(res))
	}
}

// checkAllIndicators runs a list of indicators and verifies they all produce results.
func checkAllIndicators(t *testing.T, input *Input, keys []string) {
	t.Helper()
	results := ComputeIndicators(context.Background(), input, keys)
	if results == nil {
		t.Fatal("ComputeIndicators returned nil")
	}
	for _, key := range keys {
		checkResult(t, results, key, 1)
	}
}

func TestComputeRSI(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"rsi_14"})
	checkResult(t, results, "rsi_14", 1)
}

func TestComputeSMA(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(60), []string{"sma_20", "sma_50"})
	checkResult(t, results, "sma_20", 1)
	checkResult(t, results, "sma_50", 1)
}

func TestComputeEMA(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"ema_20"})
	checkResult(t, results, "ema_20", 1)
}

func TestComputeMACD(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(60), []string{"macd_12_26_9"})
	checkResult(t, results, "macd_12_26_9", 1)
	checkSubIndicators(t, results, "macd_12_26_9", 3)
}

func TestComputeBollingerBands(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(40), []string{"bb_20_2"})
	checkResult(t, results, "bb_20_2", 1)
	checkSubIndicators(t, results, "bb_20_2", 3)
}

func TestComputeATR(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"atr_14"})
	checkResult(t, results, "atr_14", 1)
}

func TestComputeTrueRange(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"tr"})
	checkResult(t, results, "tr", 1)
}

func TestComputeOBV(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"obv"})
	checkResult(t, results, "obv", 1)
}

func TestComputeAD(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"ad"})
	checkResult(t, results, "ad", 1)
}

func TestComputeStochastic(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"stoch_14_3"})
	checkResult(t, results, "stoch_14_3", 1)
	checkSubIndicators(t, results, "stoch_14_3", 2)
}

func TestComputeVWMA(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"vwma_20"})
	checkResult(t, results, "vwma_20", 1)
}

func TestComputeAPO(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(40), []string{"apo_14_30"})
	checkResult(t, results, "apo_14_30", 1)
}

func TestComputeROC(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"roc_9"})
	checkResult(t, results, "roc_9", 1)
}

func TestComputeWilliamsR(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"williams_r_14"})
	checkResult(t, results, "williams_r_14", 1)
}

func TestComputeAwesomeOscillator(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(40), []string{"awesome_oscillator"})
	checkResult(t, results, "awesome_oscillator", 1)
}

func TestComputeIBS(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"ibs"})
	checkResult(t, results, "ibs", 1)
}

func TestComputeEmptyInput(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(0), []string{"sma_20"})
	if results != nil {
		t.Error("expected nil for empty input")
	}
}

func TestComputeNilInput(t *testing.T) {
	results := ComputeIndicators(context.Background(), nil, []string{"sma_20"})
	if results != nil {
		t.Error("expected nil for nil input")
	}
}

func TestComputeUnknownIndicator(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"unknown_99"})
	if results == nil {
		t.Error("expected empty map, not nil")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for unknown indicator, got %d", len(results))
	}
}

func TestComputeAllRegisteredIndicators(t *testing.T) {
	input := makeTestInput(200)
	// Test all registered indicator names with default parameters
	keys := []string{
		// Trend
		"sma_20", "sma_50", "ema_20", "macd_12_26_9", "vwma_20",
		"apo_14_30", "roc_9", "aroon_25", "bop", "cci_20",
		"cfo_14", "dema_20", "dpo_20", "envelope_20_5",
		"hma_20", "kama_30", "kdj_9", "mass_index_25",
		"mcginley_20", "mlr_20", "mls_20", "moving_max_20",
		"moving_min_20", "moving_sum_20", "pivot_point",
		"rma_20", "slow_stochastic_14_3", "smma_20",
		"t3_20", "tema_20", "trima_20", "trix_20",
		"tsi", "typical_price", "weighted_close", "wma_20",
		// Momentum
		"rsi_14", "stoch_14_3", "williams_r_14",
		"awesome_oscillator", "ibs", "chaikin_oscillator_3_10",
		"connors_rsi_14", "coppock_curve", "elder_ray_13",
		"fisher_9", "ichimoku_cloud", "ppo_12_26_9",
		"prings_special_k", "pvo_12_26_9", "qstick_14",
		"rvi_14", "stochastic_rsi_14_3_3", "td_sequential",
		"ultimate_oscillator",
		// Volatility
		"bb_20_2", "atr_14", "tr", "acceleration_bands_20_4",
		"annualized_historical_volatility_20", "bollinger_band_width_20",
		"chandelier_exit_22_3", "chop_14", "donchian_channel_20",
		"historical_volatility_20", "keltner_channel_20_2",
		"moving_std_20", "percent_b_20", "po_14",
		"super_trend_10_3", "ulcer_index_14", "z_score_20",
		// Volume
		"obv", "ad", "cmf_20", "emv_14", "fi_13",
		"kvo_34_55", "mfi_14", "mfm", "mfv", "nvi",
		"vpt", "vwap",
		// Valuation
		"fv", "npv", "pv",
	}

	results := ComputeIndicators(context.Background(), input, keys)
	if results == nil {
		t.Fatal("ComputeIndicators returned nil")
	}

	t.Logf("Total results: %d/%d", len(results), len(keys))

	var missing []string
	for _, key := range keys {
		if _, ok := results[key]; !ok {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		t.Errorf("Missing results for %d indicators: %v", len(missing), missing)
	}

	// Verify each result has values
	for _, key := range keys {
		res, ok := results[key]
		if !ok {
			continue
		}
		for i, r := range res {
			if len(r.Values) == 0 {
				t.Errorf("%s sub-indicator %d has empty values", key, i)
			}
		}
	}

	t.Logf("All %d registered base indicators produce non-empty results", len(keys))
}

// Test helper functions
func TestLookupIndicator(t *testing.T) {
	tests := []struct {
		key    string
		name   string
		params []int
	}{
		{"rsi_14", "rsi", []int{14}},
		{"sma_20", "sma", []int{20}},
		{"macd_12_26_9", "macd", []int{12, 26, 9}},
		{"bb_20_2", "bb", []int{20, 2}},
		{"obv", "obv", nil},
		{"tr", "tr", nil},
		{"williams_r_14", "williams_r", []int{14}},
		{"awesome_oscillator", "awesome_oscillator", nil},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			fn, params := lookupIndicator(tc.key)
			if fn == nil {
				t.Fatalf("lookupIndicator(%q) returned nil", tc.key)
			}
			if len(params) != len(tc.params) {
				t.Errorf("expected %d params, got %d", len(tc.params), len(params))
			}
			for i, p := range params {
				if i < len(tc.params) && p != tc.params[i] {
					t.Errorf("param %d: expected %d, got %d", i, tc.params[i], p)
				}
			}
		})
	}
}

func TestComputeBOP(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"bop"})
	checkResult(t, results, "bop", 1)
}

func TestComputeCCI(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"cci_20"})
	checkResult(t, results, "cci_20", 1)
}

func TestComputeAroon(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(40), []string{"aroon_25"})
	checkResult(t, results, "aroon_25", 1)
	checkSubIndicators(t, results, "aroon_25", 2)
}

func TestComputeDEMA(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"dema_20"})
	checkResult(t, results, "dema_20", 1)
}

func TestComputeTEMA(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"tema_20"})
	checkResult(t, results, "tema_20", 1)
}

func TestComputeKAMA(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(40), []string{"kama_30"})
	checkResult(t, results, "kama_30", 1)
}

func TestComputeKDJ(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"kdj_9"})
	checkResult(t, results, "kdj_9", 1)
	checkSubIndicators(t, results, "kdj_9", 3)
}

func TestComputeMFI(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"mfi_14"})
	checkResult(t, results, "mfi_14", 1)
}

func TestComputeStochasticRSI(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(50), []string{"stochastic_rsi_14_3_3"})
	checkResult(t, results, "stochastic_rsi_14_3_3", 1)
	checkSubIndicators(t, results, "stochastic_rsi_14_3_3", 2)
}

func TestComputeConnorsRSI(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"connors_rsi_14"})
	checkResult(t, results, "connors_rsi_14", 1)
}

func TestComputeIchimoku(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(60), []string{"ichimoku_cloud"})
	checkResult(t, results, "ichimoku_cloud", 1)
}

func TestComputePPO(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(40), []string{"ppo_12_26_9"})
	checkResult(t, results, "ppo_12_26_9", 1)
	checkSubIndicators(t, results, "ppo_12_26_9", 3)
}

func TestComputeSuperTrend(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"super_trend_10_3"})
	checkResult(t, results, "super_trend_10_3", 1)
}

func TestComputeDonchian(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"donchian_channel_20"})
	checkResult(t, results, "donchian_channel_20", 1)
	checkSubIndicators(t, results, "donchian_channel_20", 3)
}

func TestComputeKeltner(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"keltner_channel_20_2"})
	checkResult(t, results, "keltner_channel_20_2", 1)
	checkSubIndicators(t, results, "keltner_channel_20_2", 3)
}

func TestComputeUlcerIndex(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"ulcer_index_14"})
	checkResult(t, results, "ulcer_index_14", 1)
}

func TestComputeVWAP(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"vwap"})
	checkResult(t, results, "vwap", 1)
}

func TestComputeCMF(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"cmf_20"})
	checkResult(t, results, "cmf_20", 1)
}

func TestComputeNVI(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"nvi"})
	checkResult(t, results, "nvi", 1)
}

func TestComputeVPT(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"vpt"})
	checkResult(t, results, "vpt", 1)
}

func TestComputePivotPoint(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"pivot_point"})
	checkResult(t, results, "pivot_point", 1)
	checkSubIndicators(t, results, "pivot_point", 5)
}

func TestComputeTRIX(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"trix_20"})
	checkResult(t, results, "trix_20", 1)
}

func TestComputeTSI(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(60), []string{"tsi"})
	checkResult(t, results, "tsi", 1)
}

func TestComputeHMA(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"hma_20"})
	checkResult(t, results, "hma_20", 1)
}

func TestComputeWMA(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"wma_20"})
	checkResult(t, results, "wma_20", 1)
}

func TestComputeTrima(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"trima_20"})
	checkResult(t, results, "trima_20", 1)
}

func TestComputeContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	input := makeTestInput(100)
	results := ComputeIndicators(ctx, input, []string{"sma_20", "rsi_14", "ema_20"})
	// With immediate cancellation, results may be partial or empty
	_ = results
}

func TestComputeFisher(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"fisher_9"})
	checkResult(t, results, "fisher_9", 1)
}

func TestComputeChaikinOscillator(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"chaikin_oscillator_3_10"})
	checkResult(t, results, "chaikin_oscillator_3_10", 1)
}

func TestComputeCoppock(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"coppock_curve"})
	checkResult(t, results, "coppock_curve", 1)
}

func TestComputeKST(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(60), []string{"kst"})
	checkResult(t, results, "kst", 1)
}

func TestComputeKVO(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(70), []string{"kvo_34_55"})
	checkResult(t, results, "kvo_34_55", 1)
}

func TestComputeChandelierExit(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"chandelier_exit_22_3"})
	checkResult(t, results, "chandelier_exit_22_3", 1)
	checkSubIndicators(t, results, "chandelier_exit_22_3", 2)
}

func TestComputeEnvelope(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"envelope_20_5"})
	checkResult(t, results, "envelope_20_5", 1)
	checkSubIndicators(t, results, "envelope_20_5", 3)
}

func TestComputeAccelerationBands(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(30), []string{"acceleration_bands_20_4"})
	checkResult(t, results, "acceleration_bands_20_4", 1)
	checkSubIndicators(t, results, "acceleration_bands_20_4", 3)
}

func TestComputeElderRay(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeTestInput(20), []string{"elder_ray_13"})
	checkResult(t, results, "elder_ray_13", 1)
	checkSubIndicators(t, results, "elder_ray_13", 2)
}

func TestComputeSMAValues(t *testing.T) {
	// Test SMA produces expected values
	input := makeTestInput(100)
	result := computeSMASlice(input.Close, 20)

	// First 19 values should be 0
	for i := 0; i < 19; i++ {
		if result[i] != 0 {
			t.Errorf("expected 0 at position %d, got %f", i, result[i])
		}
	}

	// Value at index 19 should be SMA of first 20 values
	expected := 0.0
	for i := 0; i < 20; i++ {
		expected += 100 + float64(i)
	}
	expected /= 20.0

	if diff := math.Abs(result[19] - expected); diff > 1e-9 {
		t.Errorf("expected %f, got %f (diff=%e)", expected, result[19], diff)
	}
}

func TestComputeEMAMatchesExpected(t *testing.T) {
	input := makeTestInput(30)
	ema := computeEMASlice(input.Close, 20)

	// EMA should have 0 for first 19, non-zero at index 19+
	if ema[19] == 0 {
		t.Error("expected non-zero EMA at index 19")
	}
	for i := 20; i < len(ema); i++ {
		if ema[i] == 0 {
			t.Errorf("unexpected zero EMA at %d", i)
		}
	}
}

func TestComputeSMAConverges(t *testing.T) {
	// With constant values, SMA should converge to that value
	n := 100
	data := make([]float64, n)
	for i := 0; i < n; i++ {
		data[i] = 150.0
	}

	sma := computeSMASlice(data, 20)
	for i := 19; i < n; i++ {
		if sma[i] != 150.0 {
			t.Errorf("expected 150 at position %d, got %f", i, sma[i])
		}
	}
}

func TestComputeEMAConverges(t *testing.T) {
	n := 100
	data := make([]float64, n)
	for i := 0; i < n; i++ {
		data[i] = 200.0
	}

	ema := computeEMASlice(data, 20)
	// After enough periods, EMA should converge to 200
	lastVal := ema[n-1]
	if diff := math.Abs(lastVal - 200.0); diff > 0.01 {
		t.Errorf("EMA did not converge to 200, got %f (diff=%e)", lastVal, diff)
	}
}

func TestComputeRSIKnownValues(t *testing.T) {
	// Create a monotonically increasing price series: RSI should be high
	input := makeTestInput(30)
	results := ComputeIndicators(context.Background(), input, []string{"rsi_14"})
	rsi := results["rsi_14"][0].Values

	// With consistently increasing prices, RSI should be high (>50)
	lastRsi := rsi[len(rsi)-1]
	if lastRsi < 50 {
		t.Errorf("expected RSI > 50 for uptrend, got %f", lastRsi)
	}
}

func TestComputeZeroVolumeIndicators(t *testing.T) {
	// Create input with zero volume
	input := makeTestInput(20)
	for i := range input.Volume {
		input.Volume[i] = 0
	}

	results := ComputeIndicators(context.Background(), input, []string{"obv", "ad", "vwap", "cmf_20"})
	for _, key := range []string{"obv", "ad", "vwap", "cmf_20"} {
		checkResult(t, results, key, 1)
	}
}

func TestSliceHelpers(t *testing.T) {
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{5, 4, 3, 2, 1}

	if r := add(a, b); r[0] != 6 || r[4] != 6 {
		t.Errorf("add failed: %v", r)
	}
	if r := subtract(a, b); r[0] != -4 || r[4] != 4 {
		t.Errorf("subtract failed: %v", r)
	}
	if r := multiply(a, b); r[0] != 5 || r[4] != 5 {
		t.Errorf("multiply failed: %v", r)
	}
	if r := divide(a, b); r[0] != 0.2 || r[4] != 5 {
		t.Errorf("divide failed: %v", r)
	}
	if r := incrementBy(a, 10); r[0] != 11 || r[4] != 15 {
		t.Errorf("incrementBy failed: %v", r)
	}
	if r := absSlice(subtract(a, b)); r[0] != 4 {
		t.Errorf("absSlice failed: %v", r)
	}
	if r := multiplyBy(a, 2); r[0] != 2 || r[4] != 10 {
		t.Errorf("multiplyBy failed: %v", r)
	}
	if r := keepPositives([]float64{-1, 0, 1, -2, 3}); r[0] != 0 || r[2] != 1 || r[4] != 3 {
		t.Errorf("keepPositives failed: %v", r)
	}
	if r := keepNegatives([]float64{-1, 0, 1, -2, 3}); r[0] != 1 || r[2] != 0 || r[3] != 2 {
		t.Errorf("keepNegatives failed: %v", r)
	}
}

func TestComputeMaxMinSum(t *testing.T) {
	data := []float64{3, 1, 4, 1, 5, 9, 2, 6, 5, 3}

	max3 := computeMovingMax(data, 3)
	if max3[2] != 4 || max3[5] != 9 {
		t.Errorf("movingMax failed: %v", max3)
	}

	min3 := computeMovingMin(data, 3)
	if min3[2] != 1 || min3[5] != 1 {
		t.Errorf("movingMin failed: %v", min3)
	}

	sum3 := computeMovingSum(data, 3)
	if sum3[2] != 8 || sum3[5] != 15 {
		t.Errorf("movingSum failed: %v", sum3)
	}
}

func TestComputeStd(t *testing.T) {
	// With constant data, std should be 0
	n := 30
	data := make([]float64, n)
	for i := 0; i < n; i++ {
		data[i] = 100.0
	}

	std := computeMovingStd(data, 5)
	for i := 4; i < n; i++ {
		if std[i] != 0 {
			t.Errorf("expected std=0 for constant data at %d, got %f", i, std[i])
		}
	}
}
