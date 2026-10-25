// Command polis-selfdev-go-unit-control brokers two fixed systemd operations.
// It accepts no unit name, shell command, or systemctl arguments from callers.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"polis/internal/core"
)

const (
	protocol          = "polis-selfdev-go-unit-control@1"
	systemctlPath     = "/usr/bin/systemctl"
	systemctlTimeout  = 8 * time.Second
	targetUID         = "1000"
	sealPath          = "/etc/systemd/system/polis-self-dev-go-unit-control.sha256"
	policyPath        = "/usr/share/polkit-1/actions/org.polis.selfdev.go-job.policy"
	pklaPath          = "/etc/polkit-1/localauthority/50-local.d/50-polis-selfdev-go-unit-control.pkla"
	sealMarker        = "# POLIS_SELF_DEV_GO_JOB_HOST_V1"
	maxSealedFileSize = 16 << 20
)

type systemdRunner func(context.Context, string, ...string) error
type installationVerifier func() error

func systemdArgs(operation, jobID string) ([]string, error) {
	if !core.ValidID(jobID) || !strings.HasPrefix(jobID, "job-") || len(jobID) == len("job-") {
		return nil, errors.New("invalid job identity")
	}
	unit := "polis-self-dev-go-job@" + jobID + ".service"
	switch operation {
	case "job-start":
		return []string{"start", "--no-block", unit}, nil
	case "job-stop":
		return []string{"stop", unit}, nil
	default:
		return nil, errors.New("unsupported operation")
	}
}

func execute(args []string, effectiveUID int, pkexecUID string, stdout, stderr io.Writer, run systemdRunner, verify installationVerifier) int {
	if len(args) == 1 && args[0] == "--protocol-info" {
		_, _ = fmt.Fprintln(stdout, protocol)
		return 0
	}
	if effectiveUID != 0 {
		_, _ = fmt.Fprintln(stderr, "unit control requires the Polkit-authorized root invocation")
		return 77
	}
	if pkexecUID != targetUID {
		_, _ = fmt.Fprintln(stderr, "unit control caller is not authorized")
		return 77
	}
	if len(args) == 1 && args[0] == "--verify-installation" {
		if verify == nil {
			verify = verifyInstallation
		}
		if err := verify(); err != nil {
			_, _ = fmt.Fprintf(stderr, "unit-control installation seal is invalid: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintln(stdout, "unit_control_trusted=yes")
		return 0
	}
	if len(args) != 2 {
		_, _ = fmt.Fprintln(stderr, "invalid unit control arguments")
		return 64
	}
	systemdCommand, err := systemdArgs(args[0], args[1])
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "invalid unit control request")
		return 64
	}
	if run == nil {
		run = runSystemd
	}
	ctx, cancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer cancel()
	if err = run(ctx, systemctlPath, systemdCommand...); err != nil {
		_, _ = fmt.Fprintln(stderr, "fixed systemd operation failed")
		return 1
	}
	_, _ = fmt.Fprintln(stdout, "unit_control=accepted")
	return 0
}

func verifyInstallation() error {
	return verifyInstallationFiles(sealPath, "/usr/local/libexec/polis-selfdev-go-unit-control", policyPath, pklaPath)
}

func verifyInstallationFiles(sealFile, helperFile, policyFile, pklaFile string) error {
	seal, err := os.ReadFile(sealFile)
	if err != nil || len(seal) > 4096 {
		return fmt.Errorf("unit-control seal file is unavailable: %s", filepath.Base(sealFile))
	}
	lines := strings.Split(strings.TrimSpace(string(seal)), "\n")
	if len(lines) != 4 || lines[0] != sealMarker {
		return errors.New("unit-control seal is malformed")
	}
	for index, path := range []string{helperFile, policyFile, pklaFile} {
		digest, err := boundedSHA256(path)
		if err != nil || digest != lines[index+1] {
			return fmt.Errorf("unit-control sealed file digest mismatch: %s actual=%s expected=%s err=%v", filepath.Base(path), digest, lines[index+1], err)
		}
	}
	return nil
}

func boundedSHA256(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > maxSealedFileSize {
		return "", fmt.Errorf("invalid unit-control sealed file: %s", filepath.Base(path))
	}
	content, err := os.ReadFile(path)
	if err != nil || int64(len(content)) != info.Size() {
		return "", fmt.Errorf("unit-control sealed file read failed: %s", filepath.Base(path))
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

func runSystemd(ctx context.Context, path string, args ...string) error {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func main() {
	os.Exit(execute(os.Args[1:], os.Geteuid(), os.Getenv("PKEXEC_UID"), os.Stdout, os.Stderr, nil, nil))
}
