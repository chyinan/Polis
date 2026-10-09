//go:build linux

// pattern: Imperative Shell
package gotestsupervisor

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

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
