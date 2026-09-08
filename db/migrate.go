// pattern: Imperative Shell
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate uses an explicit management connection; runtime startup never changes schema.
func Migrate(ctx context.Context, dsn string) error {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(cfg.Database, "polis_r0_") {
		return fmt.Errorf("refusing non-R0 database")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	goose.SetBaseFS(migrations)
	if err = goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, db, "migrations")
}
