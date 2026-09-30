// pattern: Imperative Shell
package db

import (
	"context"
	"embed"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrations embed.FS

//go:embed migration_hashes.sha256
var migrationHashManifest []byte

// Migrate uses an explicit management connection; runtime startup never changes schema.
func Migrate(ctx context.Context, dsn string) error {
	return MigrateToVersion(ctx, dsn, 0)
}
