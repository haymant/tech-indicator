package engine

import (
	"context"
	"math"
)

func init() {
	registerValuation()
}

func registerValuation() {
	batchRegistry["fv"] = computeFV
	batchRegistry["npv"] = computeNPV
	batchRegistry["pv"] = computePV
}

func computeFV(_ context.Context, input *Input, params []int) ([]Result, error) {
	rate := float64(intParam(params, 0, 10)) / 100.0 // default 10%
	nper := intParam(params, 1, 5)
	pmt := float64(intParam(params, 2, 0))
	pv := float64(intParam(params, 3, 1000))

	// FV = PV * (1 + rate)^nper + PMT * ((1 + rate)^nper - 1) / rate
	compounded := math.Pow(1+rate, float64(nper))
	fv := pv*compounded + pmt*(compounded-1)/rate

	n := len(input.Close)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = fv
	}
	return singleResult(result), nil
}

func computeNPV(_ context.Context, input *Input, params []int) ([]Result, error) {
	rate := float64(intParam(params, 0, 10)) / 100.0 // default 10%
	_ = intParam(params, 1, 0)                       // initial investment (handled separately)

	// NPV = sum(CF_t / (1+rate)^t)
	close := input.Close
	n := len(close)

	result := make([]float64, n)
	npv := 0.0
	for i := 0; i < n; i++ {
		npv += close[i] / math.Pow(1+rate, float64(i+1))
		result[i] = npv
	}
	return singleResult(result), nil
}

func computePV(_ context.Context, input *Input, params []int) ([]Result, error) {
	rate := float64(intParam(params, 0, 10)) / 100.0 // default 10%
	nper := intParam(params, 1, 5)
	_ = intParam(params, 2, 0) // pmt (not used in PV calc)
	fv := float64(intParam(params, 3, 1000))

	// PV = FV / (1 + rate)^nper
	pv := fv / math.Pow(1+rate, float64(nper))

	n := len(input.Close)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = pv
	}
	return singleResult(result), nil
}

func computeClose(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	n := len(input.Close)
	result := make([]float64, n)
	copy(result, input.Close)
	return singleResult(result), nil
}

func computeHigh(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	n := len(input.High)
	result := make([]float64, n)
	copy(result, input.High)
	return singleResult(result), nil
}

func computeLow(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	n := len(input.Low)
	result := make([]float64, n)
	copy(result, input.Low)
	return singleResult(result), nil
}

func computeOpen(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	n := len(input.Open)
	result := make([]float64, n)
	copy(result, input.Open)
	return singleResult(result), nil
}

func computeVolume(_ context.Context, input *Input, params []int) ([]Result, error) {
	_ = params
	n := len(input.Volume)
	result := make([]float64, n)
	copy(result, input.Volume)
	return singleResult(result), nil
}
