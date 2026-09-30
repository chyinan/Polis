// pattern: Functional Core
package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
)

func TestVerifyMigrationManifestMatchesExactEmbeddedSQLSet(t *testing.T) {
	first := []byte("-- migration one\nSELECT 1;\n")
	second := []byte("-- migration two\nSELECT 2;\n")
	fs := fstest.MapFS{
		"migrations/00001_one.sql": &fstest.MapFile{Data: first},
		"migrations/00002_two.sql": &fstest.MapFile{Data: second},
	}
	manifest := []byte(fmt.Sprintf("%s  00001_one.sql\n%s  00002_two.sql\n", sha256Hex(first), sha256Hex(second)))
	if err := verifyMigrationManifest(fs, manifest); err != nil {
		t.Fatalf("valid migration manifest: %v", err)
	}
}

func TestEmbeddedMigrationManifestMatchesRepository(t *testing.T) {
	if err := verifyMigrationManifest(migrations, migrationHashManifest); err != nil {
		t.Fatalf("embedded migration manifest: %v", err)
	}
}

func TestVerifyMigrationManifestRejectsChangedMissingExtraAndUnsortedEntries(t *testing.T) {
	first := []byte("SELECT 1;")
	second := []byte("SELECT 2;")
	fs := fstest.MapFS{
		"migrations/00001_one.sql": &fstest.MapFile{Data: first},
		"migrations/00002_two.sql": &fstest.MapFile{Data: second},
	}
	valid := fmt.Sprintf("%s  00001_one.sql\n%s  00002_two.sql\n", sha256Hex(first), sha256Hex(second))
	cases := map[string]string{
		"changed content":   strings.Replace(valid, sha256Hex(first), sha256Hex([]byte("SELECT 9;")), 1),
		"missing entry":     fmt.Sprintf("%s  00001_one.sql\n", sha256Hex(first)),
		"extra entry":       valid + fmt.Sprintf("%s  00003_extra.sql\n", sha256Hex([]byte("SELECT 3;"))),
		"unsorted":          fmt.Sprintf("%s  00002_two.sql\n%s  00001_one.sql\n", sha256Hex(second), sha256Hex(first)),
		"malformed hash":    "xyz  00001_one.sql\n",
		"zero version":      fmt.Sprintf("%s  00000_zero.sql\n", sha256Hex(first)),
		"upper extension":   fmt.Sprintf("%s  00001_one.SQL\n", sha256Hex(first)),
		"duplicate version": fmt.Sprintf("%s  00001_one.sql\n%s  00001_two.sql\n", sha256Hex(first), sha256Hex(second)),
	}
	for name, manifest := range cases {
		t.Run(name, func(t *testing.T) {
			if err := verifyMigrationManifest(fs, []byte(manifest)); err == nil {
				t.Fatal("invalid migration manifest was accepted")
			}
		})
	}
}

func TestMigrateRejectsManifestDriftBeforeParsingDatabaseConnection(t *testing.T) {
	original := migrationHashManifest
	tampered := append([]byte(nil), original...)
	if tampered[0] == '0' {
		tampered[0] = '1'
	} else {
		tampered[0] = '0'
	}
	migrationHashManifest = tampered
	defer func() { migrationHashManifest = original }()

	err := Migrate(context.Background(), "not-a-database-connection-string")
	if err == nil || !strings.Contains(err.Error(), "embedded SQL set failed its hash manifest") {
		t.Fatalf("Migrate error=%v, want hash-manifest rejection before database parsing", err)
	}
}

func sha256Hex(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}
