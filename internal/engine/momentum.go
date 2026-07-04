package engine

import (
	"context"
	"math"
)

func init() {
	registerMomentum()
}

func registerMomentum() {
	batchRegistry["rsi"] = computeRSI
	batchRegistry["stoch"] = computeStochastic
	batchRegistry["stochastic_oscillator"] = computeStochasticOscillator
	batchRegistry["williams_r"] = computeWilliamsR
	batchRegistry["awesome_oscillator"] = computeAwesomeOscillator
	batchRegistry["ibs"] = computeIBS
	batchRegistry["chaikin_oscillator"] = computeChaikinOscillator
	batchRegistry["connors_rsi"] = computeConnorsRSI
	batchRegistry["coppock_curve"] = computeCoppockCurve
	batchRegistry["elder_ray"] = computeElderRay
	batchRegistry["fisher"] = computeFisher
	batchRegistry["ichimoku_cloud"] = computeIchimokuCloud
	batchRegistry["ppo"] = computePPO
	batchRegistry["prings_special_k"] = computePringsSpecialK
	batchRegistry["pvo"] = computePVO
	batchRegistry["qstick"] = computeQstick
	batchRegistry["rvi"] = computeRVI
	batchRegistry["stochastic_rsi"] = computeStochasticRSI
	batchRegistry["td_sequential"] = computeTDSequential
	batchRegistry["ultimate_oscillator"] = computeUltimateOscillator
}

func computeRSI(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	close := input.Close
	n := len(close)
	result := make([]float64, n)

	if n < period+1 {
		return singleResult(result), nil
	}

	// Price changes
	gains := make([]float64, n)
	losses := make([]float64, n)
	for i := 1; i < n; i++ {
		diff := close[i] - close[i-1]
		if diff > 0 {
			gains[i] = diff
		} else {
			losses[i] = -diff
		}
	}

	// Initial average using SMA (as the library does with RMA using SMA seed)
	avgGain := 0.0
	avgLoss := 0.0
	for i := 1; i <= period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	if avgLoss == 0 {
		result[period] = 100
	} else {
		rs := avgGain / avgLoss
		result[period] = 100 - (100 / (1 + rs))
	}

	// Wilder's smoothing for remaining
	for i := period + 1; i < n; i++ {
		avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)
		if avgLoss == 0 {
			result[i] = 100
		} else {
			rs := avgGain / avgLoss
			result[i] = 100 - (100 / (1 + rs))
		}
	}

	return singleResult(result), nil
}

func computeWilliamsR(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	minLow := computeMovingMin(low, period)
	maxHigh := computeMovingMax(high, period)

	result := make([]float64, n)
	for i := period - 1; i < n; i++ {
		hh := maxHigh[i]
		ll := minLow[i]
		if hh != ll {
			result[i] = (hh - close[i]) / (hh - ll) * -100.0
		}
	}
	return singleResult(result), nil
}

func computeAwesomeOscillator(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	high := input.High
	low := input.Low
	n := len(high)

	// Median Price = (high + low) / 2
	mp := make([]float64, n)
	for i := 0; i < n; i++ {
		mp[i] = (high[i] + low[i]) / 2.0
	}

	sma5 := computeSMASlice(mp, 5)
	sma34 := computeSMASlice(mp, 34)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = sma5[i] - sma34[i]
	}
	return singleResult(result), nil
}

func computeIBS(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		hl := high[i] - low[i]
		if hl != 0 {
			result[i] = (close[i] - low[i]) / hl
		}
	}
	return singleResult(result), nil
}

func computeStochasticOscillator(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	smaPeriod := intParam(params, 1, 3)
	return computeStochasticImpl(input, period, smaPeriod)
}

// computeStochasticImpl is the shared implementation for Stochastic and Stochastic Oscillator.
func computeStochasticImpl(input *Input, period, smaPeriod int) ([]Result, error) {
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	minLow := computeMovingMin(low, period)
	maxHigh := computeMovingMax(high, period)

	// K = (Closing - Lowest Low) / (Highest High - Lowest Low) * 100
	k := make([]float64, n)
	for i := period - 1; i < n; i++ {
		hh := maxHigh[i]
		ll := minLow[i]
		if hh != ll {
			k[i] = (close[i] - ll) / (hh - ll) * 100.0
		}
	}

	// D = 3-Period SMA of K
	d := computeSMASlice(k, smaPeriod)

	return roundRobinMulti([]string{"k", "d"}, k, d), nil
}

func computeChaikinOscillator(_ context.Context, input *Input, params []int) ([]Result, error) {
	fastPeriod := intParam(params, 0, 3)
	slowPeriod := intParam(params, 1, 10)
	high := input.High
	low := input.Low
	close := input.Close
	volume := input.Volume
	n := len(close)

	// A/D Line
	mfm := make([]float64, n)
	mfv := make([]float64, n)
	ad := make([]float64, n)
	for i := 0; i < n; i++ {
		hl := high[i] - low[i]
		if hl != 0 {
			mfm[i] = ((close[i] - low[i]) - (high[i] - close[i])) / hl
		}
		mfv[i] = mfm[i] * volume[i]
		if i == 0 {
			ad[i] = mfv[i]
		} else {
			ad[i] = ad[i-1] + mfv[i]
		}
	}

	emaFast := computeEMASlice(ad, fastPeriod)
	emaSlow := computeEMASlice(ad, slowPeriod)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = emaFast[i] - emaSlow[i]
	}
	return singleResult(result), nil
}

func computeConnorsRSI(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	close := input.Close
	n := len(close)

	if n < period+1 {
		return singleResult(make([]float64, n)), nil
	}

	// RSI of close
	rsiClose := computeRSISlice(close, period)

	// RSI of streak (consecutive up/down days)
	streak := make([]float64, n)
	for i := 1; i < n; i++ {
		if close[i] > close[i-1] {
			if i > 1 && close[i-1] > close[i-2] {
				streak[i] = streak[i-1] + 1
			} else {
				streak[i] = 1
			}
		} else if close[i] < close[i-1] {
			if i > 1 && close[i-1] < close[i-2] {
				streak[i] = streak[i-1] - 1
			} else {
				streak[i] = -1
			}
		}
	}
	rsiStreak := computeRSISlice(streak, 2)

	// ROC(close, 3) percentile ranking
	roc := make([]float64, n)
	for i := 3; i < n; i++ {
		if close[i-3] != 0 {
			roc[i] = (close[i] - close[i-3]) / close[i-3] * 100.0
		}
	}
	rocRank := computePercentRank(roc, period)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = (rsiClose[i] + rsiStreak[i] + rocRank[i]) / 3.0
	}
	return singleResult(result), nil
}

// computeRSISlice computes RSI values directly from a slice.
func computeRSISlice(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)
	if n < period+1 {
		return result
	}

	gains := make([]float64, n)
	losses := make([]float64, n)
	for i := 1; i < n; i++ {
		diff := data[i] - data[i-1]
		if diff > 0 {
			gains[i] = diff
		} else {
			losses[i] = -diff
		}
	}

	avgGain := 0.0
	avgLoss := 0.0
	for i := 1; i <= period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	if avgLoss == 0 {
		result[period] = 100
	} else {
		result[period] = 100 - (100 / (1 + avgGain/avgLoss))
	}

	for i := period + 1; i < n; i++ {
		avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)
		if avgLoss == 0 {
			result[i] = 100
		} else {
			result[i] = 100 - (100 / (1 + avgGain/avgLoss))
		}
	}
	return result
}

// computePercentRank computes rolling percentile rank over a window.
func computePercentRank(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)

	for i := period; i < n; i++ {
		current := data[i]
		count := 0
		for j := i - period + 1; j <= i; j++ {
			if data[j] <= current {
				count++
			}
		}
		result[i] = float64(count) / float64(period) * 100.0
	}
	return result
}

func computeCoppockCurve(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	close := input.Close
	n := len(close)

	// Coppock Curve: WMA[10](ROC[14] + ROC[11])
	roc14 := make([]float64, n)
	for i := 14; i < n; i++ {
		if close[i-14] != 0 {
			roc14[i] = (close[i] - close[i-14]) / close[i-14] * 100.0
		}
	}
	roc11 := make([]float64, n)
	for i := 11; i < n; i++ {
		if close[i-11] != 0 {
			roc11[i] = (close[i] - close[i-11]) / close[i-11] * 100.0
		}
	}

	sum := make([]float64, n)
	for i := 0; i < n; i++ {
		sum[i] = roc14[i] + roc11[i]
	}

	result := computeWMASlice(sum, 10)
	return singleResult(result), nil
}

func computeElderRay(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 13)
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	// Bull Power = High - EMA(close, 13)
	// Bear Power = Low - EMA(close, 13)
	ema := computeEMASlice(close, period)

	bull := make([]float64, n)
	bear := make([]float64, n)
	for i := 0; i < n; i++ {
		bull[i] = high[i] - ema[i]
		bear[i] = low[i] - ema[i]
	}

	return roundRobinMulti([]string{"bull", "bear"}, bull, bear), nil
}

func computeFisher(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 9)
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	result := make([]float64, n)
	if n < period {
		return singleResult(result), nil
	}

	minLow := computeMovingMin(low, period)
	maxHigh := computeMovingMax(high, period)

	// Fisher Transform: 0.5 * ln((1+x)/(1-x))
	value := 0.0
	for i := period - 1; i < n; i++ {
		hh := maxHigh[i]
		ll := minLow[i]
		var x float64
		if hh != ll {
			x = (close[i]-ll)/(hh-ll)*2.0 - 1.0
		} else {
			x = 0
		}
		// Clamp to avoid ln(0)
		if x > 0.99 {
			x = 0.99
		} else if x < -0.99 {
			x = -0.99
		}
		value = 0.5*math.Log((1+x)/(1-x)) + 0.5*value
		result[i] = value
	}
	return singleResult(result), nil
}

func computeIchimokuCloud(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params // Default: 9, 26, 52
	convPeriod := intParam(params, 0, 9)
	basePeriod := intParam(params, 1, 26)
	spanBPeriod := intParam(params, 2, 52)
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	// Tenkan-sen (Conversion Line): (highest high + lowest low) / 2 over 9 periods
	maxHigh9 := computeMovingMax(high, convPeriod)
	minLow9 := computeMovingMin(low, convPeriod)
	tenkan := make([]float64, n)
	for i := 0; i < n; i++ {
		tenkan[i] = (maxHigh9[i] + minLow9[i]) / 2.0
	}

	// Kijun-sen (Base Line): (highest high + lowest low) / 2 over 26 periods
	maxHigh26 := computeMovingMax(high, basePeriod)
	minLow26 := computeMovingMin(low, basePeriod)
	kijun := make([]float64, n)
	for i := 0; i < n; i++ {
		kijun[i] = (maxHigh26[i] + minLow26[i]) / 2.0
	}

	// Senkou Span A (Leading Span A): (tenkan + kijun) / 2, shifted forward 26 periods
	senkouA := make([]float64, n)
	for i := 0; i < n; i++ {
		senkouA[i] = (tenkan[i] + kijun[i]) / 2.0
	}

	// Senkou Span B (Leading Span B): (highest high + lowest low) / 2 over 52, shifted forward 26
	maxHigh52 := computeMovingMax(high, spanBPeriod)
	minLow52 := computeMovingMin(low, spanBPeriod)
	senkouB := make([]float64, n)
	for i := 0; i < n; i++ {
		senkouB[i] = (maxHigh52[i] + minLow52[i]) / 2.0
	}

	// Chikou Span (Lagging Span): close shifted backward 26 periods
	chikou := make([]float64, n)
	for i := 0; i < n-basePeriod; i++ {
		chikou[i] = close[i+basePeriod]
	}

	return roundRobinMulti([]string{"tenkan", "kijun", "senkou_a", "senkou_b", "chikou"},
		tenkan, kijun, senkouA, senkouB, chikou), nil
}

func computePPO(_ context.Context, input *Input, params []int) ([]Result, error) {
	fastPeriod := intParam(params, 0, 12)
	slowPeriod := intParam(params, 1, 26)
	signalPeriod := intParam(params, 2, 9)
	close := input.Close

	// PPO = ((EMA(fast) - EMA(slow)) / EMA(slow)) * 100
	fastEMA := computeEMASlice(close, fastPeriod)
	slowEMA := computeEMASlice(close, slowPeriod)

	n := len(close)
	ppo := make([]float64, n)
	for i := 0; i < n; i++ {
		if slowEMA[i] != 0 {
			ppo[i] = (fastEMA[i] - slowEMA[i]) / slowEMA[i] * 100.0
		}
	}

	signal := computeEMASlice(ppo, signalPeriod)

	histogram := make([]float64, n)
	for i := 0; i < n; i++ {
		histogram[i] = ppo[i] - signal[i]
	}

	return roundRobinMulti([]string{"ppo", "signal", "histogram"}, ppo, signal, histogram), nil
}

func computePringsSpecialK(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	close := input.Close
	n := len(close)

	// Special K combines ROC across multiple timeframes
	roc10 := computeROCIntegral(close, 10, 10)
	roc15 := computeROCIntegral(close, 15, 10)
	roc20 := computeROCIntegral(close, 20, 10)
	roc30 := computeROCIntegral(close, 30, 15)
	roc40 := computeROCIntegral(close, 40, 10)
	roc50 := computeROCIntegral(close, 50, 10)
	roc65 := computeROCIntegral(close, 65, 15)
	roc75 := computeROCIntegral(close, 75, 10)
	roc100 := computeROCIntegral(close, 100, 15)
	roc195 := computeROCIntegral(close, 195, 65)
	roc265 := computeROCIntegral(close, 265, 90)
	roc390 := computeROCIntegral(close, 390, 130)
	roc530 := computeROCIntegral(close, 530, 195)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = roc10[i]*1 + roc15[i]*2 + roc20[i]*3 + roc30[i]*4 +
			roc40[i]*1 + roc50[i]*2 + roc65[i]*3 + roc75[i]*4 +
			roc100[i]*1 + roc195[i]*2 + roc265[i]*3 + roc390[i]*4 + roc530[i]*1
	}
	return singleResult(result), nil
}

func computePVO(_ context.Context, input *Input, params []int) ([]Result, error) {
	fastPeriod := intParam(params, 0, 12)
	slowPeriod := intParam(params, 1, 26)
	signalPeriod := intParam(params, 2, 9)
	volume := input.Volume

	// PVO = ((EMA(fast, volume) - EMA(slow, volume)) / EMA(slow, volume)) * 100
	fastEMA := computeEMASlice(volume, fastPeriod)
	slowEMA := computeEMASlice(volume, slowPeriod)

	n := len(volume)
	pvo := make([]float64, n)
	for i := 0; i < n; i++ {
		if slowEMA[i] != 0 {
			pvo[i] = (fastEMA[i] - slowEMA[i]) / slowEMA[i] * 100.0
		}
	}

	signal := computeEMASlice(pvo, signalPeriod)
	histogram := subtract(pvo, signal)

	return roundRobinMulti([]string{"pvo", "signal", "histogram"}, pvo, signal, histogram), nil
}

func computeQstick(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	open := input.Open
	close := input.Close
	n := len(close)

	diff := make([]float64, n)
	for i := 0; i < n; i++ {
		diff[i] = close[i] - open[i]
	}

	result := computeSMASlice(diff, period)
	return singleResult(result), nil
}

func computeRVI(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	// RVI numerator = close - open, denominator = high - low
	num := make([]float64, n)
	den := make([]float64, n)
	for i := 0; i < n; i++ {
		num[i] = close[i] - low[i]
		den[i] = high[i] - low[i]
	}

	smaNum := computeSMASlice(num, period)
	smaDen := computeSMASlice(den, period)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		if smaDen[i] != 0 {
			result[i] = smaNum[i] / smaDen[i]
		}
	}
	return singleResult(result), nil
}

func computeStochasticRSI(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	smoothK := intParam(params, 1, 3)
	smoothD := intParam(params, 2, 3)
	close := input.Close

	// Stochastic RSI = Stochastic(K) applied to RSI values
	rsi := computeRSISlice(close, period)

	// Compute StochRSI
	minRsi := computeMovingMin(rsi, period)
	maxRsi := computeMovingMax(rsi, period)

	n := len(close)
	stochRsi := make([]float64, n)
	for i := period - 1; i < n; i++ {
		if maxRsi[i] != minRsi[i] {
			stochRsi[i] = (rsi[i] - minRsi[i]) / (maxRsi[i] - minRsi[i])
		}
	}

	k := computeSMASlice(stochRsi, smoothK)
	d := computeSMASlice(k, smoothD)

	return roundRobinMulti([]string{"k", "d"}, k, d), nil
}

func computeTDSequential(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	close := input.Close
	n := len(close)

	// TD Sequential Setup: count consecutive closes higher/lower than 4 bars ago
	setup := make([]float64, n)
	countdown := make([]float64, n)

	setupCount := 0
	cdCount := 0
	cdActive := false

	for i := 4; i < n; i++ {
		// Setup
		if close[i] > close[i-4] {
			if setupCount >= 0 {
				setupCount++
			} else {
				setupCount = 1
			}
		} else if close[i] < close[i-4] {
			if setupCount <= 0 {
				setupCount--
			} else {
				setupCount = -1
			}
		}
		setup[i] = float64(setupCount)

		// Simplified countdown (after setup completes)
		if absInt(setupCount) >= 9 && !cdActive {
			cdActive = true
			cdCount = 0
		}
		if cdActive {
			if close[i] <= close[i-2] && setupCount < 0 {
				cdCount++
			} else if close[i] >= close[i-2] && setupCount > 0 {
				cdCount++
			}
			countdown[i] = float64(cdCount)
			if cdCount >= 13 {
				cdActive = false
			}
		}
	}

	return roundRobinMulti([]string{"setup", "countdown"}, setup, countdown), nil
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func computeUltimateOscillator(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	// Ultimate Oscillator uses three timeframes: 7, 14, 28
	result := make([]float64, n)
	if n < 28 {
		return singleResult(result), nil
	}

	// Use default periods: 7, 14, 28 with weights 4, 2, 1
	uo7 := computeUOComponent(high, low, close, 7)
	uo14 := computeUOComponent(high, low, close, 14)
	uo28 := computeUOComponent(high, low, close, 28)

	for i := 0; i < n; i++ {
		total := 4*uo7[i] + 2*uo14[i] + 1*uo28[i]
		result[i] = total / 7.0 * 100.0
	}
	return singleResult(result), nil
}

// computeUOComponent computes a single component for the Ultimate Oscillator.
func computeUOComponent(high, low, close []float64, period int) []float64 {
	n := len(high)
	result := make([]float64, n)

	bp := make([]float64, n) // buying pressure
	tr := make([]float64, n) // true range

	for i := 0; i < n; i++ {
		var prevClose float64
		if i > 0 {
			prevClose = close[i-1]
		} else {
			prevClose = close[i]
		}
		bp[i] = close[i] - math.Min(low[i], prevClose)
		tr[i] = math.Max(high[i], prevClose) - math.Min(low[i], prevClose)
	}

	bpSum := computeMovingSum(bp, period)
	trSum := computeMovingSum(tr, period)

	for i := period - 1; i < n; i++ {
		if trSum[i] != 0 {
			result[i] = bpSum[i] / trSum[i]
		}
	}
	return result
}
