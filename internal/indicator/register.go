package indicator

import (
	"context"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/momentum"
	"github.com/cinar/indicator/v2/trend"
	"github.com/cinar/indicator/v2/volatility"
	"github.com/cinar/indicator/v2/volume"
)

func init() {
	registerTrend()
	registerMomentum()
	registerVolatility()
	registerVolume()
}

// registerTrend registers all trend indicators.
func registerTrend() {
	add("sma_50", "trend", "Simple Moving Average",
		"A simple moving average smooths price data by creating a constantly updated average price over a time period.",
		"Best for identifying trend direction and support/resistance levels.",
		[]string{"close"}, 1, nil, map[string]int{"period": 50},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := trend.NewSmaWithPeriod[float64](50)
			return single(ctx, p.ComputeWithContext(ctx, s.Close))
		})

	add("sma_20", "trend", "Simple Moving Average (20)",
		"Short-term simple moving average.",
		"Best for short-term trend following.",
		[]string{"close"}, 1, nil, map[string]int{"period": 20},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := trend.NewSmaWithPeriod[float64](20)
			return single(ctx, p.ComputeWithContext(ctx, s.Close))
		})

	add("ema_20", "trend", "Exponential Moving Average",
		"An exponential moving average gives more weight to recent prices, making it more responsive to new information.",
		"Best for identifying trend direction with faster signal response than SMA.",
		[]string{"close"}, 1, nil, map[string]int{"period": 20},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := trend.NewEmaWithPeriod[float64](20)
			return single(ctx, p.ComputeWithContext(ctx, s.Close))
		})

	add("macd_12_26_9", "trend", "Moving Average Convergence Divergence",
		"MACD shows the relationship between two exponential moving averages, generating signals through crossovers.",
		"Best for identifying trend direction, momentum shifts, and potential entry/exit points.",
		[]string{"close"}, 3, []string{"line", "signal", "histogram"}, map[string]int{"fast_period": 12, "slow_period": 26, "signal_period": 9},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := trend.NewMacd[float64]()
			lineCh, signalCh := p.ComputeWithContext(ctx, s.Close)
			res := collectMulti(ctx, []string{"line", "signal"}, lineCh, signalCh)
			// Compute histogram from slices: histogram[i] = line[i] - signal[i]
			line := res[0].Values
			signal := res[1].Values
			n := len(line)
			if len(signal) < n {
				n = len(signal)
			}
			hist := make([]float64, n)
			for i := 0; i < n; i++ {
				hist[i] = line[i] - signal[i]
			}
			return []IndicatorResult{
				{SubIndicator: "line", Values: line},
				{SubIndicator: "signal", Values: signal},
				{SubIndicator: "histogram", Values: hist},
			}
		})

	add("vwma_20", "trend", "Volume Weighted Moving Average",
		"VWMA is a moving average that weights each period's price by its volume, giving more significance to high-volume periods.",
		"Best for confirming trends when volume supports price movement.",
		[]string{"close", "volume"}, 1, nil, map[string]int{"period": 20},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := trend.NewVwma[float64]()
			return single(ctx, p.ComputeWithContext(ctx, s.Close, s.Volume))
		})

	add("apo_14_30", "trend", "Absolute Price Oscillator",
		"The APO measures the difference between two exponential moving averages, expressed in absolute terms.",
		"Best for identifying trend strength and momentum shifts.",
		[]string{"close"}, 1, nil, map[string]int{"fast_period": 14, "slow_period": 30},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := trend.NewApo[float64]()
			return single(ctx, p.ComputeWithContext(ctx, s.Close))
		})

	add("roc_9", "trend", "Rate of Change",
		"ROC measures the percentage change in price between the current price and a price from n periods ago.",
		"Best for identifying overbought/oversold conditions and momentum divergences.",
		[]string{"close"}, 1, nil, map[string]int{"period": 9},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := trend.NewRoc[float64]()
			return single(ctx, p.ComputeWithContext(ctx, s.Close))
		})
}

// registerMomentum registers all momentum indicators.
func registerMomentum() {
	add("rsi_14", "momentum", "Relative Strength Index",
		"Momentum oscillator measuring the speed and magnitude of recent price changes to identify overbought (>70) and oversold (<30) conditions.",
		"Best for identifying trend reversals and overbought/oversold levels in ranging markets.",
		[]string{"close"}, 1, nil, map[string]int{"period": 14},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := momentum.NewRsi[float64]()
			return single(ctx, p.ComputeWithContext(ctx, s.Close))
		})

	add("stoch_14_3", "momentum", "Stochastic Oscillator",
		"The Stochastic Oscillator compares a closing price to the high-low range over a period, generating %K and %D lines.",
		"Best for identifying overbought (>80) and oversold (<20) conditions in ranging markets.",
		[]string{"high", "low", "close"}, 2, []string{"k", "d"}, map[string]int{"period": 14, "sma_period": 3},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := momentum.NewStochasticOscillator[float64]()
			k, d := p.ComputeWithContext(ctx, s.High, s.Low, s.Close)
			return collectMulti(ctx, []string{"k", "d"}, k, d)
		})

	add("williams_r_14", "momentum", "Williams %R",
		"Williams %R shows the current closing price relative to the high-low range. Values below -80 indicate oversold, above -20 indicate overbought.",
		"Best for identifying overbought and oversold conditions and potential reversals.",
		[]string{"high", "low", "close"}, 1, nil, map[string]int{"period": 14},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := momentum.NewWilliamsR[float64]()
			return single(ctx, p.ComputeWithContext(ctx, s.High, s.Low, s.Close))
		})

	add("awesome_oscillator", "momentum", "Awesome Oscillator",
		"The Awesome Oscillator gauges market momentum by comparing short-term price action (5-period) against long-term trends (34-period).",
		"Best for identifying momentum shifts and zero-line crossovers.",
		[]string{"high", "low"}, 1, nil, map[string]int{"short_period": 5, "long_period": 34},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := momentum.NewAwesomeOscillator[float64]()
			return single(ctx, p.ComputeWithContext(ctx, s.High, s.Low))
		})

	add("ibs", "momentum", "Internal Bar Strength",
		"IBS tracks the position of the close within the daily high-low range. Values near 1 indicate buying pressure, near 0 indicate selling pressure.",
		"Best for identifying intraday momentum and extreme price positioning.",
		[]string{"high", "low", "close"}, 1, nil, nil,
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := momentum.NewInternalBarStrength[float64]()
			return single(ctx, p.ComputeWithContext(ctx, s.High, s.Low, s.Close))
		})
}

// registerVolatility registers all volatility indicators.
func registerVolatility() {
	add("bb_20_2", "volatility", "Bollinger Bands",
		"Bollinger Bands consist of a middle SMA band with upper and lower bands two standard deviations away, indicating volatility and potential overextension.",
		"Best for identifying volatility expansion/contraction and potential reversal points when price touches the bands.",
		[]string{"close"}, 3, []string{"upper", "middle", "lower"}, map[string]int{"period": 20, "stdev": 2},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := volatility.NewBollingerBands[float64]()
			upper, middle, lower := p.ComputeWithContext(ctx, s.Close)
			return collectMulti(ctx,
				[]string{"upper", "middle", "lower"},
				upper, middle, lower)
		})

	add("atr_14", "volatility", "Average True Range",
		"ATR measures market volatility by decomposing the entire range of price movement over a period.",
		"Best for setting stop-loss levels and measuring volatility regardless of direction.",
		[]string{"high", "low", "close"}, 1, nil, map[string]int{"period": 14},
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := volatility.NewAtr[float64]()
			return single(ctx, p.ComputeWithContext(ctx, s.High, s.Low, s.Close))
		})

	add("tr", "volatility", "True Range",
		"True Range is the greatest of: current high minus current low, absolute of current high minus previous close, absolute of current low minus previous close.",
		"Best for measuring raw volatility as a building block for ATR and other indicators.",
		[]string{"high", "low", "close"}, 1, nil, nil,
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := volatility.NewTrueRange[float64]()
			return single(ctx, p.ComputeWithContext(ctx, s.High, s.Low, s.Close))
		})
}

// registerVolume registers all volume indicators.
func registerVolume() {
	add("obv", "volume", "On-Balance Volume",
		"OBV uses volume flow to predict changes in stock price. It adds volume on up days and subtracts on down days.",
		"Best for confirming price trends — OBV should move in the same direction as price.",
		[]string{"close", "volume"}, 1, nil, nil,
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := volume.NewObv[float64]()
			return single(ctx, p.ComputeWithContext(ctx, s.Close, s.Volume))
		})

	add("ad", "volume", "Accumulation/Distribution",
		"The A/D line measures cumulative buying and selling pressure by considering the position of the close within the daily range, multiplied by volume.",
		"Best for confirming price trends and detecting divergence that may signal reversals.",
		[]string{"high", "low", "close", "volume"}, 1, nil, nil,
		func(ctx context.Context, s *OHLCVStreams) []IndicatorResult {
			p := volume.NewAd[float64]()
			return single(ctx, p.ComputeWithContext(ctx, s.High, s.Low, s.Close, s.Volume))
		})
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// add registers an indicator in both the metadata Registry and the ComputeMap.
func add(key, category, displayName, description, whenToUse string, inputs []string,
	outputs int, subIndicators []string, defaultParams map[string]int, fn ComputeFunc) {

	Registry[key] = IndicatorDef{
		Key:           key,
		Category:      category,
		DisplayName:   displayName,
		Description:   description,
		WhenToUse:     whenToUse,
		Inputs:        inputs,
		Outputs:       outputs,
		SubIndicators: subIndicators,
		DefaultParams: defaultParams,
	}
	ComputeMap[key] = fn
}

// single wraps a single-output indicator result.
func single(ctx context.Context, c <-chan float64) []IndicatorResult {
	return []IndicatorResult{{Values: collect(ctx, c)}}
}

// collect drains a channel into a slice.
func collect(ctx context.Context, c <-chan float64) []float64 {
	return helper.ChanToSlice(c)
}

// collectMulti drains multiple channels concurrently and returns named results.
// This avoids deadlocks when a producer writes to all channels in lockstep.
func collectMulti(ctx context.Context, names []string, chs ...<-chan float64) []IndicatorResult {
	if len(names) != len(chs) {
		panic("collectMulti: names and channels length mismatch")
	}
	results := make([]IndicatorResult, len(chs))
	vals := make([][]float64, len(chs))
	done := make(chan int, len(chs))

	for i, ch := range chs {
		i, ch := i, ch
		go func() {
			vals[i] = collect(ctx, ch)
			done <- i
		}()
	}
	for range chs {
		<-done
	}
	for i, name := range names {
		results[i] = IndicatorResult{SubIndicator: name, Values: vals[i]}
	}
	return results
}
