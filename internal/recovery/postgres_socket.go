// pattern: Functional Core
package recovery

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const PostgreSQLUnixSocketMaxBytes = 107

type PostgreSQLUnixSocketEndpoint struct {
	ConfiguredDirectory string `json:"configured_directory"`
	ResolvedDirectory   string `json:"resolved_directory"`
	Port                int    `json:"port"`
	EffectiveSocketPath string `json:"effective_socket_path"`
	PathBytes           int    `json:"path_bytes"`
	MaximumBytes        int    `json:"maximum_bytes"`
	Compatibility       string `json:"compatibility"`
}

type PostgreSQLUnixSocketError struct {
	ReasonCode string
	Path       string
}

func (e *PostgreSQLUnixSocketError) Error() string {
	if e.Path == "" {
		return e.ReasonCode
	}
	return fmt.Sprintf("%s: %s", e.ReasonCode, e.Path)
}

func ValidatePostgresUnixSocketEndpoint(directory string, port int) (PostgreSQLUnixSocketEndpoint, error) {
	if directory == "" {
		return PostgreSQLUnixSocketEndpoint{}, &PostgreSQLUnixSocketError{ReasonCode: "socket_directory_missing"}
	}
	abs, err := filepath.Abs(directory)
	if err != nil {
		return PostgreSQLUnixSocketEndpoint{}, err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return PostgreSQLUnixSocketEndpoint{}, &PostgreSQLUnixSocketError{ReasonCode: "socket_directory_not_found", Path: abs}
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return PostgreSQLUnixSocketEndpoint{}, &PostgreSQLUnixSocketError{ReasonCode: "socket_directory_not_directory", Path: resolved}
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
		return PostgreSQLUnixSocketEndpoint{}, &PostgreSQLUnixSocketError{ReasonCode: "socket_directory_unsafe_permissions", Path: resolved}
	}
	if port < 1 || port > 65535 {
		return PostgreSQLUnixSocketEndpoint{}, &PostgreSQLUnixSocketError{ReasonCode: "socket_port_invalid"}
	}
	effective := filepath.Join(resolved, fmt.Sprintf(".s.PGSQL.%d", port))
	pathBytes := len([]byte(effective))
	if pathBytes > PostgreSQLUnixSocketMaxBytes {
		return PostgreSQLUnixSocketEndpoint{ConfiguredDirectory: directory, ResolvedDirectory: resolved, Port: port, EffectiveSocketPath: effective, PathBytes: pathBytes, MaximumBytes: PostgreSQLUnixSocketMaxBytes, Compatibility: "FAIL"}, &PostgreSQLUnixSocketError{ReasonCode: "FRESH_RESTORE_SOCKET_PATH_TOO_LONG", Path: effective}
	}
	if _, err := os.Lstat(effective); err == nil {
		return PostgreSQLUnixSocketEndpoint{ConfiguredDirectory: directory, ResolvedDirectory: resolved, Port: port, EffectiveSocketPath: effective, PathBytes: pathBytes, MaximumBytes: PostgreSQLUnixSocketMaxBytes, Compatibility: "FAIL"}, &PostgreSQLUnixSocketError{ReasonCode: "socket_namespace_collision", Path: effective}
	} else if !os.IsNotExist(err) {
		return PostgreSQLUnixSocketEndpoint{}, err
	}
	return PostgreSQLUnixSocketEndpoint{ConfiguredDirectory: directory, ResolvedDirectory: resolved, Port: port, EffectiveSocketPath: effective, PathBytes: pathBytes, MaximumBytes: PostgreSQLUnixSocketMaxBytes, Compatibility: "PASS"}, nil
}
