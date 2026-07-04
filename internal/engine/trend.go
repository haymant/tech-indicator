package engine

import (
	"context"
	"math"
)

func init() {
	registerTrend()
}

func registerTrend() {
	batchRegistry["sma"] = computeSMA
	batchRegistry["ema"] = computeEMA
	batchRegistry["macd"] = computeMACD
	batchRegistry["vwma"] = computeVWMA
	batchRegistry["apo"] = computeAPO
	batchRegistry["roc"] = computeROC
	batchRegistry["aroon"] = computeAroon
	batchRegistry["bop"] = computeBOP
	batchRegistry["cci"] = computeCCI
	batchRegistry["cfo"] = computeCFO
	batchRegistry["dema"] = computeDEMA
	batchRegistry["dpo"] = computeDPO
	batchRegistry["envelope"] = computeEnvelope
	batchRegistry["hma"] = computeHMA
	batchRegistry["kama"] = computeKAMA
	batchRegistry["kdj"] = computeKDJ
	batchRegistry["kst"] = computeKST
	batchRegistry["mass_index"] = computeMassIndex
	batchRegistry["mcginley"] = computeMcGinley
	batchRegistry["mlr"] = computeMLR
	batchRegistry["mls"] = computeMLS
	batchRegistry["moving_max"] = computeMovingMaxIndicator
	batchRegistry["moving_min"] = computeMovingMinIndicator
	batchRegistry["moving_sum"] = computeMovingSumIndicator
	batchRegistry["pivot_point"] = computePivotPoint
	batchRegistry["rma"] = computeRMA
	batchRegistry["slope"] = computeSlope
	batchRegistry["slow_stochastic"] = computeSlowStochastic
	batchRegistry["smma"] = computeSMMA
	batchRegistry["stc"] = computeSTC
	batchRegistry["t3"] = computeT3
	batchRegistry["tema"] = computeTEMA
	batchRegistry["trima"] = computeTRIMA
	batchRegistry["trix"] = computeTRIX
	batchRegistry["tsi"] = computeTSI
	batchRegistry["typical_price"] = computeTypicalPrice
	batchRegistry["weighted_close"] = computeWeightedClose
	batchRegistry["wma"] = computeWMA
	batchRegistry["ma"] = computeMA
	batchRegistry["stochastic"] = computeStochastic
	batchRegistry["slow_stoch"] = computeSlowStochastic
}

func computeSMA(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	return singleResult(computeSMASlice(input.Close, period)), nil
}

func computeEMA(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	return singleResult(computeEMASlice(input.Close, period)), nil
}

func computeMACD(_ context.Context, input *Input, params []int) ([]Result, error) {
	fastPeriod := intParam(params, 0, 12)
	slowPeriod := intParam(params, 1, 26)
	signalPeriod := intParam(params, 2, 9)
	close := input.Close
	n := len(close)

	fastEMA := computeEMASlice(close, fastPeriod)
	slowEMA := computeEMASlice(close, slowPeriod)

	// MACD line = fastEMA - slowEMA, aligned by slow period
	macdLine := make([]float64, n)
	for i := 0; i < n; i++ {
		macdLine[i] = fastEMA[i] - slowEMA[i]
	}

	// Signal line = EMA of MACD line
	signalLine := computeEMASlice(macdLine, signalPeriod)

	// Histogram = MACD line - Signal line
	histogram := make([]float64, n)
	for i := 0; i < n; i++ {
		histogram[i] = macdLine[i] - signalLine[i]
	}

	return roundRobinMulti([]string{"line", "signal", "histogram"}, macdLine, signalLine, histogram), nil
}

func computeVWMA(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close
	volume := input.Volume

	pv := multiply(close, volume)
	pvSum := computeMovingSum(pv, period)
	volSum := computeMovingSum(volume, period)
	result := divide(pvSum, volSum)

	return singleResult(result), nil
}

func computeAPO(_ context.Context, input *Input, params []int) ([]Result, error) {
	fastPeriod := intParam(params, 0, 14)
	slowPeriod := intParam(params, 1, 30)
	close := input.Close

	fastEMA := computeEMASlice(close, fastPeriod)
	slowEMA := computeEMASlice(close, slowPeriod)

	result := subtract(fastEMA, slowEMA)
	return singleResult(result), nil
}

func computeROC(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 9)
	close := input.Close
	n := len(close)

	result := make([]float64, n)
	for i := period; i < n; i++ {
		if close[i-period] != 0 {
			result[i] = (close[i] - close[i-period]) / close[i-period] * 100.0
		}
	}
	return singleResult(result), nil
}

func computeAroon(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 25)
	high := input.High
	low := input.Low
	n := len(high)

	aroonUp := make([]float64, n)
	aroonDown := make([]float64, n)

	for i := period; i < n; i++ {
		// Find highest high and lowest low in window
		highIdx := i - period
		lowIdx := i - period
		for j := i - period; j <= i; j++ {
			if high[j] >= high[highIdx] {
				highIdx = j
			}
			if low[j] <= low[lowIdx] {
				lowIdx = j
			}
		}
		aroonUp[i] = float64(period-(i-highIdx)) / float64(period) * 100.0
		aroonDown[i] = float64(period-(i-lowIdx)) / float64(period) * 100.0
	}

	return roundRobinMulti([]string{"up", "down"}, aroonUp, aroonDown), nil
}

func computeBOP(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	open := input.Open
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		hl := high[i] - low[i]
		if hl != 0 {
			result[i] = (close[i] - open[i]) / hl
		}
	}
	return singleResult(result), nil
}

func computeCCI(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	// Typical Price
	tp := make([]float64, n)
	for i := 0; i < n; i++ {
		tp[i] = (high[i] + low[i] + close[i]) / 3.0
	}

	smaTP := computeSMASlice(tp, period)
	meanDev := make([]float64, n)

	// Mean deviation (not std dev)
	for i := period - 1; i < n; i++ {
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			diff := tp[j] - smaTP[i]
			if diff < 0 {
				diff = -diff
			}
			sum += diff
		}
		meanDev[i] = sum / float64(period)
	}

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		if meanDev[i] != 0 {
			result[i] = (tp[i] - smaTP[i]) / (0.015 * meanDev[i])
		}
	}
	return singleResult(result), nil
}

func computeCFO(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	close := input.Close
	n := len(close)

	// Chande Forecast Oscillator
	// CFO = (close - SMA(close, period)) / SMA(close, period) * 100
	sma := computeSMASlice(close, period)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		if sma[i] != 0 {
			result[i] = (close[i] - sma[i]) / sma[i] * 100.0
		}
	}
	return singleResult(result), nil
}

func computeDEMA(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close

	ema1 := computeEMASlice(close, period)
	ema2 := computeEMASlice(ema1, period)

	// DEMA = 2*EMA - EMA(EMA)
	result := make([]float64, len(close))
	for i := 0; i < len(close); i++ {
		result[i] = 2*ema1[i] - ema2[i]
	}
	return singleResult(result), nil
}

func computeDPO(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close

	sma := computeSMASlice(close, period)
	shift := period/2 + 1

	// DPO = Close - SMA(shifted back by period/2 + 1)
	result := make([]float64, len(close))
	for i := shift; i < len(close); i++ {
		result[i] = close[i-shift] - sma[i]
	}
	return singleResult(result), nil
}

func computeEnvelope(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	pct := intParam(params, 1, 5) // default 5%
	close := input.Close
	n := len(close)

	sma := computeSMASlice(close, period)
	factor := float64(pct) / 100.0

	upper := make([]float64, n)
	lower := make([]float64, n)
	for i := 0; i < n; i++ {
		upper[i] = sma[i] * (1 + factor)
		lower[i] = sma[i] * (1 - factor)
	}

	return roundRobinMulti([]string{"upper", "middle", "lower"}, upper, sma, lower), nil
}

func computeHMA(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close
	n := len(close)

	// HMA = WMA(2*WMA(close, n/2) - WMA(close, n), sqrt(n))
	halfPeriod := period / 2
	sqrtPeriod := int(math.Sqrt(float64(period)))
	if sqrtPeriod < 1 {
		sqrtPeriod = 1
	}

	wmaHalf := computeWMASlice(close, halfPeriod)
	wmaFull := computeWMASlice(close, period)

	diff := make([]float64, n)
	for i := 0; i < n; i++ {
		diff[i] = 2*wmaHalf[i] - wmaFull[i]
	}

	result := computeWMASlice(diff, sqrtPeriod)
	return singleResult(result), nil
}

// computeWMASlice computes Weighted Moving Average.
func computeWMASlice(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)
	if n == 0 || period <= 0 {
		return result
	}

	weightSum := float64(period * (period + 1) / 2)

	for i := period - 1; i < n; i++ {
		wsum := 0.0
		for j := 0; j < period; j++ {
			wsum += data[i-period+1+j] * float64(j+1)
		}
		result[i] = wsum / weightSum
	}
	return result
}

func computeKAMA(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 30)
	fastEnd := intParam(params, 1, 2)
	slowEnd := intParam(params, 2, 30)
	close := input.Close
	n := len(close)

	result := make([]float64, n)
	if n < period {
		return singleResult(result), nil
	}

	fastAlpha := 2.0 / float64(fastEnd+1)
	slowAlpha := 2.0 / float64(slowEnd+1)

	// Initial SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += close[i]
	}
	result[period-1] = sum / float64(period)

	change := make([]float64, n)
	for i := period; i < n; i++ {
		change[i] = math.Abs(close[i] - close[i-period])
	}

	volatility := make([]float64, n)
	for i := period; i < n; i++ {
		v := 0.0
		for j := i - period + 1; j <= i; j++ {
			v += math.Abs(close[j] - close[j-1])
		}
		volatility[i] = v
	}

	for i := period; i < n; i++ {
		er := change[i] / volatility[i]
		if volatility[i] == 0 {
			er = 0
		}
		sc := er*(fastAlpha-slowAlpha) + slowAlpha
		sc = sc * sc // squared
		result[i] = result[i-1] + sc*(close[i]-result[i-1])
	}

	return singleResult(result), nil
}

func computeKDJ(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 9)
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	// KDJ (Random Index): K, D, J
	// RSV = (close - min(low, period)) / (max(high, period) - min(low, period)) * 100
	// K = 2/3 * prevK + 1/3 * RSV
	// D = 2/3 * prevD + 1/3 * K
	// J = 3*K - 2*D

	minLow := computeMovingMin(low, period)
	maxHigh := computeMovingMax(high, period)

	rsv := make([]float64, n)
	for i := period - 1; i < n; i++ {
		hh := maxHigh[i]
		ll := minLow[i]
		if hh != ll {
			rsv[i] = (close[i] - ll) / (hh - ll) * 100.0
		} else {
			rsv[i] = 50.0 // midpoint when range is 0
		}
	}

	k := make([]float64, n)
	d := make([]float64, n)
	j := make([]float64, n)

	for i := period - 1; i < n; i++ {
		if i == period-1 {
			k[i] = rsv[i]
			d[i] = rsv[i]
		} else {
			k[i] = (2.0/3.0)*k[i-1] + (1.0/3.0)*rsv[i]
			d[i] = (2.0/3.0)*d[i-1] + (1.0/3.0)*k[i]
		}
		j[i] = 3*k[i] - 2*d[i]
	}

	return roundRobinMulti([]string{"k", "d", "j"}, k, d, j), nil
}

func computeKST(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	close := input.Close

	// KST uses fixed periods: 10,15,20,30 with SMAs 10,10,10,15
	r1 := computeROCIntegral(close, 10, 10)
	r2 := computeROCIntegral(close, 15, 10)
	r3 := computeROCIntegral(close, 20, 10)
	r4 := computeROCIntegral(close, 30, 15)

	// KST = 1*R1 + 2*R2 + 3*R3 + 4*R4
	n := len(close)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = r1[i] + 2*r2[i] + 3*r3[i] + 4*r4[i]
	}
	return singleResult(result), nil
}

// computeROCIntegral computes ROC(rocPeriod) then SMA(smaPeriod).
func computeROCIntegral(data []float64, rocPeriod, smaPeriod int) []float64 {
	n := len(data)
	roc := make([]float64, n)
	for i := rocPeriod; i < n; i++ {
		if data[i-rocPeriod] != 0 {
			roc[i] = (data[i] - data[i-rocPeriod]) / data[i-rocPeriod] * 100.0
		}
	}
	return computeSMASlice(roc, smaPeriod)
}

func computeMassIndex(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 25)
	high := input.High
	low := input.Low
	n := len(high)

	// Mass Index: sum of EMA9(high-low) / EMA9(EMA9(high-low)) over period
	hl := make([]float64, n)
	for i := 0; i < n; i++ {
		hl[i] = high[i] - low[i]
	}

	ema9 := computeEMASlice(hl, 9)
	emaEma9 := computeEMASlice(ema9, 9)

	ratio := make([]float64, n)
	for i := 0; i < n; i++ {
		if emaEma9[i] != 0 {
			ratio[i] = ema9[i] / emaEma9[i]
		}
	}

	result := computeMovingSum(ratio, period)
	return singleResult(result), nil
}

func computeMcGinley(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close
	n := len(close)

	result := make([]float64, n)
	if n < period {
		return singleResult(result), nil
	}

	// Initial SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += close[i]
	}
	result[period-1] = sum / float64(period)

	for i := period; i < n; i++ {
		// McGinley Dynamic = MG[i-1] + (close - MG[i-1]) / (k * period * (close/MG[i-1])^4)
		ratio := close[i] / result[i-1]
		k := 0.6
		denom := k * float64(period) * ratio * ratio * ratio * ratio
		if denom == 0 {
			result[i] = result[i-1]
		} else {
			result[i] = result[i-1] + (close[i]-result[i-1])/denom
		}
	}
	return singleResult(result), nil
}

func computeMLR(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close
	n := len(close)

	result := make([]float64, n)

	for i := period - 1; i < n; i++ {
		// Linear regression: y = a + bx
		sumX := 0.0
		sumY := 0.0
		sumXY := 0.0
		sumX2 := 0.0
		for j := 0; j < period; j++ {
			x := float64(j + 1)
			y := close[i-period+1+j]
			sumX += x
			sumY += y
			sumXY += x * y
			sumX2 += x * x
		}
		slope := (float64(period)*sumXY - sumX*sumY) / (float64(period)*sumX2 - sumX*sumX)
		intercept := (sumY - slope*sumX) / float64(period)
		result[i] = intercept + slope*float64(period)
	}
	return singleResult(result), nil
}

func computeMLS(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close
	n := len(close)

	// Moving Least Square = SMA + slope*(period/2)
	sma := computeSMASlice(close, period)
	slope := computeSlopeSlice(close, period)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = sma[i] + slope[i]*float64(period-1)/2.0
	}
	return singleResult(result), nil
}

// computeSlopeSlice computes the slope of linear regression over a window.
func computeSlopeSlice(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)

	for i := period - 1; i < n; i++ {
		sumX := 0.0
		sumY := 0.0
		sumXY := 0.0
		sumX2 := 0.0
		for j := 0; j < period; j++ {
			x := float64(j + 1)
			y := data[i-period+1+j]
			sumX += x
			sumY += y
			sumXY += x * y
			sumX2 += x * x
		}
		denom := float64(period)*sumX2 - sumX*sumX
		if denom != 0 {
			result[i] = (float64(period)*sumXY - sumX*sumY) / denom
		}
	}
	return result
}

func computeMovingMaxIndicator(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	return singleResult(computeMovingMax(input.Close, period)), nil
}

func computeMovingMinIndicator(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	return singleResult(computeMovingMin(input.Close, period)), nil
}

func computeMovingSumIndicator(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	return singleResult(computeMovingSum(input.Close, period)), nil
}

func computePivotPoint(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	pp := make([]float64, n)
	r1 := make([]float64, n)
	r2 := make([]float64, n)
	s1 := make([]float64, n)
	s2 := make([]float64, n)

	for i := 1; i < n; i++ {
		pp[i] = (high[i-1] + low[i-1] + close[i-1]) / 3.0
		r1[i] = 2*pp[i] - low[i-1]
		r2[i] = pp[i] + high[i-1] - low[i-1]
		s1[i] = 2*pp[i] - high[i-1]
		s2[i] = pp[i] - high[i-1] + low[i-1]
	}

	return roundRobinMulti([]string{"pp", "r1", "r2", "s1", "s2"}, pp, r1, r2, s1, s2), nil
}

func computeRMA(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	return singleResult(computeRMASlice(input.Close, period)), nil
}

func computeSlope(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	return singleResult(computeSlopeSlice(input.Close, period)), nil
}

func computeSlowStochastic(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	smaPeriod := intParam(params, 1, 3)
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	// Slow Stochastic = SMA of Fast Stochastic K, then SMA again for D
	minLow := computeMovingMin(low, period)
	maxHigh := computeMovingMax(high, period)

	fastK := make([]float64, n)
	for i := period - 1; i < n; i++ {
		hh := maxHigh[i]
		ll := minLow[i]
		if hh != ll {
			fastK[i] = (close[i] - ll) / (hh - ll) * 100.0
		}
	}

	k := computeSMASlice(fastK, smaPeriod)
	d := computeSMASlice(k, smaPeriod)

	return roundRobinMulti([]string{"k", "d"}, k, d), nil
}

func computeSMMA(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close
	n := len(close)

	// Smoothed Moving Average
	result := make([]float64, n)
	if n < period {
		return singleResult(result), nil
	}

	// First value = SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += close[i]
	}
	result[period-1] = sum / float64(period)

	for i := period; i < n; i++ {
		result[i] = (result[i-1]*float64(period-1) + close[i]) / float64(period)
	}
	return singleResult(result), nil
}

func computeSTC(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 23)
	fastPeriod := intParam(params, 1, 2)
	slowPeriod := intParam(params, 2, 5)
	close := input.Close
	n := len(close)

	if n < period+5 {
		return singleResult(make([]float64, n)), nil
	}

	// Calculate MACD using fast and slow EMAs
	fastEMA := computeEMASlice(close, fastPeriod)
	slowEMA := computeEMASlice(close, slowPeriod)

	macd := make([]float64, n)
	for i := 0; i < n; i++ {
		macd[i] = fastEMA[i] - slowEMA[i]
	}

	// Stochastic K on MACD over period
	minMacd := computeMovingMin(macd, period)
	maxMacd := computeMovingMax(macd, period)

	stochK := make([]float64, n)
	for i := period - 1; i < n; i++ {
		if maxMacd[i] != minMacd[i] {
			stochK[i] = (macd[i] - minMacd[i]) / (maxMacd[i] - minMacd[i]) * 100.0
		}
	}

	// Double smooth with EMA
	ema1 := computeEMASlice(stochK, int(math.Round(float64(period)/2.0)))
	ema2 := computeEMASlice(ema1, int(math.Round(float64(period)/2.0)))

	// Scale back to 0-100
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = ema2[i]
	}
	return singleResult(result), nil
}

func computeT3(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close

	// T3 = GD(GD(GD(close)))
	// GD = EMA + v * (EMA - EMA(EMA))
	// where GD is generalized DEMA with v factor
	vFactor := 0.7

	gd1 := computeGD(close, period, vFactor)
	gd2 := computeGD(gd1, period, vFactor)
	gd3 := computeGD(gd2, period, vFactor)

	return singleResult(gd3), nil
}

// computeGD computes Generalized DEMA: GD = EMA + v * (EMA - EMA(EMA))
func computeGD(data []float64, period int, v float64) []float64 {
	ema1 := computeEMASlice(data, period)
	ema2 := computeEMASlice(ema1, period)

	n := len(data)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = ema1[i] + v*(ema1[i]-ema2[i])
	}
	return result
}

func computeTEMA(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close
	n := len(close)

	// TEMA = 3*EMA - 3*EMA(EMA) + EMA(EMA(EMA))
	ema1 := computeEMASlice(close, period)
	ema2 := computeEMASlice(ema1, period)
	ema3 := computeEMASlice(ema2, period)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = 3*ema1[i] - 3*ema2[i] + ema3[i]
	}
	return singleResult(result), nil
}

func computeTRIMA(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close

	// Triangular Moving Average = SMA(SMA(close, ceil(period/2)), floor(period/2)+1)
	half := (period + 1) / 2
	sma1 := computeSMASlice(close, half)
	result := computeSMASlice(sma1, period-half+1)
	return singleResult(result), nil
}

func computeTRIX(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	close := input.Close

	// TRIX = ROC(EMA(EMA(EMA(close))))
	ema1 := computeEMASlice(close, period)
	ema2 := computeEMASlice(ema1, period)
	ema3 := computeEMASlice(ema2, period)

	n := len(close)
	result := make([]float64, n)
	for i := period; i < n; i++ {
		if ema3[i-1] != 0 {
			result[i] = (ema3[i] - ema3[i-1]) / ema3[i-1] * 100.0
		}
	}
	return singleResult(result), nil
}

func computeTSI(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	close := input.Close
	n := len(close)

	// TSI = EMA(EMA(momentum)) / EMA(EMA(abs(momentum))) * 100
	// where momentum = close[i] - close[i-1]
	mom := make([]float64, n)
	absMom := make([]float64, n)
	for i := 1; i < n; i++ {
		mom[i] = close[i] - close[i-1]
		absMom[i] = math.Abs(mom[i])
	}

	// Default periods: 25 for first EMA, 13 for second EMA
	ema25 := computeEMASlice(mom, 25)
	dblEma25 := computeEMASlice(ema25, 13)

	absEma25 := computeEMASlice(absMom, 25)
	dblAbsEma25 := computeEMASlice(absEma25, 13)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		if dblAbsEma25[i] != 0 {
			result[i] = dblEma25[i] / dblAbsEma25[i] * 100.0
		}
	}
	return singleResult(result), nil
}

func computeTypicalPrice(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = (high[i] + low[i] + close[i]) / 3.0
	}
	return singleResult(result), nil
}

func computeWeightedClose(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		// Weighted Close = (High + Low + 2*Close) / 4
		result[i] = (high[i] + low[i] + 2*close[i]) / 4.0
	}
	return singleResult(result), nil
}

func computeWMA(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	return singleResult(computeWMASlice(input.Close, period)), nil
}

func computeMA(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	return singleResult(computeSMASlice(input.Close, period)), nil
}

func computeStochastic(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	smaPeriod := intParam(params, 1, 3)
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	minLow := computeMovingMin(low, period)
	maxHigh := computeMovingMax(high, period)

	k := make([]float64, n)
	for i := period - 1; i < n; i++ {
		hh := maxHigh[i]
		ll := minLow[i]
		if hh != ll {
			k[i] = (close[i] - ll) / (hh - ll) * 100.0
		}
	}

	d := computeSMASlice(k, smaPeriod)

	return roundRobinMulti([]string{"k", "d"}, k, d), nil
}
