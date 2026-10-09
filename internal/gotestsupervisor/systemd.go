// pattern: Imperative Shell
package gotestsupervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"polis/internal/core"
	"polis/internal/gotestprofile"
)

const InstalledBinary = "/usr/local/libexec/polis-selfdev-go-job-supervisor"

// Executor is a fixed Go-profile adapter, not a shell or another scheduler.
type Executor interface {
	Readiness(context.Context, string) HostCheck
	Run(context.Context, Request) Report
	Stop(context.Context, string) error
}

type SystemdExecutor struct{ binary, toolchain string }

func NewSystemdExecutor(binary, toolchain string) *SystemdExecutor {
	return &SystemdExecutor{binary: binary, toolchain: toolchain}
}

type commandBuffer struct {
	bytes.Buffer
	limit int
}

func (b *commandBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, io.ErrShortWrite
	}
	return b.Buffer.Write(p)
}

func commandOutput(ctx context.Context, path string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, path, args...)
	c.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	var out commandBuffer
	out.limit = 65536
	c.Stdout = &out
	c.Stderr = io.Discard
	err := c.Run()
	return out.Bytes(), err
}

func (e *SystemdExecutor) Readiness(ctx context.Context, id string) HostCheck {
	h := HostCheck{Reason: "supervisor_not_installed", Limits: gotestprofile.FixtureManifest(strings.Repeat("0", 64)).Limits}
	if runtime.GOOS != "linux" || e.binary != InstalledBinary || !core.ValidID(id) {
		return h
	}
	qualificationSHA, trusted := trustedInstalledRunner(e.binary)
	if !trusted {
		h.Reason = "installed_runner_or_seal_untrusted"
		return h
	}
	if e.toolchain == "" || !filepath.IsAbs(e.toolchain) || filepath.Clean(e.toolchain) != e.toolchain {
		h.Reason = "toolchain_root_not_configured"
		return h
	}
	raw, err := commandOutput(ctx, e.binary, "--inspect-host", "--job-id="+id, "--toolchain-root="+e.toolchain)
	if json.Unmarshal(raw, &h) != nil {
		h.Qualified = false
		h.Reason = "supervisor_preflight_unavailable"
		return h
	}
	if err != nil {
		h.Qualified = false
	}
	if h.QualificationSHA256 != qualificationSHA {
		h.Qualified = false
		h.Reason = "qualification_identity_mismatch"
	}
	return h
}

func unitName(id string) (string, error) {
	if !core.ValidID(id) || !strings.HasPrefix(id, "job-") {
		return "", ErrInvalid
	}
	return "polis-self-dev-go-job@" + id + ".service", nil
}

func unitProperties(ctx context.Context, unit string) (map[string]string, error) {
	raw, err := commandOutput(ctx, "/usr/bin/systemctl", "show", unit, "--property=ActiveState", "--property=SubState", "--property=Result", "--property=ExecMainPID", "--property=ExecMainStatus", "--property=InvocationID", "--property=ControlGroup")
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			out[key] = value
		}
	}
	return out, nil
}

func unitEmpty(unit string) bool {
	path := "/sys/fs/cgroup/system.slice/" + unit
	i, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return true
	}
	if err != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(path, "cgroup.events"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "populated 0" {
			return true
		}
	}
	return false
}

func (e *SystemdExecutor) Stop(ctx context.Context, id string) error {
	unit, err := unitName(id)
	if err != nil {
		return err
	}
	if _, err = commandOutput(ctx, "/usr/bin/systemctl", "stop", unit); err != nil {
		return err
	}
	props, err := unitProperties(ctx, unit)
	if err != nil {
		return err
	}
	if (props["ActiveState"] != "inactive" && props["ActiveState"] != "failed") || !unitEmpty(unit) {
		return errors.New("job_process_tree_cleanup_unconfirmed")
	}
	return nil
}

func readReport(path string) (Report, error) {
	var report Report
	i, err := os.Lstat(path)
	if err != nil || !i.Mode().IsRegular() || i.Mode().Perm()&0077 != 0 || i.Size() > 2<<20 {
		return report, ErrInvalid
	}
	f, err := os.Open(path)
	if err != nil {
		return report, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(i, opened) {
		return report, ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(f, (2<<20)+1))
	if err != nil || int64(len(raw)) != i.Size() {
		return report, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&report) != nil {
		return report, ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return report, ErrInvalid
	}
	return report, nil
}

func (e *SystemdExecutor) Run(ctx context.Context, r Request) Report {
	h := e.Readiness(ctx, r.Binding.JobRunID)
	report := NewUnstartedReport(r.Binding, "real", "blocked", "host_profile_unqualified", h)
	if !h.Qualified {
		return report
	}
	if ValidateRequest(r) != nil || r.HostQualificationSHA256 != h.QualificationSHA256 {
		report.State, report.Reason = "failed", "dispatch_request_mismatch"
		return report
	}
	unit, err := unitName(r.Binding.JobRunID)
	if err != nil {
		report.State, report.Reason = "failed", "invalid_job_unit"
		return report
	}
	base := "/run/polis-self-dev-go-job"
	i, err := os.Lstat(base)
	if err != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm()&0077 != 0 {
		report.State, report.Reason = "failed", "runtime_handoff_not_prepared"
		return report
	}
	dir := filepath.Join(base, r.Binding.JobRunID)
	if os.Mkdir(dir, 0700) != nil {
		report.State, report.Reason = "failed", "runtime_handoff_already_exists"
		return report
	}
	raw, _ := json.Marshal(r)
	f, err := os.OpenFile(filepath.Join(dir, "request.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		_, err = f.Write(raw)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil {
		report.State, report.Reason = "failed", "request_handoff_failed"
		return report
	}
	if _, err = commandOutput(ctx, "/usr/bin/systemctl", "start", "--no-block", unit); err != nil {
		// A timed-out D-Bus command may already have queued the unit. Reconcile
		// the whole group; never infer that candidate execution did not start.
		report.DispatchAccepted = true
		report.State, report.Reason = "outcome_unknown", "systemd_start_unconfirmed"
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		report.CleanupConfirmed = e.Stop(stopCtx, r.Binding.JobRunID) == nil
		return report
	}
	report.DispatchAccepted = true
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.Binding.Profile.Limits.WallTimeoutMS)*time.Millisecond+15*time.Second)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	invocation := ""
	for {
		props, observeErr := unitProperties(ctx, unit)
		if observeErr == nil {
			if props["InvocationID"] != "" {
				if invocation != "" && invocation != props["InvocationID"] {
					break
				}
				invocation = props["InvocationID"]
			}
			pid, _ := strconv.Atoi(props["ExecMainPID"])
			report.LauncherStarted = pid > 0
			if unitTerminal(props) {
				result, resultErr := readReport(filepath.Join(dir, "result.json"))
				if resultErr == nil && ValidateReport(r.Binding, result) == nil && result.Host.QualificationSHA256 == r.HostQualificationSHA256 && result.Host.BootID == h.BootID {
					result.DispatchAccepted = true
					if result.QualificationEligible && (props["Result"] != "success" || props["ExecMainStatus"] != "0" || invocation == "" || !unitEmpty(unit)) {
						break
					}
					return result
				}
				break
			}
		}
		select {
		case <-ctx.Done():
			goto stop
		case <-ticker.C:
		}
	}
stop:
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()
	report.CleanupConfirmed = e.Stop(stopCtx, r.Binding.JobRunID) == nil
	report.State, report.Reason = "outcome_unknown", "supervisor_result_missing"
	// A missing receipt cannot establish candidate start/exit, even when
	// systemd accepted dispatch. Never invent exit code zero or a test PASS.
	return report
}

// Collect observes an existing unit/result only. It never calls start, writes
// request.json, or replays a job after loss of process-local state.
func (e *SystemdExecutor) Collect(ctx context.Context, r Request) Report {
	h := e.Readiness(ctx, r.Binding.JobRunID)
	report := NewUnstartedReport(r.Binding, "real", "outcome_unknown", "result_collection_unconfirmed", h)
	report.DispatchAccepted = true
	report.CleanupConfirmed = false
	if !h.Qualified {
		return report
	}
	unit, err := unitName(r.Binding.JobRunID)
	if err != nil {
		return report
	}
	raw, err := validatePrivateRequest(managedRequestPath(r.Binding.JobRunID))
	if err != nil {
		return report
	}
	issued, err := ParseRequest(raw)
	if err != nil || issued.BindingSHA256 != r.BindingSHA256 || issued.HostQualificationSHA256 != h.QualificationSHA256 {
		return report
	}
	props, err := unitProperties(ctx, unit)
	if err == nil && (props["ActiveState"] == "active" || props["ActiveState"] == "activating" || props["ActiveState"] == "inactive" && props["InvocationID"] == "") {
		report.State = "running"
		return report
	}
	if err != nil || props["InvocationID"] == "" || (props["ActiveState"] != "inactive" && props["ActiveState"] != "failed") {
		return report
	}
	result, err := readReport(filepath.Join(filepath.Dir(managedRequestPath(r.Binding.JobRunID)), "result.json"))
	if err != nil || ValidateReport(r.Binding, result) != nil || result.Host.BootID != h.BootID || result.Host.QualificationSHA256 != h.QualificationSHA256 || !unitEmpty(unit) {
		return report
	}
	if result.QualificationEligible && (props["Result"] != "success" || props["ExecMainStatus"] != "0") {
		return report
	}
	result.DispatchAccepted = true
	return result
}
