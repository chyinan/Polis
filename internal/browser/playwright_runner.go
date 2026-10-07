// pattern: Imperative Shell
package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const browserRunnerOutputLimit = 2 << 20

type PlaywrightRunner struct {
	PythonPath        string
	PythonPackageRoot string
	ScriptPath        string
	ProfileRoot       string
	BrowserExecutable string
}

type playwrightRunnerPayload struct {
	BrowserRunPlan
	ProfileDir        string `json:"profile_dir"`
	BrowserExecutable string `json:"browser_executable,omitempty"`
}

func (runner PlaywrightRunner) Run(ctx context.Context, plan BrowserRunPlan) (BrowserRunOutcome, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, err := NormalizeBrowserRunPlan(plan)
	if err != nil {
		return BrowserRunOutcome{}, err
	}
	if runner.PythonPath == "" || runner.ScriptPath == "" || runner.ProfileRoot == "" || !filepath.IsAbs(runner.ScriptPath) || !filepath.IsAbs(runner.ProfileRoot) {
		return BrowserRunOutcome{}, errors.New("browser runner configuration is incomplete")
	}
	if _, err := os.Stat(runner.ScriptPath); err != nil {
		return BrowserRunOutcome{}, fmt.Errorf("browser runner script is unavailable: %w", err)
	}
	if runner.PythonPackageRoot != "" {
		info, err := os.Stat(runner.PythonPackageRoot)
		if err != nil || !info.IsDir() || !filepath.IsAbs(runner.PythonPackageRoot) {
			return BrowserRunOutcome{}, errors.New("browser runner Python package root is unavailable")
		}
	}
	if info, err := os.Stat(runner.ProfileRoot); err != nil || !info.IsDir() {
		return BrowserRunOutcome{}, errors.New("browser runner profile root is unavailable")
	}
	profileDir, err := os.MkdirTemp(runner.ProfileRoot, "polis-browser-")
	if err != nil {
		return BrowserRunOutcome{}, fmt.Errorf("create isolated browser profile: %w", err)
	}
	defer os.RemoveAll(profileDir)
	entries, err := os.ReadDir(profileDir)
	if err != nil || len(entries) != 0 {
		return BrowserRunOutcome{}, errors.New("isolated browser profile was not empty")
	}
	payload, err := json.Marshal(playwrightRunnerPayload{BrowserRunPlan: normalized, ProfileDir: profileDir, BrowserExecutable: runner.BrowserExecutable})
	if err != nil {
		return BrowserRunOutcome{}, fmt.Errorf("encode browser runner plan: %w", err)
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(normalized.TimeoutMS+2000)*time.Millisecond)
	defer cancel()
	command := exec.CommandContext(runCtx, runner.PythonPath, runner.ScriptPath)
	command.Env = controlledBrowserEnvironment(profileDir, runner.PythonPackageRoot)
	command.Stdin = bytes.NewReader(payload)
	stdout := &cappedBuffer{limit: browserRunnerOutputLimit}
	stderr := &cappedBuffer{limit: browserRunnerOutputLimit}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return BrowserRunOutcome{}, context.DeadlineExceeded
		}
		return BrowserRunOutcome{}, fmt.Errorf("playwright runner failed: %w: %s", err, stderr.String())
	}
	if stdout.overflow || stderr.overflow {
		return BrowserRunOutcome{}, errors.New("playwright runner output exceeds its bound")
	}
	var outcome BrowserRunOutcome
	if err := json.Unmarshal(stdout.Bytes(), &outcome); err != nil {
		return BrowserRunOutcome{}, fmt.Errorf("decode playwright runner result: %w", err)
	}
	if err := ValidateBrowserRunOutcome(normalized, outcome); err != nil {
		return outcome, err
	}
	return outcome, nil
}

func controlledBrowserEnvironment(profileDir, pythonPackageRoot string) []string {
	environment := []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + profileDir,
		"TMPDIR=" + profileDir,
		"TEMP=" + profileDir,
		"TMP=" + profileDir,
	}
	if pythonPackageRoot != "" {
		environment = append(environment, "PYTHONPATH="+pythonPackageRoot)
	}
	if runtime.GOOS == "windows" {
		if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
			environment = append(environment, "SystemRoot="+systemRoot)
		}
	}
	return environment
}

type cappedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (buffer *cappedBuffer) Write(data []byte) (int, error) {
	if len(data) > buffer.limit-buffer.Len() {
		remaining := buffer.limit - buffer.Len()
		if remaining > 0 {
			_, _ = buffer.Buffer.Write(data[:remaining])
		}
		buffer.overflow = true
		return len(data), io.ErrShortWrite
	}
	return buffer.Buffer.Write(data)
}
