package signal

import (
	"context"
	"testing"
	"time"

	"github.com/cinar/indicator/v2/asset"
	cindicator "github.com/cinar/indicator/v2/strategy"
)

func makeTestSnapshots() []*asset.Snapshot {
	prices := []float64{150, 152, 148, 155, 157, 153, 158, 160, 162, 159}
	var snapshots []*asset.Snapshot
	for i, p := range prices {
		snapshots = append(snapshots, &asset.Snapshot{
			Date:   time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
			Close:  p,
			High:   p * 1.02,
			Low:    p * 0.98,
			Open:   p * 0.99,
			Volume: 1000000,
		})
	}
	return snapshots
}

func TestGenerateBuyAndHold(t *testing.T) {
	snapshots := makeTestSnapshots()
	st := cindicator.NewBuyAndHoldStrategy()

	records, err := Generate(context.Background(), st, snapshots, 1, "buy_and_hold_strategy", "TEST")
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != len(snapshots) {
		t.Fatalf("expected %d signals, got %d", len(snapshots), len(records))
	}

	// First signal should be "buy"
	if records[0].Action != "buy" {
		t.Errorf("expected first action 'buy', got '%s'", records[0].Action)
	}

	// Remaining should be "hold"
	for i := 1; i < len(records); i++ {
		if records[i].Action != "hold" {
			t.Errorf("records[%d]: expected 'hold', got '%s'", i, records[i].Action)
		}
	}

	// Prices should match closing prices
	for i, rec := range records {
		expectedPrice := snapshots[i].Close
		if rec.Price != expectedPrice {
			t.Errorf("records[%d].Price = %f, want %f", i, rec.Price, expectedPrice)
		}
	}

	// Dates should match
	for i, rec := range records {
		if !rec.SignalDate.Equal(snapshots[i].Date) {
			t.Errorf("records[%d].Date = %v, want %v", i, rec.SignalDate, snapshots[i].Date)
		}
	}
}

func TestGenerateEmptySnapshots(t *testing.T) {
	st := cindicator.NewBuyAndHoldStrategy()
	records, err := Generate(context.Background(), st, nil, 1, "buy_and_hold_strategy", "TEST")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 signals for empty snapshots, got %d", len(records))
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
	if actionToString(cindicator.Action(99)) != "hold" {
		t.Error("unknown action should map to hold")
	}
}
