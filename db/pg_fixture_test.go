// pattern: Imperative Shell
package db

import (
	"context"
	"database/sql"
	"github.com/pressly/goose/v3"
	"polis/internal/pgfixture"
	"strconv"
	"strings"
	"testing"
)

func migrationFixtureDSN(t *testing.T) string {
	t.Helper()
	dsn := pgfixture.EmptyDSN(t, "POLIS_MIGRATION_TEST_DSN")
	if dsn == "" {
		return ""
	}
	c, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	goose.SetBaseFS(migrations)
	if e = goose.SetDialect("postgres"); e != nil {
		t.Fatal(e)
	}
	if e = goose.UpToContext(context.Background(), c, "migrations", 39); e != nil {
		t.Fatal(e)
	}
	return dsn
}
func currentSourceVersion(t *testing.T) int64 {
	t.Helper()
	files, e := migrations.ReadDir("migrations")
	if e != nil {
		t.Fatal(e)
	}
	var max int64
	for _, f := range files {
		prefix, _, _ := strings.Cut(f.Name(), "_")
		n, e := strconv.ParseInt(prefix, 10, 64)
		if e == nil && n > max {
			max = n
		}
	}
	return max
}
