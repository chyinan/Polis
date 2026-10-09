// pattern: Imperative Shell
package gotestsupervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"polis/internal/core"
	"polis/internal/gotestprofile"
	"polis/internal/runner"
	"runtime"
	"sort"
	"strings"
)

const QualificationPath = "/etc/systemd/system/polis-self-dev-go-job-host-qualification.json"

type Config struct{ ToolchainRoot, Bubblewrap, ScratchRoot, QualificationFile string }

func DefaultConfig() Config {
	return Config{ToolchainRoot: "/opt/polis/go", Bubblewrap: "/usr/bin/bwrap", ScratchRoot: "/scratch", QualificationFile: QualificationPath}
}

type Qualification struct {
	Protocol, ProfileRevision, BootID, RunnerSHA256, ToolchainTreeSHA256, BubblewrapSHA256, ProbeSHA256 string
	Limits                                                                                              gotestprofile.ResourceLimits
}

func fileSHA(path string) (string, error) {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 512<<20 {
		return "", ErrInvalid
	}
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return "", ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(f, (512<<20)+1))
	if e != nil || int64(len(raw)) != info.Size() {
		return "", ErrInvalid
	}
	return SHA256(raw), nil
}
func ExecutableSHA256() (string, error) {
	p, e := os.Executable()
	if e != nil {
		return "", e
	}
	return fileSHA(p)
}
func ToolchainTreeSHA256(root string) (string, error) {
	var refs []string
	var total int64
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return ErrInvalid
		}
		if d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil || !info.Mode().IsRegular() {
			return ErrInvalid
		}
		total += info.Size()
		if len(refs) > 100000 || total > 2<<30 {
			return ErrInvalid
		}
		sha, e := fileSHA(path)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		refs = append(refs, filepath.ToSlash(rel)+"\x00"+sha)
		return nil
	})
	if err != nil || len(refs) == 0 {
		return "", ErrInvalid
	}
	sort.Strings(refs)
	return SHA256([]byte(strings.Join(refs, "\n"))), nil
}
func Inspect(ctx context.Context, id string, cfg Config, expectedQualification string) HostCheck {
	p := gotestprofile.FixtureManifest(strings.Repeat("0", 64))
	h := HostCheck{Reason: "linux_host_required", Limits: p.Limits}
	if runtime.GOOS != "linux" {
		return h
	}
	h.Reason = "supervisor_must_be_unprivileged"
	if os.Geteuid() == 0 {
		return h
	}
	if !core.ValidID(id) {
		h.Reason = "managed_job_id_required"
		return h
	}
	h.Cgroup = "/sys/fs/cgroup/system.slice/polis-self-dev-go-job@" + id + ".service"
	if err := runner.CheckFixedGoControls(h.Cgroup, cfg.ScratchRoot, p.Limits); err != nil {
		h.Reason = err.Error()
		return h
	}
	if err := checkSystemdUnit(ctx, id); err != nil {
		h.Reason = "systemd_wall_or_cleanup_policy_unavailable"
		return h
	}
	h.Reason = "host_qualification_missing"
	raw, e := readRootQualification(cfg.QualificationFile)
	if e != nil {
		return h
	}
	h.QualificationSHA256 = SHA256(raw)
	if expectedQualification != "" && expectedQualification != h.QualificationSHA256 {
		return h
	}
	var q Qualification
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil {
		return h
	}
	boot, e := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if e != nil {
		return h
	}
	h.BootID = strings.TrimSpace(string(boot))
	exe, e := ExecutableSHA256()
	if e != nil {
		return h
	}
	bwrap, e := fileSHA(cfg.Bubblewrap)
	if e != nil {
		return h
	}
	gobin, e := fileSHA(filepath.Join(cfg.ToolchainRoot, "bin", "go"))
	if e != nil || gobin != gotestprofile.FixtureToolchainSHA256 {
		h.Reason = "toolchain_identity_mismatch"
		return h
	}
	tree, e := ToolchainTreeSHA256(cfg.ToolchainRoot)
	if e != nil {
		return h
	}
	if q.Protocol != Protocol || q.ProfileRevision != gotestprofile.FixtureRevision || q.BootID != h.BootID || q.RunnerSHA256 != exe || q.ToolchainTreeSHA256 != tree || q.BubblewrapSHA256 != bwrap || !validDigest(q.ProbeSHA256) || q.Limits != p.Limits {
		h.Reason = "qualification_identity_mismatch"
		return h
	}
	h.Qualified = true
	h.Reason = "qualified"
	return h
}
func Execute(ctx context.Context, r Request, cfg Config) Report {
	if ValidateRequest(r) != nil {
		return NewUnstartedReport(r.Binding, "real", "failed", "invalid_request", HostCheck{})
	}
	h := Inspect(ctx, r.Binding.JobRunID, cfg, r.HostQualificationSHA256)
	if !h.Qualified || r.HostQualificationSHA256 == "" {
		return NewUnstartedReport(r.Binding, "real", "blocked", "host_profile_unqualified", h)
	}
	actual, e := ExecutableSHA256()
	if e != nil || actual != r.Binding.Profile.RunnerSHA256 {
		return NewUnstartedReport(r.Binding, "real", "failed", "runner_identity_mismatch", h)
	}
	ctx, cancel := context.WithTimeout(ctx, 120000000000)
	defer cancel()
	paths := []string{filepath.Join(cfg.ScratchRoot, "source"), filepath.Join(cfg.ScratchRoot, "work"), filepath.Join(cfg.ScratchRoot, "deps")}
	for _, path := range paths {
		if _, e = os.Lstat(path); !os.IsNotExist(e) {
			return NewUnstartedReport(r.Binding, "real", "failed", "scratch_not_empty", h)
		}
	}
	created := []string{}
	cleanup := func() bool {
		ok := true
		for _, path := range created {
			_ = os.Chmod(path, 0700)
			if os.RemoveAll(path) != nil {
				ok = false
			}
		}
		return ok
	}
	for _, path := range paths {
		if e = os.Mkdir(path, 0700); e != nil {
			report := NewUnstartedReport(r.Binding, "real", "failed", "scratch_prepare_failed", h)
			report.CleanupConfirmed = cleanup()
			return report
		}
		created = append(created, path)
	}
	if _, e = gotestprofile.MaterializeFixture(paths[0], r.Files); e != nil {
		report := NewUnstartedReport(r.Binding, "real", "failed", "materialize_failed", h)
		report.CleanupConfirmed = cleanup()
		return report
	}
	for _, name := range []string{"gocache", "gopath", "go-tmp", "home", "tmp"} {
		if e = os.Mkdir(filepath.Join(paths[1], name), 0700); e != nil {
			report := NewUnstartedReport(r.Binding, "real", "failed", "scratch_prepare_failed", h)
			report.CleanupConfirmed = cleanup()
			return report
		}
	}
	run := runner.RunFixedGo(ctx, cfg.Bubblewrap, paths[0], cfg.ToolchainRoot, paths[2], paths[1], h.Cgroup, r.Binding.Profile)
	report := NewUnstartedReport(r.Binding, "real", "failed", "process_start_failed", h)
	report.LauncherStarted = run.LaunchObserved
	report.LauncherExitObserved = run.ExitObserved
	if run.Facts.ExitCodeKnown {
		code := run.Facts.ExitCode
		report.LauncherExitCode = &code
	}
	if !run.Started {
		report.CleanupConfirmed = cleanup() && (!run.LaunchObserved || run.Facts.ProcessTreeReaped)
		report.LogsTruncated = run.Facts.StdoutTruncated || run.Facts.StderrTruncated
		if run.Facts.SetupFailure != "" {
			report.Reason = run.Facts.SetupFailure
		}
		report.Stdout = Log{Data: run.Facts.Stdout, SHA256: SHA256(run.Facts.Stdout)}
		report.Stderr = Log{Data: run.Facts.Stderr, SHA256: SHA256(run.Facts.Stderr)}
		report.TimedOut = run.Facts.TimedOut
		report.Cancelled = run.Facts.Cancelled
		report.ResourceLimit = run.Facts.ResourceLimit
		if report.TimedOut {
			report.Outcome = gotestprofile.OutcomeTimedOut
			report.Reason = "wall_clock_timeout"
		} else if report.Cancelled {
			report.Outcome = gotestprofile.OutcomeCancelled
			report.State = "cancelled"
			report.Reason = "cancelled_before_candidate_start"
		} else if report.ResourceLimit != "" {
			report.Outcome = gotestprofile.OutcomeResourceLimit
			report.Reason = "launch_resource_limit"
		}
		return report
	}
	report.ProcessStarted = true
	report.ExitObserved = run.ExitObserved
	if run.Facts.ExitCodeKnown {
		n := run.Facts.ExitCode
		report.ExitCode = &n
	}
	report.TimedOut = run.Facts.TimedOut
	report.Cancelled = run.Facts.Cancelled
	report.ResourceLimit = run.Facts.ResourceLimit
	report.LogsTruncated = run.Facts.StdoutTruncated || run.Facts.StderrTruncated
	removed := cleanup()
	report.CleanupConfirmed = run.Facts.ProcessTreeReaped && removed
	run.Facts.ProcessTreeReaped = report.CleanupConfirmed
	result := gotestprofile.ClassifyResult(run.Facts)
	report.TestResult = &result
	report.Outcome = result.Outcome
	report.Reason = result.ReasonCode
	report.Stdout = Log{Data: run.Facts.Stdout, SHA256: SHA256(run.Facts.Stdout)}
	report.Stderr = Log{Data: run.Facts.Stderr, SHA256: SHA256(run.Facts.Stderr)}
	report.State = "exited"
	if report.Cancelled {
		report.State = "cancelled"
	} else if result.Outcome == "infrastructure_failed" {
		report.State = "failed"
	}
	report.QualificationEligible = result.Outcome == gotestprofile.OutcomePassed && report.CleanupConfirmed && report.ExitObserved && report.ExitCode != nil && *report.ExitCode == 0
	if !report.CleanupConfirmed {
		report.State = "outcome_unknown"
	}
	if ValidateReport(r.Binding, report) != nil {
		report.QualificationEligible = false
		report.State = "failed"
		report.Outcome = gotestprofile.OutcomeInfrastructureFailed
		report.Reason = "invalid_runner_report"
		run.Facts.SetupFailure = "invalid_runner_report"
		result = gotestprofile.ClassifyResult(run.Facts)
		report.TestResult = &result
	}
	return report
}
func validatePrivateRequest(path string) ([]byte, error) {
	i, e := os.Lstat(path)
	if e != nil || !i.Mode().IsRegular() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm()&0077 != 0 || i.Size() > 24<<20 {
		return nil, ErrInvalid
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(i, opened) || !opened.Mode().IsRegular() || opened.Mode().Perm()&0077 != 0 {
		return nil, ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(f, (24<<20)+1))
	if e != nil || int64(len(raw)) != i.Size() {
		return nil, ErrInvalid
	}
	return raw, nil
}
func managedRequestPath(id string) string {
	return fmt.Sprintf("/run/polis-self-dev-go-job/%s/request.json", id)
}
