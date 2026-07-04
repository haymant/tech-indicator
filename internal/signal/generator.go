package signal

import (
	"context"
	"time"

	"github.com/cinar/indicator/v2/asset"
	"github.com/cinar/indicator/v2/helper"
	cindicator "github.com/cinar/indicator/v2/strategy"
)

// SignalRecord is the internal representation of a generated signal.
type SignalRecord struct {
	StrategyID   int
	StrategyType string
	Underlying   string
	SignalDate   time.Time
	Action       string // "buy", "sell", "hold"
	Price        float64
}

// Generate runs a strategy against OHLCV snapshots and produces a slice of SignalRecords.
// Each snapshot produces exactly one signal record (or fewer if the strategy returns fewer actions).
// Uses st.Compute directly (not ComputeWithOutcome) to avoid channel deadlocks.
func Generate(
	ctx context.Context,
	st cindicator.Strategy,
	snapshots []*asset.Snapshot,
	strategyID int,
	strategyType string,
	underlying string,
) ([]SignalRecord, error) {
	if len(snapshots) == 0 {
		return nil, nil
	}

	// Use st.Compute directly (simpler than ComputeWithOutcome).
	snapshotChan := helper.SliceToChan(snapshots)
	actionChan := st.Compute(snapshotChan)

	var actions []cindicator.Action
	for a := range actionChan {
		actions = append(actions, a)
	}

	records := make([]SignalRecord, 0, len(actions))
	for i, a := range actions {
		if i >= len(snapshots) {
			break
		}
		records = append(records, SignalRecord{
			StrategyID:   strategyID,
			StrategyType: strategyType,
			Underlying:   underlying,
			SignalDate:   snapshots[i].Date,
			Action:       actionToString(a),
			Price:        snapshots[i].Close,
		})
	}

	return records, nil
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
