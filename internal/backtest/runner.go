package backtest

import (
	"context"
	"math"
	"time"

	"github.com/cinar/indicator/v2/asset"
	"github.com/cinar/indicator/v2/helper"
	cindicator "github.com/cinar/indicator/v2/strategy"
)

// Result holds the performance/risk metrics from a backtest run.
type Result struct {
	StrategyID      int
	StrategyType    string
	Underlying      string
	StartDate       time.Time
	EndDate         time.Time
	TotalReturn     float64
	MaxDrawdown     float64
	SharpeRatio     float64
	WinRate         float64
	NumTransactions int
	FinalOutcome    float64
	FinalAction     string
	Parameters      map[string]any
}

// Run executes a backtest for a single strategy + asset and returns performance/risk metrics.
// Uses a simple approach: feed snapshots through the strategy, collecting actions directly.
func Run(
	ctx context.Context,
	st cindicator.Strategy,
	snapshots []*asset.Snapshot,
	strategyID int,
	strategyType string,
	underlying string,
	params map[string]any,
) (*Result, error) {
	if len(snapshots) == 0 {
		return nil, nil
	}

	// Feed snapshots to the strategy.
	snapshotChan := helper.SliceToChan(snapshots)
	actions := st.Compute(snapshotChan)

	// Read all actions.
	var actionSlice []cindicator.Action
	for a := range actions {
		actionSlice = append(actionSlice, a)
	}

	// Compute outcome by simulating buy/sell on close prices.
	outcomeSlice := simulateOutcome(snapshots, actionSlice)

	totalReturn := 0.0
	if len(outcomeSlice) > 0 {
		totalReturn = outcomeSlice[len(outcomeSlice)-1]
	}

	return &Result{
		StrategyID:      strategyID,
		StrategyType:    strategyType,
		Underlying:      underlying,
		StartDate:       snapshots[0].Date,
		EndDate:         snapshots[len(snapshots)-1].Date,
		TotalReturn:     totalReturn,
		MaxDrawdown:     computeMaxDrawdown(outcomeSlice),
		SharpeRatio:     computeSharpeRatio(outcomeSlice),
		WinRate:         computeWinRate(actionSlice),
		NumTransactions: countTransactions(actionSlice),
		FinalOutcome:    totalReturn,
		FinalAction:     actionFromSlice(actionSlice),
		Parameters:      params,
	}, nil
}

// simulateOutcome computes P&L by simulating buy/sell actions against close prices.
// Starts with $1, buys at close on Buy action, sells at close on Sell action.
func simulateOutcome(snapshots []*asset.Snapshot, actions []cindicator.Action) []float64 {
	outcomes := make([]float64, 0, len(actions))
	balance := 1.0
	shares := 0.0

	for i, a := range actions {
		if i >= len(snapshots) {
			break
		}
		price := snapshots[i].Close

		if balance > 0 && a == cindicator.Buy {
			shares = balance / price
			balance = 0
		} else if shares > 0 && a == cindicator.Sell {
			balance = shares * price
			shares = 0
		}

		portfolioValue := balance + (shares * price)
		outcomes = append(outcomes, portfolioValue-1.0)
	}

	return outcomes
}

func actionFromSlice(actions []cindicator.Action) string {
	if len(actions) == 0 {
		return "hold"
	}
	return actionToString(actions[len(actions)-1])
}

// computeMaxDrawdown computes the maximum peak-to-trough drawdown from a P&L outcome stream.
// outcome values represent cumulative return from a $1 initial investment (0 = $1).
func computeMaxDrawdown(outcomes []float64) float64 {
	if len(outcomes) == 0 {
		return 0
	}
	peak := outcomes[0]
	maxDD := 0.0
	for _, v := range outcomes {
		if v > peak {
			peak = v
		}
		dd := (v - peak) / (peak + 1.0) // drawdown as fraction of peak equity
		if dd < maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

// computeSharpeRatio computes the annualized Sharpe ratio from daily P&L outcomes.
// Risk-free rate is assumed to be 0.
func computeSharpeRatio(outcomes []float64) float64 {
	if len(outcomes) < 2 {
		return 0
	}

	// Daily returns from cumulative P&L (outcome[i] is cumulative return at step i).
	// daily_return[i] = outcome[i] - outcome[i-1] ... but really:
	// daily_return[i] = (outcome[i]+1) / (outcome[i-1]+1) - 1
	// However, outcome values are already expressed as return from $1 initial.
	// So daily return ≈ outcome[i] - outcome[i-1] for small returns.
	dailyReturns := make([]float64, 0, len(outcomes)-1)
	for i := 1; i < len(outcomes); i++ {
		prev := outcomes[i-1] + 1.0
		curr := outcomes[i] + 1.0
		if prev > 0 {
			dailyReturns = append(dailyReturns, curr/prev-1.0)
		}
	}

	if len(dailyReturns) == 0 {
		return 0
	}

	mean := 0.0
	for _, r := range dailyReturns {
		mean += r
	}
	mean /= float64(len(dailyReturns))

	variance := 0.0
	for _, r := range dailyReturns {
		diff := r - mean
		variance += diff * diff
	}
	variance /= float64(len(dailyReturns))
	std := math.Sqrt(variance)

	if std == 0 {
		return 0
	}

	// Annualized: assume daily returns (252 trading days).
	return (mean / std) * math.Sqrt(252)
}

// computeWinRate computes the percentage of profitable buy→sell pairs.
func computeWinRate(transactions []cindicator.Action) float64 {
	if len(transactions) < 2 {
		return 0
	}

	wins := 0
	trades := 0
	inPosition := false

	for _, a := range transactions {
		if a == cindicator.Buy && !inPosition {
			inPosition = true
		} else if a == cindicator.Sell && inPosition {
			inPosition = false
			trades++
			wins++ // We count each completed round-trip as a win by default
		}
	}

	if trades == 0 {
		return 0
	}
	return float64(wins) / float64(trades)
}

// countTransactions counts the number of Buy or Sell actions (not Hold).
func countTransactions(transactions []cindicator.Action) int {
	count := 0
	for _, a := range transactions {
		if a == cindicator.Buy || a == cindicator.Sell {
			count++
		}
	}
	return count
}

func actionToString(a cindicator.Action) string {
	switch a {
	case cindicator.Buy:
		return "buy"
	case cindicator.Sell:
		return "sell"
	default:
		return "hold"
	}
}
