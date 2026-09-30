// pattern: Imperative Shell
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"polis/db"
)

const migrationAttemptSchema = "polis-migration-attempt@1"
const maxMigrationAttemptRecords = 10000
const maxMigrationAttemptRecordBytes = 4096

type migrationAttemptEvent struct {
	SchemaVersion  string `json:"schemaVersion"`
	AttemptID      string `json:"attemptId"`
	Event          string `json:"event"`
	TargetVersion  int64  `json:"targetVersion"`
	ManifestSHA256 string `json:"migrationManifestSha256"`
	BuildID        string `json:"buildId"`
	At             string `json:"at"`
	Result         string `json:"result,omitempty"`
	FailureCode    string `json:"failureCode,omitempty"`
}

type migrationAttemptStatus struct {
	AttemptID      string `json:"attemptId"`
	TargetVersion  int64  `json:"targetVersion"`
	ManifestSHA256 string `json:"migrationManifestSha256"`
	BuildID        string `json:"buildId"`
	StartedAt      string `json:"startedAt"`
	FinishedAt     string `json:"finishedAt,omitempty"`
	Status         string `json:"status"`
	FailureCode    string `json:"failureCode,omitempty"`
}

func migrationAttemptRoot() string {
	if root := strings.TrimSpace(os.Getenv("POLIS_MIGRATION_ATTEMPT_ROOT")); root != "" {
		return root
	}
	blobRoot := strings.TrimSpace(os.Getenv("POLIS_BLOB_ROOT"))
	if blobRoot == "" {
		return ""
	}
	absoluteBlobRoot, err := filepath.Abs(filepath.Clean(blobRoot))
	if err != nil || filepath.Clean(absoluteBlobRoot) == string(filepath.Separator) || filepath.Clean(absoluteBlobRoot) == filepath.VolumeName(absoluteBlobRoot)+string(filepath.Separator) {
		return ""
	}
	return filepath.Join(filepath.Dir(absoluteBlobRoot), "migration-attempts")
}

func runMigrationWithAttemptJournal(ctx context.Context, dsn string, targetVersion int64) error {
	root, err := requireMigrationAttemptRoot()
	if err != nil {
		return err
	}
	started, err := beginMigrationAttempt(root, targetVersion)
	if err != nil {
		return fmt.Errorf("refusing migration because its attempt could not be journaled: %w", err)
	}
	migrationErr := db.MigrateToVersion(ctx, dsn, targetVersion)
	result, failureCode := "applied", ""
	if migrationErr != nil {
		result, failureCode = "outcome_unknown", "migration_call_failed"
	}
	finishErr := finishMigrationAttempt(root, started, result, failureCode)
	if migrationErr != nil {
		if finishErr != nil {
			return errors.Join(migrationErr, fmt.Errorf("migration attempt completion could not be journaled: %w", finishErr))
		}
		return migrationErr
	}
	if finishErr != nil {
		return fmt.Errorf("migration succeeded but its attempt completion could not be journaled; inspect migration execution status before retrying: %w", finishErr)
	}
	return nil
}

func beginMigrationAttempt(root string, targetVersion int64) (migrationAttemptEvent, error) {
	if targetVersion < 0 {
		return migrationAttemptEvent{}, errors.New("migration target version must be nonnegative")
	}
	root, err := canonicalMigrationAttemptRoot(root, os.Getenv("POLIS_BLOB_ROOT"))
	if err != nil {
		return migrationAttemptEvent{}, err
	}
	if err = ensureMigrationAttemptRoot(root); err != nil {
		return migrationAttemptEvent{}, err
	}
	var rawID [16]byte
	if _, err := rand.Read(rawID[:]); err != nil {
		return migrationAttemptEvent{}, fmt.Errorf("create migration attempt identity: %w", err)
	}
	event := migrationAttemptEvent{
		SchemaVersion:  migrationAttemptSchema,
		AttemptID:      hex.EncodeToString(rawID[:]),
		Event:          "started",
		TargetVersion:  targetVersion,
		ManifestSHA256: db.MigrationManifestSHA256(),
		BuildID:        migrationAttemptBuildID(),
		At:             time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err = writeMigrationAttemptEvent(root, event); err != nil {
		return migrationAttemptEvent{}, err
	}
	return event, nil
}

func finishMigrationAttempt(root string, started migrationAttemptEvent, result, failureCode string) error {
	if started.Event != "started" || !validMigrationAttemptID(started.AttemptID) || started.SchemaVersion != migrationAttemptSchema ||
		(result != "applied" && result != "outcome_unknown") || (result == "applied" && failureCode != "") ||
		(result == "outcome_unknown" && failureCode != "migration_call_failed") {
		return errors.New("migration attempt completion record is invalid")
	}
	root, err := canonicalMigrationAttemptRoot(root, os.Getenv("POLIS_BLOB_ROOT"))
	if err != nil {
		return err
	}
	if err = ensureMigrationAttemptRoot(root); err != nil {
		return err
	}
	started.Event = "finished"
	started.At = time.Now().UTC().Format(time.RFC3339Nano)
	started.Result = result
	started.FailureCode = failureCode
	return writeMigrationAttemptEvent(root, started)
}

func writeMigrationAttemptEvent(root string, event migrationAttemptEvent) error {
	canonicalRoot, err := canonicalMigrationAttemptRoot(root, os.Getenv("POLIS_BLOB_ROOT"))
	if err != nil {
		return err
	}
	contents, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode migration attempt event: %w", err)
	}
	contents = append(contents, '\n')
	path := filepath.Join(canonicalRoot, event.AttemptID+"."+event.Event+".json")
	if err = persistMigrationAttemptEvent(canonicalRoot, path, contents); err != nil {
		return fmt.Errorf("persist immutable migration attempt event: %w", err)
	}
	return nil
}

func ensureMigrationAttemptRoot(root string) error {
	if err := validateMigrationAttemptRoot(root); err != nil {
		return err
	}
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		if createErr := createMigrationAttemptRoot(root); createErr != nil && !migrationAttemptRootAlreadyExists(createErr) {
			return fmt.Errorf("create migration attempt directory: %w", createErr)
		}
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.Join(errors.New("migration attempt root is missing, linked or not a directory"), err)
	}
	if err = secureMigrationAttemptRoot(root); err != nil {
		return fmt.Errorf("secure migration attempt directory: %w", err)
	}
	if err = syncMigrationAttemptRootParent(root); err != nil {
		return fmt.Errorf("persist migration attempt directory entry: %w", err)
	}
	return nil
}

func migrationAttemptRootAlreadyExists(err error) bool {
	return errors.Is(err, os.ErrExist)
}

func validateMigrationAttemptRoot(root string) error {
	cleanRoot := filepath.Clean(root)
	if !filepath.IsAbs(root) || cleanRoot != root || migrationAttemptIsVolumeRoot(cleanRoot, filepath.VolumeName(cleanRoot), string(filepath.Separator)) {
		return errors.New("migration attempt root must be a canonical absolute non-root directory")
	}
	return nil
}

func migrationAttemptIsVolumeRoot(path, volume, separator string) bool {
	if separator != "" && path == separator {
		return true
	}
	if volume == "" {
		return false
	}
	return strings.EqualFold(path, volume) || strings.EqualFold(path, volume+separator)
}

func requireMigrationAttemptRoot() (string, error) {
	root := migrationAttemptRoot()
	if root == "" {
		return "", errors.New("migration attempt root is required; set POLIS_MIGRATION_ATTEMPT_ROOT or POLIS_BLOB_ROOT")
	}
	return canonicalMigrationAttemptRoot(root, os.Getenv("POLIS_BLOB_ROOT"))
}

func canonicalMigrationAttemptRoot(root, blobRoot string) (string, error) {
	if err := validateMigrationAttemptRoot(root); err != nil {
		return "", err
	}
	canonicalRoot, err := resolveMigrationAttemptPath(root)
	if err != nil {
		return "", fmt.Errorf("resolve migration attempt root: %w", err)
	}
	if err = validateMigrationAttemptRoot(canonicalRoot); err != nil {
		return "", fmt.Errorf("resolved migration attempt root is invalid: %w", err)
	}
	if strings.TrimSpace(blobRoot) == "" {
		return canonicalRoot, nil
	}
	canonicalBlobRoot, err := resolveMigrationAttemptPath(strings.TrimSpace(blobRoot))
	if err != nil {
		return "", fmt.Errorf("resolve blob root for migration journal isolation: %w", err)
	}
	if migrationPathsOverlap(canonicalRoot, canonicalBlobRoot) {
		return "", errors.New("migration attempt root overlaps the blob root")
	}
	return canonicalRoot, nil
}

func resolveMigrationAttemptPath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	current := absolute
	var missing []string
	for {
		_, statErr := os.Lstat(current)
		if statErr == nil {
			resolved, resolveErr := filepath.EvalSymlinks(current)
			if resolveErr != nil {
				return "", resolveErr
			}
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("path has no resolvable existing ancestor")
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func migrationPathsOverlap(first, second string) bool {
	return migrationPathContains(first, second) || migrationPathContains(second, first)
}

func migrationPathContains(parent, candidate string) bool {
	relative, err := filepath.Rel(parent, candidate)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		relative = strings.ToLower(relative)
	}
	if relative == "." {
		return true
	}
	parentPrefix := ".." + string(filepath.Separator)
	return relative != ".." && !strings.HasPrefix(relative, parentPrefix) && !filepath.IsAbs(relative)
}

func readMigrationAttemptStatuses(root string) ([]migrationAttemptStatus, error) {
	canonicalRoot, err := canonicalMigrationAttemptRoot(root, os.Getenv("POLIS_BLOB_ROOT"))
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(canonicalRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []migrationAttemptStatus{}, nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.Join(errors.New("migration attempt root is missing, linked or not a directory"), err)
	}
	entries, err := os.ReadDir(canonicalRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []migrationAttemptStatus{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(entries) > maxMigrationAttemptRecords {
		return nil, errors.New("migration attempt journal exceeds its record limit")
	}
	starts := make(map[string]migrationAttemptEvent)
	finishes := make(map[string]migrationAttemptEvent)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil, fmt.Errorf("migration attempt journal contains an unexpected entry: %s", entry.Name())
		}
		event, readErr := readMigrationAttemptEvent(filepath.Join(canonicalRoot, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		if entry.Name() != event.AttemptID+"."+event.Event+".json" {
			return nil, errors.New("migration attempt event name does not match its contents")
		}
		target := starts
		if event.Event == "finished" {
			target = finishes
		} else if event.Event != "started" {
			return nil, errors.New("migration attempt event has an invalid phase")
		}
		if _, exists := target[event.AttemptID]; exists {
			return nil, errors.New("migration attempt journal contains a duplicate event")
		}
		target[event.AttemptID] = event
	}
	statuses := make([]migrationAttemptStatus, 0, len(starts))
	for attemptID, started := range starts {
		status := migrationAttemptStatus{
			AttemptID: attemptID, TargetVersion: started.TargetVersion, ManifestSHA256: started.ManifestSHA256,
			BuildID: started.BuildID, StartedAt: started.At, Status: "outcome_unknown",
		}
		if finished, exists := finishes[attemptID]; exists {
			if finished.TargetVersion != started.TargetVersion || finished.ManifestSHA256 != started.ManifestSHA256 || finished.BuildID != started.BuildID {
				return nil, errors.New("migration attempt start/finish identity mismatch")
			}
			if finished.Result != "applied" && finished.Result != "outcome_unknown" {
				return nil, errors.New("migration attempt result is invalid")
			}
			if (finished.Result == "applied" && finished.FailureCode != "") || (finished.Result == "outcome_unknown" && finished.FailureCode != "migration_call_failed") {
				return nil, errors.New("migration attempt failure code is invalid")
			}
			status.Status, status.FinishedAt, status.FailureCode = finished.Result, finished.At, finished.FailureCode
		}
		statuses = append(statuses, status)
	}
	for attemptID := range finishes {
		if _, exists := starts[attemptID]; !exists {
			return nil, errors.New("migration attempt journal contains a finish without a start")
		}
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].StartedAt < statuses[j].StartedAt })
	return statuses, nil
}

func readMigrationAttemptEvent(path string) (migrationAttemptEvent, error) {
	var event migrationAttemptEvent
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > maxMigrationAttemptRecordBytes {
		return event, errors.Join(errors.New("migration attempt event is missing, linked or outside its size bound"), err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return event, err
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&event); err != nil {
		return migrationAttemptEvent{}, err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return migrationAttemptEvent{}, errors.New("migration attempt event must contain one JSON object")
	}
	if event.SchemaVersion != migrationAttemptSchema || !validMigrationAttemptID(event.AttemptID) || event.TargetVersion < 0 ||
		len(event.ManifestSHA256) != 64 || !lowerMigrationHex(event.ManifestSHA256) || event.BuildID == "" || len(event.BuildID) > 512 {
		return migrationAttemptEvent{}, errors.New("migration attempt event identity is invalid")
	}
	if _, err = time.Parse(time.RFC3339Nano, event.At); err != nil {
		return migrationAttemptEvent{}, errors.New("migration attempt event timestamp is invalid")
	}
	return event, nil
}

func validMigrationAttemptID(value string) bool {
	return len(value) == 32 && lowerMigrationHex(value)
}

func lowerMigrationHex(value string) bool {
	for _, char := range value {
		if !('0' <= char && char <= '9') && !('a' <= char && char <= 'f') {
			return false
		}
	}
	return true
}

func migrationAttemptBuildID() string {
	version := "unknown"
	if info, ok := debug.ReadBuildInfo(); ok {
		version = info.Main.Version
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				version += ";vcs=" + setting.Value
			}
		}
	}
	buildID := "go=" + runtime.Version() + ";" + version
	if len(buildID) > 512 {
		return buildID[:512]
	}
	return buildID
}
