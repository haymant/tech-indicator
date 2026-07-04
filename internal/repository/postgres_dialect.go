package repository

import (
	"fmt"

	"github.com/cinar/indicator/v2/asset"
)

// PostgresDialect implements the asset.SQLRepositoryDialect interface
// for MotherDuck accessed via PostgreSQL wire protocol, using
// PostgreSQL-style $N parameter placeholders.
type PostgresDialect struct{}

// NewPostgresDialect creates a new PostgresDialect instance.
func NewPostgresDialect() *PostgresDialect {
	return &PostgresDialect{}
}

// compile-time check: PostgresDialect implements asset.SQLRepositoryDialect.
var _ asset.SQLRepositoryDialect = (*PostgresDialect)(nil)

// CreateTable returns the SQL statement to create the snapshots table.
// Index creation is handled separately in RegisterMotherDuck since
// pgx (PostgreSQL protocol) doesn't support multi-statement Exec
// through database/sql.
func (d *PostgresDialect) CreateTable() string {
	return `CREATE TABLE IF NOT EXISTS snapshots (
    name   TEXT NOT NULL,
    date   DATE NOT NULL,
    open   DOUBLE PRECISION NOT NULL,
    high   DOUBLE PRECISION NOT NULL,
    low    DOUBLE PRECISION NOT NULL,
    close  DOUBLE PRECISION NOT NULL,
    volume DOUBLE PRECISION NOT NULL
)`
}

// CreateIndex returns the SQL statement to create the composite index.
func (d *PostgresDialect) CreateIndex() string {
	return `CREATE INDEX IF NOT EXISTS idx_snapshots_name_date ON snapshots (name, date)`
}

// DropTable returns the SQL statement to drop the snapshots table.
func (d *PostgresDialect) DropTable() string {
	return `DROP TABLE IF EXISTS snapshots`
}

// Assets returns the SQL statement to get distinct asset names.
func (d *PostgresDialect) Assets() string {
	return `SELECT DISTINCT name FROM snapshots ORDER BY name`
}

// GetSince returns the SQL statement to query snapshots since a given date.
func (d *PostgresDialect) GetSince() string {
	return fmt.Sprintf(`SELECT %s FROM snapshots WHERE name = $1 AND date >= $2 ORDER BY date`,
		d.columns(),
	)
}

// LastDate returns the SQL statement to query the latest snapshot date.
func (d *PostgresDialect) LastDate() string {
	return `SELECT MAX(date) FROM snapshots WHERE name = $1`
}

// Append returns the SQL statement to insert a snapshot row.
func (d *PostgresDialect) Append() string {
	return fmt.Sprintf(`INSERT INTO snapshots (%s) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		d.columnsWithName(),
	)
}

// columns returns the column list used in SELECT queries (without name).
func (d *PostgresDialect) columns() string {
	return "date, open, high, low, close, volume"
}

// columnsWithName returns the full column list including name.
func (d *PostgresDialect) columnsWithName() string {
	return "name, date, open, high, low, close, volume"
}
