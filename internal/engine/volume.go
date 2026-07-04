package engine

import (
	"context"
	"math"
)

func init() {
	registerVolume()
}

func registerVolume() {
	batchRegistry["obv"] = computeOBV
	batchRegistry["ad"] = computeAD
	batchRegistry["cmf"] = computeCMF
	batchRegistry["emv"] = computeEMV
	batchRegistry["fi"] = computeFI
	batchRegistry["kvo"] = computeKVO
	batchRegistry["mfi"] = computeMFI
	batchRegistry["mfm"] = computeMFM
	batchRegistry["mfv"] = computeMFV
	batchRegistry["nvi"] = computeNVI
	batchRegistry["vpt"] = computeVPT
	batchRegistry["vwap"] = computeVWAP
}

func computeOBV(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	close := input.Close
	volume := input.Volume
	n := len(close)

	result := make([]float64, n)
	var prevClose float64
	var obv float64

	for i := 0; i < n; i++ {
		if i == 0 {
			obv = volume[i]
		} else if close[i] > prevClose {
			obv += volume[i]
		} else if close[i] < prevClose {
			obv -= volume[i]
		}
		result[i] = obv
		prevClose = close[i]
	}
	return singleResult(result), nil
}

func computeAD(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	high := input.High
	low := input.Low
	close := input.Close
	volume := input.Volume
	n := len(close)

	result := make([]float64, n)
	ad := 0.0

	for i := 0; i < n; i++ {
		hl := high[i] - low[i]
		if hl != 0 {
			mfm := ((close[i] - low[i]) - (high[i] - close[i])) / hl
			mfv := mfm * volume[i]
			ad += mfv
		}
		result[i] = ad
	}
	return singleResult(result), nil
}

func computeCMF(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 20)
	high := input.High
	low := input.Low
	close := input.Close
	volume := input.Volume
	n := len(close)

	// CMF = sum(MFV, period) / sum(Volume, period)
	mfv := make([]float64, n)
	for i := 0; i < n; i++ {
		hl := high[i] - low[i]
		if hl != 0 {
			mfm := ((close[i] - low[i]) - (high[i] - close[i])) / hl
			mfv[i] = mfm * volume[i]
		}
	}

	mfvSum := computeMovingSum(mfv, period)
	volSum := computeMovingSum(volume, period)

	result := divide(mfvSum, volSum)
	return singleResult(result), nil
}

func computeEMV(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	high := input.High
	low := input.Low
	volume := input.Volume
	n := len(high)

	// EMV = ((high + low)/2 - (prevHigh + prevLow)/2) / (volume / 1000000 / (high - low))
	// Simplified: EMV = MA(volume/HL ratio adjusted midpoint change)
	ratio := make([]float64, n)
	for i := 1; i < n; i++ {
		mp := (high[i] + low[i]) / 2.0
		prevMp := (high[i-1] + low[i-1]) / 2.0
		hl := high[i] - low[i]
		if hl != 0 && volume[i] != 0 {
			brokerage := volume[i] / 1000000.0 / hl
			if brokerage != 0 {
				ratio[i] = (mp - prevMp) / brokerage
			}
		}
	}

	result := computeSMASlice(ratio, period)
	return singleResult(result), nil
}

func computeFI(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 13)
	close := input.Close
	volume := input.Volume
	n := len(close)

	// Force Index = EMA(volume * (close - prevClose))
	raw := make([]float64, n)
	for i := 1; i < n; i++ {
		raw[i] = volume[i] * (close[i] - close[i-1])
	}

	result := computeEMASlice(raw, period)
	return singleResult(result), nil
}

func computeKVO(_ context.Context, input *Input, params []int) ([]Result, error) {
	fastPeriod := intParam(params, 0, 34)
	slowPeriod := intParam(params, 1, 55)
	high := input.High
	low := input.Low
	close := input.Close
	volume := input.Volume
	n := len(high)

	// KVO = EMA(volume * trend, fast) - EMA(volume * trend, slow)
	// trend = (high + low + close) / 3 - (prevHigh + prevLow + prevClose) / 3
	trend := make([]float64, n)
	vpt := make([]float64, n)
	for i := 1; i < n; i++ {
		vp := (high[i] + low[i] + close[i]) / 3.0
		prevVp := (high[i-1] + low[i-1] + close[i-1]) / 3.0
		trend[i] = vp - prevVp
		vpt[i] = volume[i] * trend[i]
	}

	emaFast := computeEMASlice(vpt, fastPeriod)
	emaSlow := computeEMASlice(vpt, slowPeriod)

	result := subtract(emaFast, emaSlow)
	return singleResult(result), nil
}

func computeMFI(_ context.Context, input *Input, params []int) ([]Result, error) {
	period := intParam(params, 0, 14)
	high := input.High
	low := input.Low
	close := input.Close
	volume := input.Volume
	n := len(close)

	// Money Flow Index
	tp := make([]float64, n)
	rawMF := make([]float64, n)
	for i := 0; i < n; i++ {
		tp[i] = (high[i] + low[i] + close[i]) / 3.0
		rawMF[i] = tp[i] * volume[i]
	}

	posMF := make([]float64, n)
	negMF := make([]float64, n)
	for i := 1; i < n; i++ {
		if tp[i] > tp[i-1] {
			posMF[i] = rawMF[i]
		} else if tp[i] < tp[i-1] {
			negMF[i] = rawMF[i]
		}
	}

	posSum := computeMovingSum(posMF, period)
	negSum := computeMovingSum(negMF, period)

	result := make([]float64, n)
	for i := period - 1; i < n; i++ {
		if negSum[i] != 0 {
			mfr := posSum[i] / negSum[i]
			result[i] = 100.0 - (100.0 / (1.0 + mfr))
		}
	}
	return singleResult(result), nil
}

func computeMFM(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	high := input.High
	low := input.Low
	close := input.Close
	n := len(close)

	// Money Flow Multiplier = ((close - low) - (high - close)) / (high - low)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		hl := high[i] - low[i]
		if hl != 0 {
			result[i] = ((close[i] - low[i]) - (high[i] - close[i])) / hl
		}
	}
	return singleResult(result), nil
}

func computeMFV(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	high := input.High
	low := input.Low
	close := input.Close
	volume := input.Volume
	n := len(close)

	// Money Flow Volume = MFM * Volume
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		hl := high[i] - low[i]
		if hl != 0 {
			mfm := ((close[i] - low[i]) - (high[i] - close[i])) / hl
			result[i] = mfm * volume[i]
		}
	}
	return singleResult(result), nil
}

func computeNVI(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	close := input.Close
	volume := input.Volume
	n := len(close)

	// Negative Volume Index: only changes on down-volume days
	result := make([]float64, n)
	nvi := 1000.0
	result[0] = nvi

	for i := 1; i < n; i++ {
		if volume[i] < volume[i-1] {
			pctChange := (close[i] - close[i-1]) / close[i-1] * 100.0
			nvi += pctChange
		}
		result[i] = nvi
	}
	return singleResult(result), nil
}

func computeVPT(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	close := input.Close
	volume := input.Volume
	n := len(close)

	// Volume Price Trend = cumulative sum of volume * %price change
	result := make([]float64, n)
	vpt := 0.0

	for i := 0; i < n; i++ {
		if i > 0 && close[i-1] != 0 {
			pctChange := (close[i] - close[i-1]) / close[i-1]
			vpt += pctChange * volume[i]
		}
		result[i] = vpt
	}
	return singleResult(result), nil
}

func computeVWAP(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	high := input.High
	low := input.Low
	close := input.Close
	volume := input.Volume
	n := len(close)

	// VWAP = cumulative sum(TP * volume) / cumulative sum(volume)
	// TP = (high + low + close) / 3
	result := make([]float64, n)
	cumPV := 0.0
	cumVol := 0.0

	for i := 0; i < n; i++ {
		tp := (high[i] + low[i] + close[i]) / 3.0
		cumPV += tp * volume[i]
		cumVol += volume[i]
		if cumVol != 0 {
			result[i] = cumPV / cumVol
		}
	}
	return singleResult(result), nil
}

func computeLogReturn(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	close := input.Close
	n := len(close)

	result := make([]float64, n)
	for i := 1; i < n; i++ {
		if close[i-1] > 0 {
			result[i] = math.Log(close[i] / close[i-1])
		}
	}
	return singleResult(result), nil
}
