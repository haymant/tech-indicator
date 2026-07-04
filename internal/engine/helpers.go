package engine

import "math"

// intParam safely extracts the i-th parameter or returns a default value.
func intParam(params []int, i, def int) int {
	if i < len(params) {
		return params[i]
	}
	return def
}

// computeSMASlice computes Simple Moving Average directly from a slice.
// O(n) single pass with sliding window. First (period-1) values are 0.
func computeSMASlice(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)
	if n == 0 || period <= 0 {
		return result
	}

	sum := 0.0
	for i := 0; i < n; i++ {
		sum += data[i]
		if i >= period {
			sum -= data[i-period]
		}
		if i >= period-1 {
			result[i] = sum / float64(period)
		}
	}
	return result
}

// computeEMASlice computes Exponential Moving Average directly from a slice.
// Initial value is SMA of first `period` values, then EMA formula.
// Multiplier = 2 / (period + 1). First (period-1) values are 0.
func computeEMASlice(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)
	if n == 0 || period <= 0 {
		return result
	}

	multiplier := 2.0 / float64(period+1)

	// Initial SMA value
	sum := 0.0
	for i := 0; i < period && i < n; i++ {
		sum += data[i]
	}
	if n >= period {
		result[period-1] = sum / float64(period)

		// EMA for remaining
		for i := period; i < n; i++ {
			result[i] = (data[i]-result[i-1])*multiplier + result[i-1]
		}
	}

	return result
}

// computeRMASlice computes Rolling Moving Average (Wilder's smoothing).
// RMA is like EMA with alpha = 1/period.
// R[0..p-1] = SMA(values), R[p..] = (R[i-1]*(p-1) + v[i]) / p.
func computeRMASlice(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)
	if n == 0 || period <= 0 {
		return result
	}

	// Initial SMA value
	sum := 0.0
	for i := 0; i < period && i < n; i++ {
		sum += data[i]
	}
	if n >= period {
		result[period-1] = sum / float64(period)

		factor := float64(period-1) / float64(period)
		invPeriod := 1.0 / float64(period)

		for i := period; i < n; i++ {
			result[i] = result[i-1]*factor + data[i]*invPeriod
		}
	}

	return result
}

// computeMovingStd computes rolling standard deviation over a window.
// Uses the population formula: sqrt(mean((x - mean)^2)).
// First (period-1) values are 0. O(n) pass with sliding window.
func computeMovingStd(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)
	if n == 0 || period <= 0 {
		return result
	}

	// Ring buffer of last `period` values
	buf := make([]float64, period)
	sum := 0.0
	idx := 0
	count := 0

	for i := 0; i < n; i++ {
		if count < period {
			buf[count] = data[i]
			sum += data[i]
			count++
		} else {
			old := buf[idx]
			buf[idx] = data[i]
			sum += data[i] - old
			idx = (idx + 1) % period
		}

		if count == period {
			mean := sum / float64(period)
			sqSum := 0.0
			for j := 0; j < period; j++ {
				diff := buf[j] - mean
				sqSum += diff * diff
			}
			result[i] = math.Sqrt(sqSum / float64(period))
		}
	}

	return result
}

// computeMovingSum computes rolling sum over a window.
func computeMovingSum(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)
	if n == 0 || period <= 0 {
		return result
	}

	sum := 0.0
	for i := 0; i < n; i++ {
		sum += data[i]
		if i >= period {
			sum -= data[i-period]
		}
		if i >= period-1 {
			result[i] = sum
		}
	}
	return result
}

// computeMovingMax computes rolling maximum over a window.
func computeMovingMax(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)
	if n == 0 || period <= 0 {
		return result
	}

	for i := 0; i < n; i++ {
		if i < period-1 {
			continue
		}
		maxVal := data[i-period+1]
		for j := i - period + 2; j <= i; j++ {
			if data[j] > maxVal {
				maxVal = data[j]
			}
		}
		result[i] = maxVal
	}
	return result
}

// computeMovingMin computes rolling minimum over a window.
func computeMovingMin(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)
	if n == 0 || period <= 0 {
		return result
	}

	for i := 0; i < n; i++ {
		if i < period-1 {
			continue
		}
		minVal := data[i-period+1]
		for j := i - period + 2; j <= i; j++ {
			if data[j] < minVal {
				minVal = data[j]
			}
		}
		result[i] = minVal
	}
	return result
}

// computeChange computes the period-over-period change: data[i] - data[i-period].
func computeChange(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)
	if n == 0 {
		return result
	}
	for i := period; i < n; i++ {
		result[i] = data[i] - data[i-period]
	}
	return result
}

// computeChangePercent computes the percentage change: (data[i] - data[i-1]) / data[i-1] * 100.
func computeChangePercent(data []float64) []float64 {
	n := len(data)
	result := make([]float64, n)
	for i := 1; i < n; i++ {
		if data[i-1] != 0 {
			result[i] = (data[i] - data[i-1]) / data[i-1] * 100.0
		}
	}
	return result
}

// keepPositives returns a slice where negative values are set to 0.
func keepPositives(data []float64) []float64 {
	n := len(data)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		if data[i] > 0 {
			result[i] = data[i]
		}
	}
	return result
}

// keepNegatives returns a slice where positive values are set to 0 (negatives as positive).
func keepNegatives(data []float64) []float64 {
	n := len(data)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		if data[i] < 0 {
			result[i] = -data[i]
		}
	}
	return result
}

// multiplyBy multiplies every element by a constant.
func multiplyBy(data []float64, factor float64) []float64 {
	n := len(data)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = data[i] * factor
	}
	return result
}

// divideBy divides every element by a constant.
func divideBy(data []float64, divisor float64) []float64 {
	n := len(data)
	result := make([]float64, n)
	if divisor == 0 {
		return result
	}
	for i := 0; i < n; i++ {
		result[i] = data[i] / divisor
	}
	return result
}

// add adds two slices element-wise.
func add(a, b []float64) []float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = a[i] + b[i]
	}
	return result
}

// subtract subtracts b from a element-wise.
func subtract(a, b []float64) []float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = a[i] - b[i]
	}
	return result
}

// multiply multiplies two slices element-wise.
func multiply(a, b []float64) []float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = a[i] * b[i]
	}
	return result
}

// divide divides a by b element-wise.
func divide(a, b []float64) []float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		if b[i] != 0 {
			result[i] = a[i] / b[i]
		}
	}
	return result
}

// incrementBy adds a constant to every element.
func incrementBy(data []float64, val float64) []float64 {
	n := len(data)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = data[i] + val
	}
	return result
}

// abs computes absolute value of every element.
func absSlice(data []float64) []float64 {
	n := len(data)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = math.Abs(data[i])
	}
	return result
}

// maxOf computes element-wise max of two slices.
func maxOf(a, b []float64) []float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		if a[i] > b[i] {
			result[i] = a[i]
		} else {
			result[i] = b[i]
		}
	}
	return result
}

// minOf computes element-wise min of two slices.
func minOf(a, b []float64) []float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		if a[i] < b[i] {
			result[i] = a[i]
		} else {
			result[i] = b[i]
		}
	}
	return result
}

// roundRobinMulti wraps multiple slices into named Results.
func roundRobinMulti(names []string, slices ...[]float64) []Result {
	if len(names) != len(slices) {
		panic("roundRobinMulti: names and slices length mismatch")
	}
	results := make([]Result, len(slices))
	for i, name := range names {
		results[i] = Result{SubIndicator: name, Values: slices[i]}
	}
	return results
}

// singleResult wraps a single slice into a Result list.
func singleResult(values []float64) []Result {
	return []Result{{Values: values}}
}
