package signal

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Repository handles CRUD operations on the signals table.
type Repository struct {
	databaseURL string
}

// NewRepository creates a new signal repository.
func NewRepository(databaseURL string) *Repository {
	return &Repository{databaseURL: databaseURL}
}

// SignalFilter holds optional filter parameters for querying signals.
type SignalFilter struct {
	StrategyID   int
	StrategyType string
	Underlying   string
	DateFrom     string
	DateTo       string
	Action       string
	Limit        int
	Offset       int
}

// ExistingSignals checks if signals already exist for a strategy + underlying within a date range.
func (r *Repository) ExistingSignals(ctx context.Context, strategyID int, underlying string, startDate, endDate time.Time) (bool, error) {
	conn, err := pgx.Connect(ctx, r.databaseURL)
	if err != nil {
		return false, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	var count int
	err = conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM signals WHERE strategy_id=$1 AND underlying=$2 AND signal_date>=$3 AND signal_date<=$4`,
		strategyID, underlying, startDate, endDate,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query: %w", err)
	}
	return count > 0, nil
}

// GetSignals retrieves signals with optional filters.
func (r *Repository) GetSignals(ctx context.Context, filter SignalFilter) ([]SignalRecord, int, error) {
	conn, err := pgx.Connect(ctx, r.databaseURL)
	if err != nil {
		return nil, 0, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	// Build query
	where := ""
	args := []any{}
	argIdx := 1

	if filter.StrategyID > 0 {
		where += fmt.Sprintf(" AND s.strategy_id=$%d", argIdx)
		args = append(args, filter.StrategyID)
		argIdx++
	}
	if filter.StrategyType != "" {
		where += fmt.Sprintf(" AND s.strategy_type=$%d", argIdx)
		args = append(args, filter.StrategyType)
		argIdx++
	}
	if filter.Underlying != "" {
		where += fmt.Sprintf(" AND LOWER(s.underlying)=LOWER($%d)", argIdx)
		args = append(args, filter.Underlying)
		argIdx++
	}
	if filter.DateFrom != "" {
		where += fmt.Sprintf(" AND s.signal_date>=$%d", argIdx)
		args = append(args, filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != "" {
		where += fmt.Sprintf(" AND s.signal_date<=$%d", argIdx)
		args = append(args, filter.DateTo)
		argIdx++
	}
	if filter.Action != "" {
		where += fmt.Sprintf(" AND s.action=$%d", argIdx)
		args = append(args, filter.Action)
		argIdx++
	}

	if len(where) > 0 {
		where = " WHERE" + where[4:] // remove leading " AND"
	}

	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM signals s` + where
	err = conn.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count query: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 1000
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	dataQuery := `SELECT s.strategy_id, s.strategy_type, s.underlying, s.signal_date::text, s.action, s.price, COALESCE(st.name, '') as strategy_name FROM signals s LEFT JOIN strategies st ON st.id = s.strategy_id` + where + ` ORDER BY s.signal_date ASC LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)
	argsData := append(args, limit, offset)

	rows, err := conn.Query(ctx, dataQuery, argsData...)
	if err != nil {
		return nil, 0, fmt.Errorf("data query: %w", err)
	}
	defer rows.Close()

	var records []SignalRecord
	for rows.Next() {
		var rec SignalRecord
		var dateStr, action, underlying, strategyType string
		var strategyName string
		var price float64
		if err := rows.Scan(&rec.StrategyID, &strategyType, &underlying, &dateStr, &action, &price, &strategyName); err != nil {
			continue
		}
		rec.SignalDate, _ = time.Parse("2006-01-02", dateStr[:10])
		rec.Action = action
		rec.Price = price
		rec.StrategyType = strategyType
		rec.Underlying = underlying
		_ = strategyName // available for enrichment
		records = append(records, rec)
	}

	if records == nil {
		records = []SignalRecord{}
	}
	return records, total, nil
}

// InsertSignals batch-inserts signal records into the signals table.
func (r *Repository) InsertSignals(ctx context.Context, records []SignalRecord) error {
	if len(records) == 0 {
		return nil
	}

	conn, err := pgx.Connect(ctx, r.databaseURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	batch := &pgx.Batch{}
	for _, rec := range records {
		batch.Queue(
			`INSERT INTO signals (strategy_id, strategy_type, underlying, signal_date, action, price) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (strategy_id, underlying, signal_date) DO NOTHING`,
			rec.StrategyID, rec.StrategyType, rec.Underlying, rec.SignalDate, rec.Action, rec.Price,
		)
	}

	br := conn.SendBatch(ctx, batch)
	defer br.Close()

	for range records {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("batch insert: %w", err)
		}
	}
	return nil
}

// DeleteSignals removes signals for a strategy + underlying within a date range.
func (r *Repository) DeleteSignals(ctx context.Context, strategyID int, underlying string, startDate, endDate time.Time) error {
	conn, err := pgx.Connect(ctx, r.databaseURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx,
		`DELETE FROM signals WHERE strategy_id=$1 AND underlying=$2 AND signal_date>=$3 AND signal_date<=$4`,
		strategyID, underlying, startDate, endDate,
	)
	return err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
