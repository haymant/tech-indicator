package repository

import (
	"fmt"

	"github.com/cinar/indicator/v2/asset"
)

// DuckDBDialect implements the asset.SQLRepositoryDialect interface
// for DuckDB and MotherDuck databases, using PostgreSQL-style $N
// parameter placeholders.
type DuckDBDialect struct{}

// NewDuckDBDialect creates a new DuckDBDialect instance.
func NewDuckDBDialect() *DuckDBDialect {
	return &DuckDBDialect{}
}

// compile-time check: DuckDBDialect implements asset.SQLRepositoryDialect.
var _ asset.SQLRepositoryDialect = (*DuckDBDialect)(nil)

// CreateTable returns the SQL statements to create the snapshots table
// and its index. The index on (name, date) accelerates the primary query
// pattern (WHERE name = $1 AND date >= $2). DuckDB supports multi-statement
// Exec, and both statements use IF NOT EXISTS for idempotency.
func (d *DuckDBDialect) CreateTable() string {
	return `CREATE TABLE IF NOT EXISTS snapshots (
    name   TEXT NOT NULL,
    date   DATE NOT NULL,
    open   DOUBLE NOT NULL,
    high   DOUBLE NOT NULL,
    low    DOUBLE NOT NULL,
    close  DOUBLE NOT NULL,
    volume DOUBLE NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_snapshots_name_date ON snapshots (name, date)`
}

// DropTable returns the SQL statement to drop the snapshots table.
func (d *DuckDBDialect) DropTable() string {
	return `DROP TABLE IF EXISTS snapshots`
}

// Assets returns the SQL statement to get distinct asset names.
func (d *DuckDBDialect) Assets() string {
	return `SELECT DISTINCT name FROM snapshots ORDER BY name`
}

// GetSince returns the SQL statement to query snapshots since a given date.
func (d *DuckDBDialect) GetSince() string {
	return fmt.Sprintf(`SELECT %s FROM snapshots WHERE name = $1 AND date >= $2 ORDER BY date`,
		d.columns(),
	)
}

// LastDate returns the SQL statement to query the latest snapshot date.
func (d *DuckDBDialect) LastDate() string {
	return `SELECT MAX(date) FROM snapshots WHERE name = $1`
}

// Append returns the SQL statement to insert a snapshot row.
func (d *DuckDBDialect) Append() string {
	return fmt.Sprintf(`INSERT INTO snapshots (%s) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		d.columnsWithName(),
	)
}

// columns returns the column list used in SELECT queries (without name).
func (d *DuckDBDialect) columns() string {
	return "date, open, high, low, close, volume"
}

// columnsWithName returns the full column list including name.
func (d *DuckDBDialect) columnsWithName() string {
	return "name, date, open, high, low, close, volume"
}
