package signal

import (
	"context"
	"os"
	"testing"
	"time"
)

func getTestDB(t *testing.T) string {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	return url
}

func TestRepositoryInsertAndQuery(t *testing.T) {
	dbURL := getTestDB(t)
	ctx := context.Background()
	repo := NewRepository(dbURL)

	now := time.Now()
	records := []SignalRecord{
		{StrategyID: 1, StrategyType: "buy_and_hold_strategy", Underlying: "TEST", SignalDate: now, Action: "buy", Price: 100},
		{StrategyID: 1, StrategyType: "buy_and_hold_strategy", Underlying: "TEST", SignalDate: now.AddDate(0, 0, 1), Action: "hold", Price: 101},
		{StrategyID: 1, StrategyType: "buy_and_hold_strategy", Underlying: "TEST", SignalDate: now.AddDate(0, 0, 2), Action: "hold", Price: 102},
	}

	if err := repo.InsertSignals(ctx, records); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		repo.DeleteSignals(ctx, 1, "TEST", now.AddDate(0, 0, -1), now.AddDate(0, 0, 3))
	})

	exists, err := repo.ExistingSignals(ctx, 1, "TEST", now, now.AddDate(0, 0, 2))
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected signals to exist after insert")
	}

	// Query with filter
	filter := SignalFilter{StrategyID: 1, Limit: 10}
	results, total, err := repo.GetSignals(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if total < 3 {
		t.Errorf("expected at least 3 total signals, got %d", total)
	}
	if len(results) == 0 {
		t.Error("expected non-empty results")
	}
}

func TestRepositoryExistingSignals(t *testing.T) {
	dbURL := getTestDB(t)
	ctx := context.Background()
	repo := NewRepository(dbURL)

	now := time.Now()
	exists, err := repo.ExistingSignals(ctx, 9999, "NONEXISTENT", now, now)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("expected no signals for non-existent strategy")
	}
}
