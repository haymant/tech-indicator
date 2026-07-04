package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const strategiesDDL = `CREATE TABLE IF NOT EXISTS strategies (
	id              INTEGER PRIMARY KEY,
	name            VARCHAR(255) NOT NULL,
	strategy_type   VARCHAR(100) NOT NULL,
	underlying      VARCHAR(20) NOT NULL,
	timeframe       VARCHAR(10) NOT NULL DEFAULT '1d',
	lookback_days   INTEGER NOT NULL DEFAULT 365,
	parameters      JSON,
	created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
	updated_at      TIMESTAMP NOT NULL DEFAULT NOW(),
	CONSTRAINT uq_strategies_name UNIQUE (name)
);

CREATE INDEX IF NOT EXISTS idx_strategies_type ON strategies(strategy_type);
CREATE INDEX IF NOT EXISTS idx_strategies_underlying ON strategies(underlying);`

const signalsDDL = `CREATE TABLE IF NOT EXISTS signals (
	strategy_id     INTEGER NOT NULL REFERENCES strategies(id),
	strategy_type   VARCHAR(100) NOT NULL,
	underlying      VARCHAR(20) NOT NULL,
	signal_date     DATE NOT NULL,
	action          VARCHAR(10) NOT NULL,
	price           DOUBLE PRECISION,
	created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
	PRIMARY KEY (strategy_id, underlying, signal_date)
);

CREATE INDEX IF NOT EXISTS idx_signals_type ON signals(strategy_type);
CREATE INDEX IF NOT EXISTS idx_signals_action ON signals(action);`

const backtestResultsDDL = `CREATE TABLE IF NOT EXISTS backtest_results (
	strategy_id         INTEGER NOT NULL REFERENCES strategies(id),
	strategy_type       VARCHAR(100) NOT NULL,
	underlying          VARCHAR(20) NOT NULL,
	start_date          DATE NOT NULL,
	end_date            DATE NOT NULL,
	total_return        DOUBLE PRECISION,
	max_drawdown        DOUBLE PRECISION,
	sharpe_ratio        DOUBLE PRECISION,
	win_rate            DOUBLE PRECISION,
	num_transactions    INTEGER,
	final_outcome       DOUBLE PRECISION,
	final_action        VARCHAR(10),
	parameters_snapshot JSON,
	created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
	PRIMARY KEY (strategy_id, underlying, start_date, end_date)
);

CREATE INDEX IF NOT EXISTS idx_backtest_underlying ON backtest_results(underlying);
CREATE INDEX IF NOT EXISTS idx_backtest_return ON backtest_results(total_return);`

// Migrations is the ordered list of DDL statements to run at startup.
var Migrations = []string{
	// Drop old tables with `id INTEGER PRIMARY KEY` to recreate with composite PKs.
	`DROP TABLE IF EXISTS backtest_results`,
	`DROP TABLE IF EXISTS signals`,
	strategiesDDL,
	signalsDDL,
	backtestResultsDDL,
}

// ResetMigrations drops and recreates all F-005 tables (destructive).
var ResetMigrations = []string{
	`DROP TABLE IF EXISTS backtest_results`,
	`DROP TABLE IF EXISTS signals`,
	`DROP TABLE IF EXISTS strategies`,
	strategiesDDL,
	signalsDDL,
	backtestResultsDDL,
}

// RunMigrations executes all DDL statements in order. It is idempotent
// (all CREATE TABLE/INDEX statements use IF NOT EXISTS).
func RunMigrations(databaseURL string) error {
	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer conn.Close(context.Background())

	// Split multi-statement DDL by semicolons and execute each individually.
	for i, ddl := range Migrations {
		statements := splitSQL(ddl)
		for j, stmt := range statements {
			stmt = trimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := conn.Exec(context.Background(), stmt); err != nil {
				return fmt.Errorf("migration %d, statement %d failed: %w", i+1, j+1, err)
			}
		}
	}
	return nil
}

// splitSQL splits a multi-statement SQL string into individual statements.
func splitSQL(s string) []string {
	var result []string
	current := ""
	for i := 0; i < len(s); i++ {
		if s[i] == ';' {
			result = append(result, current)
			current = ""
		} else {
			current += string(s[i])
		}
	}
	if trimSpace(current) != "" {
		result = append(result, current)
	}
	return result
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\n' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
