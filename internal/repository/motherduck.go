package repository

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/cinar/indicator/v2/asset"
)

const (
	// MotherDuckRepositoryName is the name used to register and reference
	// the MotherDuck repository builder.
	MotherDuckRepositoryName = "motherduck"

	// MotherDuckDriverName is the database/sql driver name for PostgreSQL.
	// pgx registers as "pgx" via the stdlib adapter.
	MotherDuckDriverName = "pgx"
)

// motherduckBuilder builds a SQLRepository backed by MotherDuck via PostgreSQL protocol.
// It first ensures the table and index exist, then returns the SQLRepository.
func motherduckBuilder(config string) (asset.Repository, error) {
	// Ensure table and index exist before returning the repository.
	if err := ensureSchema(config); err != nil {
		return nil, fmt.Errorf("schema init failed: %w", err)
	}

	return asset.NewSQLRepository(MotherDuckDriverName, config, NewPostgresDialect())
}

// ensureSchema creates the snapshots table and index if they don't exist.
func ensureSchema(dsn string) error {
	db, err := sql.Open(MotherDuckDriverName, dsn)
	if err != nil {
		return fmt.Errorf("unable to open database: %w", err)
	}
	defer db.Close()

	dialect := NewPostgresDialect()

	_, err = db.Exec(dialect.CreateTable())
	if err != nil {
		return fmt.Errorf("unable to create table: %w", err)
	}

	_, err = db.Exec(dialect.CreateIndex())
	if err != nil {
		// Index creation may fail if MotherDuck's PG endpoint doesn't support it.
		// This is non-fatal — queries still work, just slower.
		slog.Warn("Index creation skipped (non-fatal)", "error", err)
	}

	return nil
}

// RegisterMotherDuck registers the "motherduck" repository builder with the
// indicator library's repository factory. Must be called at startup before
// any sync operation that uses MotherDuck as target.
func RegisterMotherDuck() {
	asset.RegisterRepositoryBuilder(MotherDuckRepositoryName, motherduckBuilder)
}
