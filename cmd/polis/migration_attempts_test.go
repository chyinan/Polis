// pattern: Imperative Shell
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestMigrationRefusesToRunWithoutAttemptJournalRoot(t *testing.T) {
	t.Setenv("POLIS_MIGRATION_ATTEMPT_ROOT", "")
	t.Setenv("POLIS_BLOB_ROOT", "")

	err := runMigrationWithAttemptJournal(context.Background(), "not-a-database-connection-string", 0)
	if err == nil || !strings.Contains(err.Error(), "migration attempt root is required") {
		t.Fatalf("migration error=%v, want fail-closed journal-root error", err)
	}
	if strings.Contains(err.Error(), "database") || strings.Contains(err.Error(), "connection string") {
		t.Fatalf("migration reached database setup before journal validation: %v", err)
	}
}

func TestMigrationAttemptVolumeRootDetection(t *testing.T) {
	tests := []struct {
		name, path, volume, separator string
		want                          bool
	}{
		{name: "windows drive root", path: `C:\`, volume: "C:", separator: `\`, want: true},
		{name: "windows unc share root", path: `\\server\share`, volume: `\\server\share`, separator: `\`, want: true},
		{name: "windows nested path", path: `C:\ProgramData\Polis`, volume: "C:", separator: `\`},
		{name: "posix root", path: "/", separator: "/", want: true},
		{name: "posix nested path", path: "/var/lib/polis", separator: "/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := migrationAttemptIsVolumeRoot(test.path, test.volume, test.separator); got != test.want {
				t.Fatalf("migrationAttemptIsVolumeRoot(%q, %q, %q)=%v, want %v", test.path, test.volume, test.separator, got, test.want)
			}
		})
	}
}

func TestMigrationAttemptRootExistenceDetectionUnwrapsCleanupError(t *testing.T) {
	joined := errors.Join(errors.New("temporary directory cleanup failed"), os.ErrExist)
	if !migrationAttemptRootAlreadyExists(joined) {
		t.Fatal("wrapped concurrent root creation was not recognized as already existing")
	}
	if migrationAttemptRootAlreadyExists(errors.New("permission denied")) {
		t.Fatal("unrelated root creation failure was classified as a concurrent creator")
	}
}

func TestMigrationRefusesAttemptJournalInsideOrAroundBlobRoot(t *testing.T) {
	base := t.TempDir()
	blobRoot := filepath.Join(base, "cas")
	if err := os.Mkdir(blobRoot, 0700); err != nil {
		t.Fatal(err)
	}
	for _, journalRoot := range []string{blobRoot, filepath.Join(blobRoot, "attempts"), base} {
		t.Run(filepath.Base(journalRoot), func(t *testing.T) {
			t.Setenv("POLIS_MIGRATION_ATTEMPT_ROOT", journalRoot)
			t.Setenv("POLIS_BLOB_ROOT", blobRoot)
			err := runMigrationWithAttemptJournal(context.Background(), "not-a-database-connection-string", 0)
			if err == nil || !strings.Contains(err.Error(), "overlaps the blob root") {
				t.Fatalf("migration error=%v, want CAS-overlap refusal", err)
			}
			if strings.Contains(err.Error(), "database") || strings.Contains(err.Error(), "connection string") {
				t.Fatalf("migration reached database setup before path isolation: %v", err)
			}
		})
	}
}

func TestMigrationRefusesAttemptJournalUnderSymlinkedBlobRoot(t *testing.T) {
	base := t.TempDir()
	blobRoot := filepath.Join(base, "cas")
	if err := os.Mkdir(blobRoot, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "cas-alias")
	if err := os.Symlink(blobRoot, alias); err != nil {
		t.Skipf("directory symlinks are unavailable: %v", err)
	}
	journalRoot := filepath.Join(alias, "attempts")
	t.Setenv("POLIS_MIGRATION_ATTEMPT_ROOT", journalRoot)
	t.Setenv("POLIS_BLOB_ROOT", blobRoot)

	err := runMigrationWithAttemptJournal(context.Background(), "not-a-database-connection-string", 0)
	if err == nil || !strings.Contains(err.Error(), "overlaps the blob root") {
		t.Fatalf("migration error=%v, want symlinked CAS-overlap refusal", err)
	}
}

func TestMigratePersistsSanitizedAttemptWhenDatabaseSetupFails(t *testing.T) {
	journalRoot := t.TempDir()
	t.Setenv("POLIS_MIGRATION_ATTEMPT_ROOT", journalRoot)
	t.Setenv("POLIS_DSN", "not-a-database-connection-string")
	originalArgs := os.Args
	os.Args = []string{"polis", "migrate"}
	defer func() { os.Args = originalArgs }()

	if err := run(); err == nil {
		t.Fatal("migration unexpectedly accepted an invalid DSN")
	}
	entries, err := os.ReadDir(journalRoot)
	if err != nil {
		t.Fatalf("migration attempt journal was not created: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("journal records=%d, want immutable started and finished events", len(entries))
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			t.Fatalf("unexpected journal entry %q", entry.Name())
		}
		contents, readErr := os.ReadFile(filepath.Join(journalRoot, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if len(contents) == 0 || bytes.Contains(contents, []byte("not-a-database-connection-string")) {
			t.Fatalf("attempt record is empty or contains the DSN: %s", contents)
		}
		var record struct {
			AttemptID   string `json:"attemptId"`
			Event       string `json:"event"`
			Target      int64  `json:"targetVersion"`
			ManifestSHA string `json:"migrationManifestSha256"`
			Result      string `json:"result"`
			FailureCode string `json:"failureCode"`
		}
		if err = json.Unmarshal(contents, &record); err != nil {
			t.Fatalf("decode %q: %v", entry.Name(), err)
		}
		if record.AttemptID == "" || record.Target != 0 || len(record.ManifestSHA) != 64 {
			t.Fatalf("attempt identity/target/manifest missing: %+v", record)
		}
		seen[record.Event] = true
		if record.Event == "finished" && (record.Result != "outcome_unknown" || record.FailureCode != "migration_call_failed") {
			t.Fatalf("failed migration result was not classified without raw error text: %+v", record)
		}
	}
	if !seen["started"] || !seen["finished"] {
		t.Fatalf("journal event pair incomplete: %v", seen)
	}
}

func TestMigrationAttemptWithoutCompletionRemainsUnknown(t *testing.T) {
	root := t.TempDir()
	started, err := beginMigrationAttempt(root, 60)
	if err != nil {
		t.Fatal(err)
	}
	statuses, err := readMigrationAttemptStatuses(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || statuses[0].AttemptID != started.AttemptID || statuses[0].Status != "outcome_unknown" || statuses[0].FinishedAt != "" {
		t.Fatalf("crash-left migration attempt status=%+v, want one incomplete outcome_unknown record", statuses)
	}
}

func TestConcurrentMigrationAttemptStartsShareNewJournalRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "journal")
	const concurrentAttempts = 12
	var wait sync.WaitGroup
	errorsByAttempt := make(chan error, concurrentAttempts)
	wait.Add(concurrentAttempts)
	for range concurrentAttempts {
		go func() {
			defer wait.Done()
			_, err := beginMigrationAttempt(root, 60)
			errorsByAttempt <- err
		}()
	}
	wait.Wait()
	close(errorsByAttempt)
	for err := range errorsByAttempt {
		if err != nil {
			t.Fatalf("concurrent migration attempt failed: %v", err)
		}
	}
	statuses, err := readMigrationAttemptStatuses(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != concurrentAttempts {
		t.Fatalf("journal contains %d attempts, want %d", len(statuses), concurrentAttempts)
	}
}

func TestMigrationAttemptReaderRejectsUnexpectedJournalEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "unexpected-directory"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := readMigrationAttemptStatuses(root); err == nil {
		t.Fatal("journal reader accepted an unexpected directory entry")
	}

	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leftover.tmp"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readMigrationAttemptStatuses(root); err == nil {
		t.Fatal("journal reader accepted an unexpected temporary file")
	}
}
