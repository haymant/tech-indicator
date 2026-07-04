package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Repository handles CRUD operations on the backtest_results table.
type Repository struct {
	databaseURL string
}

// NewRepository creates a new backtest repository.
func NewRepository(databaseURL string) *Repository {
	return &Repository{databaseURL: databaseURL}
}

// BacktestFilter holds optional filter parameters for querying backtest results.
type BacktestFilter struct {
	StrategyID   int
	StrategyType string
	Underlying   string
	DateFrom     string
	DateTo       string
	MinReturn    float64
	Limit        int
	Offset       int
}

// ExistingResult checks if a backtest result already exists for the given strategy + underlying + dates.
func (r *Repository) ExistingResult(ctx context.Context, strategyID int, underlying string, startDate, endDate time.Time) (*Result, error) {
	conn, err := pgx.Connect(ctx, r.databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	row := conn.QueryRow(ctx,
		`SELECT strategy_id, strategy_type, underlying, start_date::text, end_date::text,
		        total_return, max_drawdown, sharpe_ratio, win_rate, num_transactions,
		        final_outcome, final_action, parameters_snapshot
		 FROM backtest_results
		 WHERE strategy_id=$1 AND underlying=$2 AND start_date=$3 AND end_date=$4`,
		strategyID, underlying, startDate, endDate,
	)

	var res Result
	var startStr, endStr, strategyType, underlyingStr, finalAction string
	var totalReturn, maxDrawdown, sharpeRatio, winRate, finalOutcome *float64
	var numTransactions *int
	var paramsJSON []byte

	err = row.Scan(&res.StrategyID, &strategyType, &underlyingStr, &startStr, &endStr,
		&totalReturn, &maxDrawdown, &sharpeRatio, &winRate, &numTransactions,
		&finalOutcome, &finalAction, &paramsJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan: %w", err)
	}

	res.StrategyType = strategyType
	res.Underlying = underlyingStr
	res.StartDate, _ = time.Parse("2006-01-02", startStr[:10])
	res.EndDate, _ = time.Parse("2006-01-02", endStr[:10])
	res.FinalAction = finalAction

	if totalReturn != nil {
		res.TotalReturn = *totalReturn
	}
	if maxDrawdown != nil {
		res.MaxDrawdown = *maxDrawdown
	}
	if sharpeRatio != nil {
		res.SharpeRatio = *sharpeRatio
	}
	if winRate != nil {
		res.WinRate = *winRate
	}
	if finalOutcome != nil {
		res.FinalOutcome = *finalOutcome
	}
	if numTransactions != nil {
		res.NumTransactions = *numTransactions
	}
	if len(paramsJSON) > 0 {
		json.Unmarshal(paramsJSON, &res.Parameters)
	}

	return &res, nil
}

// InsertResult stores a new backtest result.
func (r *Repository) InsertResult(ctx context.Context, result *Result) error {
	conn, err := pgx.Connect(ctx, r.databaseURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	paramsJSON, _ := json.Marshal(result.Parameters)

	_, err = conn.Exec(ctx,
		`INSERT INTO backtest_results (strategy_id, strategy_type, underlying, start_date, end_date,
		 total_return, max_drawdown, sharpe_ratio, win_rate, num_transactions,
		 final_outcome, final_action, parameters_snapshot)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 ON CONFLICT (strategy_id, underlying, start_date, end_date) DO NOTHING`,
		result.StrategyID, result.StrategyType, result.Underlying,
		result.StartDate, result.EndDate,
		nullableFloat(result.TotalReturn), nullableFloat(result.MaxDrawdown),
		nullableFloat(result.SharpeRatio), nullableFloat(result.WinRate),
		nullableInt(result.NumTransactions), nullableFloat(result.FinalOutcome),
		result.FinalAction, paramsJSON,
	)
	return err
}

// DeleteResult removes a backtest result for the given strategy + underlying + dates.
func (r *Repository) DeleteResult(ctx context.Context, strategyID int, underlying string, startDate, endDate time.Time) error {
	conn, err := pgx.Connect(ctx, r.databaseURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx,
		`DELETE FROM backtest_results WHERE strategy_id=$1 AND underlying=$2 AND start_date=$3 AND end_date=$4`,
		strategyID, underlying, startDate, endDate,
	)
	return err
}

// GetResults retrieves backtest results with optional filters.
func (r *Repository) GetResults(ctx context.Context, filter BacktestFilter) ([]Result, int, error) {
	conn, err := pgx.Connect(ctx, r.databaseURL)
	if err != nil {
		return nil, 0, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	where := ""
	args := []any{}
	argIdx := 1

	if filter.StrategyID > 0 {
		where += fmt.Sprintf(" AND br.strategy_id=$%d", argIdx)
		args = append(args, filter.StrategyID)
		argIdx++
	}
	if filter.StrategyType != "" {
		where += fmt.Sprintf(" AND br.strategy_type=$%d", argIdx)
		args = append(args, filter.StrategyType)
		argIdx++
	}
	if filter.Underlying != "" {
		where += fmt.Sprintf(" AND LOWER(br.underlying)=LOWER($%d)", argIdx)
		args = append(args, filter.Underlying)
		argIdx++
	}
	if filter.DateFrom != "" {
		where += fmt.Sprintf(" AND br.end_date>=$%d", argIdx)
		args = append(args, filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != "" {
		where += fmt.Sprintf(" AND br.end_date<=$%d", argIdx)
		args = append(args, filter.DateTo)
		argIdx++
	}
	if filter.MinReturn != 0 {
		where += fmt.Sprintf(" AND br.total_return>=$%d", argIdx)
		args = append(args, filter.MinReturn)
		argIdx++
	}

	if len(where) > 0 {
		where = " WHERE" + where[4:]
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM backtest_results br` + where
	err = conn.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count query: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	dataQuery := `SELECT br.strategy_id, br.strategy_type, br.underlying,
	 br.start_date::text, br.end_date::text,
	 br.total_return, br.max_drawdown, br.sharpe_ratio, br.win_rate,
	 br.num_transactions, br.final_outcome, br.final_action,
	 COALESCE(st.name, '') as strategy_name
	 FROM backtest_results br
	 LEFT JOIN strategies st ON st.id = br.strategy_id` + where +
		` ORDER BY br.total_return DESC LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)
	argsData := append(args, limit, offset)

	rows, err := conn.Query(ctx, dataQuery, argsData...)
	if err != nil {
		return nil, 0, fmt.Errorf("data query: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var res Result
		var startStr, endStr, strategyType, underlyingStr, finalAction, strategyName string
		var totalReturn, maxDrawdown, sharpeRatio, winRate, finalOutcome *float64
		var numTransactions *int

		if err := rows.Scan(&res.StrategyID, &strategyType, &underlyingStr,
			&startStr, &endStr,
			&totalReturn, &maxDrawdown, &sharpeRatio, &winRate,
			&numTransactions, &finalOutcome, &finalAction, &strategyName); err != nil {
			continue
		}

		res.StrategyType = strategyType
		res.Underlying = underlyingStr
		res.StartDate, _ = time.Parse("2006-01-02", startStr[:10])
		res.EndDate, _ = time.Parse("2006-01-02", endStr[:10])
		res.FinalAction = finalAction
		_ = strategyName

		if totalReturn != nil {
			res.TotalReturn = *totalReturn
		}
		if maxDrawdown != nil {
			res.MaxDrawdown = *maxDrawdown
		}
		if sharpeRatio != nil {
			res.SharpeRatio = *sharpeRatio
		}
		if winRate != nil {
			res.WinRate = *winRate
		}
		if finalOutcome != nil {
			res.FinalOutcome = *finalOutcome
		}
		if numTransactions != nil {
			res.NumTransactions = *numTransactions
		}

		results = append(results, res)
	}

	if results == nil {
		results = []Result{}
	}
	return results, total, nil
}

func nullableFloat(v float64) *float64 {
	return &v
}

func nullableInt(v int) *int {
	return &v
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
