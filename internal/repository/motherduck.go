package repository

import (
	_ "github.com/duckdb/duckdb-go/v2"

	"github.com/cinar/indicator/v2/asset"
)

const (
	// MotherDuckRepositoryName is the name used to register and reference
	// the MotherDuck repository builder.
	MotherDuckRepositoryName = "motherduck"

	// MotherDuckDriverName is the database/sql driver name for DuckDB.
	MotherDuckDriverName = "duckdb"
)

// motherduckBuilder builds a SQLRepository backed by MotherDuck.
func motherduckBuilder(config string) (asset.Repository, error) {
	return asset.NewSQLRepository(MotherDuckDriverName, config, NewDuckDBDialect())
}

// RegisterMotherDuck registers the "motherduck" repository builder with the
// indicator library's repository factory. Must be called at startup before
// any sync operation that uses MotherDuck as target.
func RegisterMotherDuck() {
	asset.RegisterRepositoryBuilder(MotherDuckRepositoryName, motherduckBuilder)
}
