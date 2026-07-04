package indicator

import (
	"context"
	"testing"
	"time"

	"github.com/cinar/indicator/v2/asset"
)

// makeSnapshots creates n mock snapshots with gradually increasing prices.
func makeSnapshots(n int) []*asset.Snapshot {
	now := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	snapshots := make([]*asset.Snapshot, n)
	for i := range snapshots {
		snapshots[i] = &asset.Snapshot{
			Date:   now.AddDate(0, 0, i-n+1),
			Open:   100 + float64(i),
			High:   105 + float64(i),
			Low:    95 + float64(i),
			Close:  100 + float64(i),
			Volume: 1000000,
		}
	}
	return snapshots
}

// checkIndicator is a helper that verifies an indicator produced non-empty values.
func checkIndicator(t *testing.T, results map[string][]IndicatorResult, key string, minValues int) {
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

func TestComputeRSI(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeSnapshots(30), []string{"rsi_14"})
	checkIndicator(t, results, "rsi_14", 1)
}

func TestComputeSMA(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeSnapshots(60), []string{"sma_20", "sma_50"})
	checkIndicator(t, results, "sma_20", 1)
	checkIndicator(t, results, "sma_50", 1)
}

func TestComputeEMA(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeSnapshots(30), []string{"ema_20"})
	checkIndicator(t, results, "ema_20", 1)
}

func TestComputeMACD(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeSnapshots(60), []string{"macd_12_26_9"})
	checkIndicator(t, results, "macd_12_26_9", 1)
	// MACD has 3 sub-indicators: line, signal, histogram
	if len(results["macd_12_26_9"]) != 3 {
		t.Errorf("macd expected 3 sub-indicators, got %d", len(results["macd_12_26_9"]))
	}
}

func TestComputeBollingerBands(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeSnapshots(40), []string{"bb_20_2"})
	checkIndicator(t, results, "bb_20_2", 1)
	if len(results["bb_20_2"]) != 3 {
		t.Errorf("bb expected 3 sub-indicators, got %d", len(results["bb_20_2"]))
	}
}

func TestComputeATR(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeSnapshots(30), []string{"atr_14"})
	checkIndicator(t, results, "atr_14", 1)
}

func TestComputeOBV(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeSnapshots(20), []string{"obv"})
	checkIndicator(t, results, "obv", 1)
}

func TestComputeStochastic(t *testing.T) {
	results := ComputeIndicators(context.Background(), makeSnapshots(30), []string{"stoch_14_3"})
	checkIndicator(t, results, "stoch_14_3", 1)
	if len(results["stoch_14_3"]) != 2 {
		t.Errorf("stoch expected 2 sub-indicators, got %d", len(results["stoch_14_3"]))
	}
}

func TestComputeAllIndicators(t *testing.T) {
	// Test the exact list that was hanging.
	keys := []string{"rsi_14", "sma_20", "ema_20", "macd_12_26_9", "bb_20_2", "atr_14", "obv", "stoch_14_3"}
	results := ComputeIndicators(context.Background(), makeSnapshots(100), keys)
	for _, key := range keys {
		checkIndicator(t, results, key, 1)
	}
}
