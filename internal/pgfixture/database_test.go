// pattern: Functional Core
package pgfixture

import (
	"context"
	"github.com/jackc/pgx/v5"
	"strings"
	"testing"
)

func TestConnectionStringUsesChangedDatabase(t *testing.T) {
	c, e := pgx.ParseConfig("host=/tmp/fixture dbname=polis_r0_template user=runtime")
	if e != nil {
		t.Fatal(e)
	}
	c.Database = "polis_r0_clone"
	parsed, e := pgx.ParseConfig(connectionString(c))
	if e != nil || parsed.Database != "polis_r0_clone" || parsed.Host != c.Host || parsed.User != c.User {
		t.Fatalf("clone identity not preserved: %v", e)
	}
}

func TestHarnessDatabaseUsesCaseClone(t *testing.T) {
	dsn := DSN(t, "POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated disposable PG18 harness required")
	}
	c, e := pgx.Connect(context.Background(), dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close(context.Background())
	var database string
	var super bool
	if e = c.QueryRow(context.Background(), "SELECT current_database(),rolsuper FROM pg_roles WHERE rolname=current_user").Scan(&database, &super); e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(database, "polis_r0_case_") || super {
		t.Fatalf("test did not use its isolated non-superuser database: %s", database)
	}
}
