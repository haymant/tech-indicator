package backtest

import (
	"context"
	"testing"
	"time"

	"github.com/cinar/indicator/v2/asset"
	cindicator "github.com/cinar/indicator/v2/strategy"
)

func makeUptrendSnapshots(n int) []*asset.Snapshot {
	var snapshots []*asset.Snapshot
	price := 100.0
	for i := 0; i < n; i++ {
		snapshots = append(snapshots, &asset.Snapshot{
			Date:   time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
			Close:  price,
			High:   price * 1.02,
			Low:    price * 0.98,
			Open:   price * 0.99,
			Volume: 1000000,
		})
		price *= 1.005 // steady uptrend
	}
	return snapshots
}

func makeDowntrendSnapshots(n int) []*asset.Snapshot {
	var snapshots []*asset.Snapshot
	price := 100.0
	for i := 0; i < n; i++ {
		snapshots = append(snapshots, &asset.Snapshot{
			Date:   time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
			Close:  price,
			High:   price * 1.02,
			Low:    price * 0.98,
			Open:   price * 0.99,
			Volume: 1000000,
		})
		price *= 0.995 // steady downtrend
	}
	return snapshots
}

func TestRunBuyAndHoldUptrend(t *testing.T) {
	snapshots := makeUptrendSnapshots(252)
	st := cindicator.NewBuyAndHoldStrategy()

	result, err := Run(context.Background(), st, snapshots, 1, "buy_and_hold_strategy", "TEST", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Uptrend should produce positive return.
	if result.TotalReturn <= 0 {
		t.Errorf("expected positive total return for uptrend, got %f", result.TotalReturn)
	}

	// MaxDrawdown should be <= 0.
	if result.MaxDrawdown > 0 {
		t.Errorf("max drawdown should be <= 0, got %f", result.MaxDrawdown)
	}

	// FinalAction should be "hold" for BuyAndHold (buy on first, hold the rest).
	if result.FinalAction != "hold" {
		t.Errorf("expected final action 'hold', got '%s'", result.FinalAction)
	}

	// NumTransactions should be at least 1 (the initial buy).
	if result.NumTransactions < 1 {
		t.Errorf("expected at least 1 transaction, got %d", result.NumTransactions)
	}
}

func TestRunBuyAndHoldDowntrend(t *testing.T) {
	snapshots := makeDowntrendSnapshots(252)
	st := cindicator.NewBuyAndHoldStrategy()

	result, err := Run(context.Background(), st, snapshots, 1, "buy_and_hold_strategy", "TEST", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Downtrend should produce negative return.
	if result.TotalReturn >= 0 {
		t.Errorf("expected negative total return for downtrend, got %f", result.TotalReturn)
	}
}

func TestRunEmptySnapshots(t *testing.T) {
	st := cindicator.NewBuyAndHoldStrategy()
	result, err := Run(context.Background(), st, nil, 1, "buy_and_hold_strategy", "TEST", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil result for empty snapshots, got %+v", result)
	}
}

func TestComputeMaxDrawdown(t *testing.T) {
	// Simple case: [0, 0.1, 0.05, 0.2] — peak at 0.2, trough after 0.1 → peak at 0.1
	outcomes := []float64{0, 0.1, 0.05, 0.2}
	dd := computeMaxDrawdown(outcomes)
	if dd > 0 {
		t.Errorf("max drawdown should be <= 0, got %f", dd)
	}
	// With peak at 0.1 and trough at 0.05: (0.05-0.1) / (0.1+1) = -0.05/1.1 ≈ -0.045
	if dd > -0.001 {
		t.Errorf("expected drawdown around -0.045, got %f", dd)
	}

	// Empty case.
	if v := computeMaxDrawdown(nil); v != 0 {
		t.Errorf("expected 0 for empty, got %f", v)
	}
}

func TestCountTransactions(t *testing.T) {
	actions := []cindicator.Action{cindicator.Buy, cindicator.Hold, cindicator.Sell, cindicator.Hold}
	if n := countTransactions(actions); n != 2 {
		t.Errorf("expected 2 transactions, got %d", n)
	}
	if n := countTransactions(nil); n != 0 {
		t.Errorf("expected 0 for nil, got %d", n)
	}
}

func TestActionToString(t *testing.T) {
	if actionToString(cindicator.Buy) != "buy" {
		t.Error("buy mapping wrong")
	}
	if actionToString(cindicator.Sell) != "sell" {
		t.Error("sell mapping wrong")
	}
	if actionToString(cindicator.Hold) != "hold" {
		t.Error("hold mapping wrong")
	}
}
