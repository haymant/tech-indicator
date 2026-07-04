package engine

import (
	"context"
	"fmt"
	"math"
	"testing"
)

// benchmarkKeys is a subset of indicators used for comparison benchmarks.
// Full set would take too long for channel path.
var benchmarkKeys = []string{
	"sma_20", "ema_20", "rsi_14", "macd_12_26_9", "bb_20_2",
	"atr_14", "obv", "stoch_14_3", "vwma_20", "apo_14_30",
}

// allBenchmarkKeys is the full set for slice-only benchmarks.
var allBenchmarkKeys = func() []string {
	keys := make([]string, 0, 80)
	for _, k := range []string{
		"sma_20", "sma_50", "ema_20", "macd_12_26_9", "vwma_20",
		"apo_14_30", "roc_9", "aroon_25", "bop", "cci_20",
		"cfo_14", "dema_20", "dpo_20", "envelope_20_5",
		"hma_20", "kama_30", "kdj_9", "kst", "mass_index_25",
		"mcginley_20", "mlr_20", "mls_20", "moving_max_20",
		"moving_min_20", "moving_sum_20", "pivot_point",
		"rma_20", "slow_stochastic_14_3", "smma_20",
		"stc_23_2_5", "t3_20", "tema_20", "trima_20", "trix_20",
		"tsi", "typical_price", "weighted_close", "wma_20",
		"rsi_14", "stoch_14_3", "williams_r_14",
		"awesome_oscillator", "ibs", "chaikin_oscillator_3_10",
		"connors_rsi_14", "coppock_curve", "elder_ray_13",
		"fisher_9", "ichimoku_cloud", "ppo_12_26_9",
		"prings_special_k", "pvo_12_26_9", "qstick_14",
		"rvi_14", "stochastic_rsi_14_3_3", "td_sequential",
		"ultimate_oscillator",
		"bb_20_2", "atr_14", "tr", "acceleration_bands_20_4",
		"annualized_historical_volatility_20", "bollinger_band_width_20",
		"chandelier_exit_22_3", "chop_14", "donchian_channel_20",
		"historical_volatility_20", "keltner_channel_20_2",
		"moving_std_20", "percent_b_20", "po_14",
		"super_trend_10_3", "ulcer_index_14", "z_score_20",
		"obv", "ad", "cmf_20", "emv_14", "fi_13",
		"kvo_34_55", "mfi_14", "mfm", "mfv", "nvi",
		"vpt", "vwap",
		"fv_10_5", "npv_10", "pv_10_5",
	} {
		keys = append(keys, k)
	}
	return keys
}()

// makeBenchInput creates random-like OHLCV data for benchmarking.
func makeBenchInput(n int) *Input {
	input := makeTestInput(n)
	// Add some variation for more realistic data
	for i := range input.Close {
		input.Close[i] = 100 + float64(i) + math.Sin(float64(i)*0.5)*10
		input.High[i] = input.Close[i] + 5 + math.Abs(math.Sin(float64(i)*0.3))*5
		input.Low[i] = input.Close[i] - 5 - math.Abs(math.Cos(float64(i)*0.3))*5
		input.Open[i] = input.Close[i] + math.Sin(float64(i)*0.7)*3
		input.Volume[i] = 1000000 + math.Abs(math.Sin(float64(i)*0.1))*500000
	}
	return input
}

// BenchmarkSliceEngine benchmarks the slice-based engine with all indicators.
func BenchmarkSliceEngine(b *testing.B) {
	sizes := []int{60, 252, 500, 1000}
	for _, size := range sizes {
		input := makeBenchInput(size)
		keys := allBenchmarkKeys
		b.Run(fmt.Sprintf("Slice_%d_indicators_%d_points", len(keys), size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ComputeIndicators(context.Background(), input, keys)
			}
		})
	}
}

// BenchmarkSliceSubset benchmarks the slice engine with a 10-indicator subset.
func BenchmarkSliceSubset(b *testing.B) {
	sizes := []int{60, 252, 500, 1000}
	for _, size := range sizes {
		input := makeBenchInput(size)
		b.Run(fmt.Sprintf("Slice_%d_indicators_%d_points", len(benchmarkKeys), size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ComputeIndicators(context.Background(), input, benchmarkKeys)
			}
		})
	}
}

// BenchmarkSMASlice benchmarks SMA computation directly.
func BenchmarkSMASlice(b *testing.B) {
	sizes := []int{60, 252, 500, 1000, 3000}
	for _, size := range sizes {
		data := make([]float64, size)
		for i := range data {
			data[i] = 100 + float64(i)
		}
		b.Run(fmt.Sprintf("SMA_20_%d_points", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				computeSMASlice(data, 20)
			}
		})
	}
}

// BenchmarkEMASlice benchmarks EMA computation directly.
func BenchmarkEMASlice(b *testing.B) {
	sizes := []int{60, 252, 500, 1000, 3000}
	for _, size := range sizes {
		data := make([]float64, size)
		for i := range data {
			data[i] = 100 + float64(i)
		}
		b.Run(fmt.Sprintf("EMA_20_%d_points", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				computeEMASlice(data, 20)
			}
		})
	}
}

// BenchmarkMovingStd benchmarks std computation.
func BenchmarkMovingStd(b *testing.B) {
	sizes := []int{60, 252, 500, 1000}
	for _, size := range sizes {
		data := make([]float64, size)
		for i := range data {
			data[i] = 100 + float64(i)
		}
		b.Run(fmt.Sprintf("Std_20_%d_points", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				computeMovingStd(data, 20)
			}
		})
	}
}
