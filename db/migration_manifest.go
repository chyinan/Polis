// pattern: Functional Core
package db

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

func MigrationManifestSHA256() string {
	digest := sha256.Sum256(migrationHashManifest)
	return hex.EncodeToString(digest[:])
}

func verifyMigrationManifest(files fs.FS, manifest []byte) error {
	if len(manifest) == 0 || manifest[len(manifest)-1] != '\n' {
		return fmt.Errorf("migration hash manifest is empty or not newline terminated")
	}
	lines := strings.Split(strings.TrimSuffix(string(manifest), "\n"), "\n")
	expected := make(map[string]string, len(lines))
	versions := make(map[int]string, len(lines))
	lastName := ""
	for _, line := range lines {
		parts := strings.Split(line, "  ")
		if len(parts) != 2 || len(parts[0]) != 64 || parts[0] != strings.ToLower(parts[0]) || !isLowerHex(parts[0]) {
			return fmt.Errorf("migration hash manifest contains an invalid entry")
		}
		name := parts[1]
		version, versionErr := parseGooseMigrationVersion(name)
		if versionErr != nil || !fs.ValidPath(name) || path.Base(name) != name || name <= lastName {
			return fmt.Errorf("migration hash manifest entry order or path is invalid")
		}
		if _, exists := versions[version]; exists {
			return fmt.Errorf("migration hash manifest contains duplicate Goose version %d", version)
		}
		lastName = name
		expected[name] = parts[0]
		versions[version] = name
	}

	entries, err := fs.ReadDir(files, "migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		actual = append(actual, entry.Name())
	}
	sort.Strings(actual)
	if len(actual) != len(expected) {
		return fmt.Errorf("migration hash manifest does not cover the exact embedded SQL set")
	}
	for index, name := range actual {
		if index >= len(lines) || lines[index] == "" || expected[name] == "" {
			return fmt.Errorf("migration hash manifest does not cover %q", name)
		}
		content, err := fs.ReadFile(files, path.Join("migrations", name))
		if err != nil {
			return fmt.Errorf("read embedded migration %q: %w", name, err)
		}
		digest := sha256.Sum256(content)
		if got := hex.EncodeToString(digest[:]); got != expected[name] {
			return fmt.Errorf("embedded migration %q does not match its pinned SHA-256", name)
		}
	}
	return nil
}

func parseGooseMigrationVersion(name string) (int, error) {
	if len(name) < 11 || name[5] != '_' || !strings.HasSuffix(name, ".sql") {
		return 0, fmt.Errorf("noncanonical Goose migration name")
	}
	for _, char := range name[:5] {
		if char < '0' || char > '9' {
			return 0, fmt.Errorf("migration version is not numeric")
		}
	}
	version, err := strconv.Atoi(name[:5])
	if err != nil || version <= 0 {
		return 0, fmt.Errorf("migration version must be positive")
	}
	return version, nil
}

func isLowerHex(value string) bool {
	for _, char := range value {
		if !('0' <= char && char <= '9') && !('a' <= char && char <= 'f') {
			return false
		}
	}
	return true
}
