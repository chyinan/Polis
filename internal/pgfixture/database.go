// pattern: Imperative Shell
// Package pgfixture provisions test-owned databases within the existing disposable
// PG18 harness. It never changes application runtime grants.
package pgfixture

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func DSN(t testing.TB, key string) string {
	return caseDSN(t, key, true)
}

func EmptyDSN(t testing.TB, key string) string { return caseDSN(t, key, false) }

func caseDSN(t testing.TB, key string, clone bool) string {
	t.Helper()
	runtimeDSN := os.Getenv(key)
	adminDSN, template := os.Getenv("POLIS_TEST_TEMPLATE_ADMIN_DSN"), os.Getenv("POLIS_TEST_TEMPLATE_DATABASE")
	if runtimeDSN == "" || adminDSN == "" || template == "" {
		if !clone && runtimeDSN != "" {
			t.Fatal("administrative migration/recovery tests require the dedicated disposable PG18 harness")
		}
		return runtimeDSN
	}
	cfg, err := pgx.ParseConfig(runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database != template {
		return runtimeDSN
	} // a child process inherits its parent's isolated database
	adminCfg, err := pgx.ParseConfig(adminDSN)
	if err != nil || !strings.HasPrefix(template, "polis_r0_self_dev_go_") || !strings.HasPrefix(adminCfg.Host, "/tmp/polis-self-dev-go-pg.") || adminCfg.Database != "postgres" {
		t.Fatal("refusing database provisioning outside the disposable PG18 harness")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, adminCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	nonce := make([]byte, 12)
	if _, err = rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	database := "polis_r0_case_" + hex.EncodeToString(nonce)
	createSQL := "CREATE DATABASE " + pgx.Identifier{database}.Sanitize()
	if clone {
		createSQL += " TEMPLATE " + pgx.Identifier{template}.Sanitize()
	}
	_, err = conn.Exec(ctx, createSQL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		c, e := pgx.ConnectConfig(ctx, adminCfg)
		if e != nil {
			t.Errorf("connect for test database cleanup: %v", e)
			return
		}
		defer c.Close(context.Background())
		if _, e = c.Exec(ctx, "DROP DATABASE "+pgx.Identifier{database}.Sanitize()+" WITH (FORCE)"); e != nil {
			t.Errorf("drop exact test database: %v", e)
		}
	})
	cfg.Database = database
	cloneDSN := connectionString(cfg)
	t.Setenv(key, cloneDSN)
	caseAdmin := adminCfg.Copy()
	caseAdmin.Database = database
	t.Setenv("POLIS_TEST_ADMIN_DSN", connectionString(caseAdmin))
	t.Setenv("POLIS_MEMORY_REVOCATION_ROOT", filepath.Join(t.TempDir(), "memory-revocations"))
	return cloneDSN
}

// pgx.ConnConfig.ConnString returns the original parsed string, even after its
// Database field changes. Construct a new DSN so tests actually use their clone.
func connectionString(cfg *pgx.ConnConfig) string {
	quote := func(v string) string {
		return "'" + strings.ReplaceAll(strings.ReplaceAll(v, "\\", "\\\\"), "'", "\\'") + "'"
	}
	return fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable", quote(cfg.Host), strconv.Itoa(int(cfg.Port)), quote(cfg.Database), quote(cfg.User), quote(cfg.Password))
}

func RuntimeForDatabase(t testing.TB, runtimeDSN, databaseDSN string) string {
	t.Helper()
	r, e := pgx.ParseConfig(runtimeDSN)
	if e != nil {
		t.Fatal(e)
	}
	d, e := pgx.ParseConfig(databaseDSN)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(d.Database, "polis_r0_case_") || r.Host != d.Host {
		t.Fatal("refusing a runtime DSN outside its test database")
	}
	r.Database = d.Database
	return connectionString(r)
}
