package engine

import (
	"context"
	"math"
)

func init() {
	registerVolatility()
}

func registerVolatility() {
	batchRegistry["bb"] = computeBollingerBands
	batchRegistry["bollinger_bands"] = computeBollingerBands
	batchRegistry["atr"] = computeATR
	batchRegistry["tr"] = computeTrueRange
	batchRegistry["acceleration_bands"] = computeAccelerationBands
	batchRegistry["annualized_historical_volatility"] = computeAHV
	batchRegistry["bollinger_band_width"] = computeBBWidth
	batchRegistry["chandelier_exit"] = computeChandelierExit
	batchRegistry["chop"] = computeChop
	batchRegistry["donchian_channel"] = computeDonchianChannel
	batchRegistry["historical_volatility"] = computeHV
	batchRegistry["keltner_channel"] = computeKeltnerChannel
	batchRegistry["moving_std"] = computeMovingStdIndicator
	batchRegistry["percent_b"] = computePercentB
	batchRegistry["po"] = computePO
	batchRegistry["super_trend"] = computeSuperTrend
	batchRegistry["ulcer_index"] = computeUlcerIndex
	batchRegistry["z_score"] = computeZScore
}

func computeBollingerBands(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	stdev := float64(intParam(params, 1, 2))
	close := input.Close
	n := len(close)

	sma := computeSMASlice(close, period)
	std := computeMovingStd(close, period)

	upper := make([]float64, n)
	lower := make([]float64, n)
	for i := 0; i < n; i++ {
		upper[i] = sma[i] + stdev*std[i]
		lower[i] = sma[i] - stdev*std[i]
	}

	return roundRobinMulti([]string{"upper", "middle", "lower"}, upper, sma, lower), nil
}

func computeTrueRange(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	high := input.High
	low := input.Low
	close := input.Close
	n := len(high)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		tr := high[i] - low[i]
		if i > 0 {
			hl := high[i] - low[i]
			hc := math.Abs(high[i] - close[i-1])
			lc := math.Abs(low[i] - close[i-1])
			tr = math.Max(hl, math.Max(hc, lc))
		}
		result[i] = tr
	}
	return singleResult(result), nil
}

func computeATR(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	high := input.High
	low := input.Low
	close := input.Close
	n := len(high)

	// True Range
	tr := make([]float64, n)
	for i := 0; i < n; i++ {
		if i == 0 {
			tr[i] = high[i] - low[i]
		} else {
			hl := high[i] - low[i]
			hc := math.Abs(high[i] - close[i-1])
			lc := math.Abs(low[i] - close[i-1])
			tr[i] = math.Max(hl, math.Max(hc, lc))
		}
	}

	// ATR = SMA of TR (library uses SMA by default)
	result := computeSMASlice(tr, period)
	return singleResult(result), nil
}

func computeAccelerationBands(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	factor := float64(intParam(params, 1, 4))
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	// Upper Band = SMA(high * (1 + factor/1000))
	// Lower Band = SMA(low * (1 - factor/1000))
	// Middle Band = SMA(close)
	f := factor / 1000.0

	highAdj := make([]float64, n)
	lowAdj := make([]float64, n)
	for i := 0; i < n; i++ {
		highAdj[i] = high[i] * (1 + f)
		lowAdj[i] = low[i] * (1 - f)
	}

	upper := computeSMASlice(highAdj, period)
	middle := computeSMASlice(close, period)
	lower := computeSMASlice(lowAdj, period)

	return roundRobinMulti([]string{"upper", "middle", "lower"}, upper, middle, lower), nil
}

func computeAHV(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close
	n := len(close)

	// Annualized Historical Volatility = Std(returns) * sqrt(252)
	returns := computeChangePercent(close)
	std := computeMovingStd(returns, period)

	result := make([]float64, n)
	sqrt252 := math.Sqrt(252)
	for i := 0; i < n; i++ {
		result[i] = std[i] * sqrt252
	}
	return singleResult(result), nil
}

func computeBBWidth(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close
	n := len(close)

	sma := computeSMASlice(close, period)
	std := computeMovingStd(close, period)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		if sma[i] != 0 {
			// Band Width = (Upper - Lower) / Middle = 2*2*std / sma
			result[i] = 4 * std[i] / sma[i]
		}
	}
	return singleResult(result), nil
}

func computeChandelierExit(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 22)
	multiplier := float64(intParam(params, 1, 3))
	high := input.High
	low := input.Low
	n := len(high)

	atrResults, _ := computeATR(context.Background(), input, []int{period})
	atrVals := atrResults[0].Values

	maxHigh := computeMovingMax(high, period)
	minLow := computeMovingMin(low, period)

	longExit := make([]float64, n)
	shortExit := make([]float64, n)
	for i := 0; i < n; i++ {
		longExit[i] = maxHigh[i] - multiplier*atrVals[i]
		shortExit[i] = minLow[i] + multiplier*atrVals[i]
	}

	return roundRobinMulti([]string{"long", "short"}, longExit, shortExit), nil
}

func computeChop(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	high := input.High
	low := input.Low
	close := input.Close
	n := len(high)

	// CHOP = 100 * log10(sum(TR, period) / (max(high, period) - min(low, period))) / log10(period)
	tr := make([]float64, n)
	for i := 0; i < n; i++ {
		if i == 0 {
			tr[i] = high[i] - low[i]
		} else {
			hl := high[i] - low[i]
			hc := math.Abs(high[i] - close[i-1])
			lc := math.Abs(low[i] - close[i-1])
			tr[i] = math.Max(hl, math.Max(hc, lc))
		}
	}

	trSum := computeMovingSum(tr, period)
	maxHigh := computeMovingMax(high, period)
	minLow := computeMovingMin(low, period)

	logPeriod := math.Log10(float64(period))
	result := make([]float64, n)
	for i := period - 1; i < n; i++ {
		hl := maxHigh[i] - minLow[i]
		if hl > 0 && logPeriod != 0 {
			result[i] = 100.0 * math.Log10(trSum[i]/hl) / logPeriod
		}
	}
	return singleResult(result), nil
}

func computeDonchianChannel(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	high := input.High
	low := input.Low
	n := len(high)

	maxHigh := computeMovingMax(high, period)
	minLow := computeMovingMin(low, period)

	middle := make([]float64, n)
	for i := 0; i < n; i++ {
		middle[i] = (maxHigh[i] + minLow[i]) / 2.0
	}

	return roundRobinMulti([]string{"upper", "middle", "lower"}, maxHigh, middle, minLow), nil
}

func computeHV(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close
	n := len(close)

	// Historical Volatility = Std of log returns over period
	logReturns := make([]float64, n)
	for i := 1; i < n; i++ {
		if close[i-1] > 0 {
			logReturns[i] = math.Log(close[i] / close[i-1])
		}
	}

	result := computeMovingStd(logReturns, period)
	return singleResult(result), nil
}

func computeKeltnerChannel(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	multiplier := float64(intParam(params, 1, 2))
	close := input.Close
	n := len(close)

	// Middle = EMA(close)
	// Upper = EMA(close) + multiplier * ATR
	// Lower = EMA(close) - multiplier * ATR
	ema := computeEMASlice(close, period)

	atrResults, _ := computeATR(context.Background(), input, []int{period})
	atrVals := atrResults[0].Values

	upper := make([]float64, n)
	lower := make([]float64, n)
	for i := 0; i < n; i++ {
		upper[i] = ema[i] + multiplier*atrVals[i]
		lower[i] = ema[i] - multiplier*atrVals[i]
	}

	return roundRobinMulti([]string{"upper", "middle", "lower"}, upper, ema, lower), nil
}

func computeMovingStdIndicator(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	return singleResult(computeMovingStd(input.Close, period)), nil
}

func computePercentB(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close
	n := len(close)

	sma := computeSMASlice(close, period)
	std := computeMovingStd(close, period)

	// %B = (close - lower) / (upper - lower) = (close - (sma-2*std)) / (4*std)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		if std[i] != 0 {
			result[i] = (close[i] - (sma[i] - 2*std[i])) / (4 * std[i])
		}
	}
	return singleResult(result), nil
}

func computePO(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	high := input.High
	low := input.Low
	n := len(high)

	// Projection Oscillator: PO = (high - low) / (max(high) - min(low)) * 100
	maxHigh := computeMovingMax(high, period)
	minLow := computeMovingMin(low, period)

	result := make([]float64, n)
	for i := period - 1; i < n; i++ {
		hl := maxHigh[i] - minLow[i]
		if hl != 0 {
			result[i] = (high[i] - low[i]) / hl * 100.0
		}
	}
	return singleResult(result), nil
}

func computeSuperTrend(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 10)
	multiplier := float64(intParam(params, 1, 3))
	high := input.High
	low := input.Low
	close := input.Close
	n := len(high)

	// SuperTrend uses ATR-based bands
	atrResults, _ := computeATR(context.Background(), input, []int{period})
	atrVals := atrResults[0].Values

	// Basic Bands
	hlAvg := make([]float64, n)
	for i := 0; i < n; i++ {
		hlAvg[i] = (high[i] + low[i]) / 2.0
	}

	upperBand := make([]float64, n)
	lowerBand := make([]float64, n)
	for i := 0; i < n; i++ {
		upperBand[i] = hlAvg[i] + multiplier*atrVals[i]
		lowerBand[i] = hlAvg[i] - multiplier*atrVals[i]
	}

	trend := make([]float64, n)
	superTrend := make([]float64, n)

	// Initialize
	for i := period - 1; i < n; i++ {
		if i == period-1 {
			if close[i] <= upperBand[i] {
				trend[i] = 1 // downtrend
				superTrend[i] = upperBand[i]
			} else {
				trend[i] = -1 // uptrend (but actually bull)
				superTrend[i] = lowerBand[i]
			}
		} else {
			if close[i] > upperBand[i-1] {
				trend[i] = -1 // uptrend signal
			} else if close[i] < lowerBand[i-1] {
				trend[i] = 1 // downtrend signal
			} else {
				trend[i] = trend[i-1]
			}

			if trend[i] == -1 {
				superTrend[i] = math.Max(lowerBand[i], superTrend[i-1])
				if superTrend[i] != lowerBand[i] {
					superTrend[i] = lowerBand[i]
				}
			} else {
				superTrend[i] = math.Min(upperBand[i], superTrend[i-1])
				if superTrend[i] != upperBand[i] {
					superTrend[i] = upperBand[i]
				}
			}
		}
	}

	return roundRobinMulti([]string{"super_trend", "trend"}, superTrend, trend), nil
}

func computeUlcerIndex(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	close := input.Close
	n := len(close)

	result := make([]float64, n)
	if n < period {
		return singleResult(result), nil
	}

	// Ulcer Index = sqrt(sum((close[i] - max_close[i])/max_close[i])^2 / period)
	maxClose := computeMovingMax(close, period)

	for i := period - 1; i < n; i++ {
		sumSq := 0.0
		for j := i - period + 1; j <= i; j++ {
			percentDrawdown := (close[j] - maxClose[i]) / maxClose[i] * 100.0
			sumSq += percentDrawdown * percentDrawdown
		}
		result[i] = math.Sqrt(sumSq / float64(period))
	}
	return singleResult(result), nil
}

func computeZScore(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close
	n := len(close)

	sma := computeSMASlice(close, period)
	std := computeMovingStd(close, period)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		if std[i] != 0 {
			result[i] = (close[i] - sma[i]) / std[i]
		}
	}
	return singleResult(result), nil
}
