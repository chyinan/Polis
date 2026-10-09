//go:build linux

// pattern: Imperative Shell
package gotestsupervisor

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Verify the installed program before executing its self-reported readiness.
// Neither a writable identity sidecar nor an echoing replacement is authority.
func trustedInstalledRunner(path string) (string, bool) {
	if path != InstalledBinary {
		return "", false
	}
	for current := path; current != "/"; current = filepath.Dir(current) {
		i, err := os.Lstat(current)
		if err != nil || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm()&0022 != 0 {
			return "", false
		}
		stat, ok := i.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != 0 {
			return "", false
		}
	}
	raw, err := readRootQualification(QualificationPath)
	if err != nil {
		return "", false
	}
	var qualification Qualification
	if json.Unmarshal(raw, &qualification) != nil {
		return "", false
	}
	sha, err := fileSHA(path)
	if err != nil || sha != qualification.RunnerSHA256 {
		return "", false
	}
	return SHA256(raw), true
}

func readRootQualification(path string) ([]byte, error) {
	i, e := os.Lstat(path)
	if e != nil || !i.Mode().IsRegular() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm()&0022 != 0 || i.Size() > 65536 {
		return nil, ErrInvalid
	}
	s, ok := i.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != 0 {
		return nil, ErrInvalid
	}
	return os.ReadFile(path)
}
func checkSystemdUnit(ctx context.Context, id string) error {
	ctx, c := context.WithTimeout(ctx, 3*time.Second)
	defer c()
	cmd := exec.CommandContext(ctx, "/usr/bin/systemctl", "show", "polis-self-dev-go-job@"+id+".service", "--property=RuntimeMaxUSec", "--property=KillMode", "--property=TimeoutStopUSec")
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	raw, e := cmd.Output()
	if e != nil {
		return e
	}
	if !strings.Contains(string(raw), "RuntimeMaxUSec=2min\n") && !strings.Contains(string(raw), "RuntimeMaxUSec=120000000\n") {
		return ErrInvalid
	}
	if !strings.Contains(string(raw), "KillMode=control-group\n") || !strings.Contains(string(raw), "TimeoutStopUSec=1s\n") {
		return ErrInvalid
	}
	return nil
}
