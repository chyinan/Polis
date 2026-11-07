package recovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePostgresUnixSocketEndpointAcceptsShortEndpoint(t *testing.T) {
	base, err := os.MkdirTemp("/tmp", "polis-socket-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(base)
	dir := filepath.Join(base, "socket")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	report, err := ValidatePostgresUnixSocketEndpoint(dir, 55432)
	if err != nil || report.Compatibility != "PASS" || report.PathBytes != len([]byte(report.EffectiveSocketPath)) {
		t.Fatalf("report=%+v err=%v", report, err)
	}
}

func TestValidatePostgresUnixSocketEndpointUsesBytesAndFailsOverlong(t *testing.T) {
	base, err := os.MkdirTemp("/tmp", "polis-socket-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(base)
	name := strings.Repeat("x", 100)
	dir := filepath.Join(base, name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = ValidatePostgresUnixSocketEndpoint(dir, 55432)
	if err == nil || !strings.Contains(err.Error(), "FRESH_RESTORE_SOCKET_PATH_TOO_LONG") {
		t.Fatalf("error=%v", err)
	}
}

func TestValidatePostgresUnixSocketEndpointRejectsInvalidDirectoryPortAndCollision(t *testing.T) {
	base, err := os.MkdirTemp("/tmp", "polis-socket-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(base)
	if _, err := ValidatePostgresUnixSocketEndpoint(filepath.Join(base, "missing"), 55432); err == nil {
		t.Fatal("missing directory accepted")
	}
	dir := filepath.Join(base, "socket")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidatePostgresUnixSocketEndpoint(dir, 0); err == nil {
		t.Fatal("invalid port accepted")
	}
	report, err := ValidatePostgresUnixSocketEndpoint(dir, 55432)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report.EffectiveSocketPath, []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidatePostgresUnixSocketEndpoint(dir, 55432); err == nil || !strings.Contains(err.Error(), "socket_namespace_collision") {
		t.Fatalf("collision error=%v", err)
	}
}
