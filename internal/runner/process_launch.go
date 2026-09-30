// pattern: Functional Core
package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
)

const (
	ProcessLaunchSpecSchema        = "polis-process-launch-spec@1"
	ControlledEnvironmentPolicy    = "controlled-runtime-allowlist@1"
	InheritedEnvironmentPolicy     = "ambient-environment-denied@1"
	LaunchPhaseValidate            = "validate"
	LaunchPhaseWorkingDirectory    = "working_directory"
	LaunchPhaseStdioPipeSetup      = "stdio_pipe_setup"
	LaunchPhaseProcessCreate       = "process_create"
	LaunchPhaseRuntimePreparation  = "runtime_preparation"
	LaunchPhaseProtocolClientSetup = "protocol_client_setup"
	LaunchPhaseInitialize          = "initialize"
	LaunchReasonInvalidArgument    = "invalid_argument"
	LaunchReasonProcessNotCreated  = "process_not_created"
	LaunchReasonExecutableMissing  = "executable_missing"
	LaunchReasonHelperMissing      = "helper_missing"
	LaunchReasonAccessDenied       = "access_denied"
	LaunchReasonCWDMissing         = "cwd_missing"
	LaunchReasonPipeSetupFailed    = "pipe_setup_failed"
	LaunchReasonChildExitedEarly   = "child_exited_before_initialize"
	LaunchReasonRuntimeCrashed     = "runtime_crashed"
	LaunchReasonEnvironmentInvalid = "environment_construction_failed"
)

// LaunchFailure is the safe, structured boundary for process-launch errors.
// Cause is intentionally excluded from JSON because it can contain paths or
// platform-specific text that is not safe evidence.
type LaunchFailure struct {
	Phase          string  `json:"phase"`
	ReasonCode     string  `json:"reason_code"`
	OSErrorCode    *uint32 `json:"os_error_code,omitempty"`
	ProcessCreated bool    `json:"process_created"`
	PIDPresent     bool    `json:"pid_present"`
	SafeMessage    string  `json:"safe_message"`
	Cause          error   `json:"-"`
}

func (e *LaunchFailure) Error() string {
	if e == nil {
		return "provider launch failed"
	}
	return fmt.Sprintf("provider launch failed: phase=%s reason_code=%s safe_message=%s", e.Phase, e.ReasonCode, e.SafeMessage)
}

func (e *LaunchFailure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type ProcessLaunchSpec struct {
	SchemaVersion        string   `json:"schema_version"`
	Executable           string   `json:"executable"`
	Argv                 []string `json:"argv"`
	Environment          []string `json:"-"`
	WorkingDirectory     string   `json:"working_directory"`
	StdinMode            string   `json:"stdin_mode"`
	StdoutMode           string   `json:"stdout_mode"`
	StderrMode           string   `json:"stderr_mode"`
	PipeSetupMode        string   `json:"pipe_setup_mode"`
	ProcessCreationFlags []string `json:"process_creation_flags"`
	ProcessGroupPolicy   string   `json:"process_group_policy"`
	JobObjectPolicy      string   `json:"job_object_policy"`
	EnvironmentPolicy    string   `json:"environment_policy"`
	InheritedEnvironment string   `json:"inherited_environment_policy"`
	RuntimeRoot          string   `json:"runtime_root"`
	RuntimeHome          string   `json:"runtime_home"`
	TempDirectory        string   `json:"temp_directory"`
	EvidenceRoot         string   `json:"evidence_root"`
	RuntimeRootRule      string   `json:"runtime_root_rule"`
	RuntimeHomeRule      string   `json:"runtime_home_rule"`
	TempDirectoryRule    string   `json:"temp_directory_rule"`
	EvidenceRootRule     string   `json:"evidence_root_rule"`
	Launcher             string   `json:"launcher"`
	PreLaunchLifecycle   []string `json:"pre_launch_lifecycle"`
}

type ProcessLaunchDirectories struct {
	RuntimeRoot   string
	RuntimeHome   string
	TempDirectory string
	EvidenceRoot  string
}

type SafeEnvironmentEntry struct {
	Name      string `json:"name"`
	ValueHash string `json:"value_sha256"`
}

type ProcessLaunchEvidence struct {
	SchemaVersion        string                 `json:"schema_version"`
	Executable           string                 `json:"executable"`
	Argv                 []string               `json:"argv"`
	WorkingDirectory     string                 `json:"working_directory"`
	StdinMode            string                 `json:"stdin_mode"`
	StdoutMode           string                 `json:"stdout_mode"`
	StderrMode           string                 `json:"stderr_mode"`
	PipeSetupMode        string                 `json:"pipe_setup_mode"`
	ProcessCreationFlags []string               `json:"process_creation_flags"`
	ProcessGroupPolicy   string                 `json:"process_group_policy"`
	JobObjectPolicy      string                 `json:"job_object_policy"`
	EnvironmentPolicy    string                 `json:"environment_policy"`
	InheritedEnvironment string                 `json:"inherited_environment_policy"`
	Environment          []SafeEnvironmentEntry `json:"environment"`
	RuntimeRoot          string                 `json:"runtime_root"`
	RuntimeHome          string                 `json:"runtime_home"`
	TempDirectory        string                 `json:"temp_directory"`
	EvidenceRoot         string                 `json:"evidence_root"`
	RuntimeRootRule      string                 `json:"runtime_root_rule"`
	RuntimeHomeRule      string                 `json:"runtime_home_rule"`
	TempDirectoryRule    string                 `json:"temp_directory_rule"`
	EvidenceRootRule     string                 `json:"evidence_root_rule"`
	Launcher             string                 `json:"launcher"`
	PreLaunchLifecycle   []string               `json:"pre_launch_lifecycle"`
}

func NewProcessLaunchSpec(argv, environment []string, workingDirectory string, directories ProcessLaunchDirectories) (ProcessLaunchSpec, error) {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return ProcessLaunchSpec{}, &LaunchFailure{Phase: LaunchPhaseValidate, ReasonCode: LaunchReasonInvalidArgument, SafeMessage: "executable argument is required"}
	}
	flags := []string{"windows_hide_window"}
	groupPolicy := "windows_process_handle"
	jobObjectPolicy := "none"
	preLaunchLifecycle := []string(nil)
	if runtime.GOOS != "windows" {
		flags = []string{"setpgid", "pdeathsig_sigkill"}
		groupPolicy = "process_group_pdeathsig"
	} else {
		flags = []string{"windows_hide_window", "create_suspended", "extended_startupinfo_present", "unicode_environment", "no_window", "explicit_handle_list", "job_list_attribute"}
		groupPolicy = "windows_named_job_object"
		jobObjectPolicy = "named;kill_tree_on_close;active_processes<=64;no_breakaway"
		preLaunchLifecycle = []string{"create_named_kill_on_close_job", "create_stdio_pipes", "create_suspended_process_in_job", "resume_main_thread"}
	}
	return ProcessLaunchSpec{
		SchemaVersion:        ProcessLaunchSpecSchema,
		Executable:           argv[0],
		Argv:                 append([]string(nil), argv...),
		Environment:          append([]string{}, environment...),
		WorkingDirectory:     workingDirectory,
		StdinMode:            "redirected_pipe_write",
		StdoutMode:           "redirected_pipe_read",
		StderrMode:           "redirected_pipe_read_bounded",
		PipeSetupMode:        "stdin_stdout_stderr_separate_pipes",
		ProcessCreationFlags: flags,
		ProcessGroupPolicy:   groupPolicy,
		JobObjectPolicy:      jobObjectPolicy,
		EnvironmentPolicy:    ControlledEnvironmentPolicy,
		InheritedEnvironment: InheritedEnvironmentPolicy,
		RuntimeRoot:          directories.RuntimeRoot,
		RuntimeHome:          directories.RuntimeHome,
		TempDirectory:        directories.TempDirectory,
		EvidenceRoot:         directories.EvidenceRoot,
		RuntimeRootRule:      "dedicated runtime root per qualification",
		RuntimeHomeRule:      "runtime_root/session_id/home",
		TempDirectoryRule:    "runtime_home/tmp",
		EvidenceRootRule:     "dedicated evidence root per qualification/session",
		Launcher:             "unspecified",
		PreLaunchLifecycle:   preLaunchLifecycle,
	}, nil
}

func (s ProcessLaunchSpec) SafeEvidence() ProcessLaunchEvidence {
	return ProcessLaunchEvidence{
		SchemaVersion:        s.SchemaVersion,
		Executable:           filepath.Base(s.Executable),
		Argv:                 safeArgv(s.Argv),
		WorkingDirectory:     safePath(s.WorkingDirectory),
		StdinMode:            s.StdinMode,
		StdoutMode:           s.StdoutMode,
		StderrMode:           s.StderrMode,
		PipeSetupMode:        s.PipeSetupMode,
		ProcessCreationFlags: append([]string(nil), s.ProcessCreationFlags...),
		ProcessGroupPolicy:   s.ProcessGroupPolicy,
		JobObjectPolicy:      s.JobObjectPolicy,
		EnvironmentPolicy:    s.EnvironmentPolicy,
		InheritedEnvironment: s.InheritedEnvironment,
		Environment:          safeEnvironment(s.Environment),
		RuntimeRoot:          safePath(s.RuntimeRoot),
		RuntimeHome:          safePath(s.RuntimeHome),
		TempDirectory:        safePath(s.TempDirectory),
		EvidenceRoot:         safePath(s.EvidenceRoot),
		RuntimeRootRule:      s.RuntimeRootRule,
		RuntimeHomeRule:      s.RuntimeHomeRule,
		TempDirectoryRule:    s.TempDirectoryRule,
		EvidenceRootRule:     s.EvidenceRootRule,
		Launcher:             s.Launcher,
		PreLaunchLifecycle:   append([]string(nil), s.PreLaunchLifecycle...),
	}
}

type ProcessLaunchDiff struct {
	SchemaVersion string             `json:"schema_version"`
	Changed       []ProcessFieldDiff `json:"changed"`
	Equal         bool               `json:"equal"`
}

type ProcessFieldDiff struct {
	Field string `json:"field"`
	Left  any    `json:"left"`
	Right any    `json:"right"`
}

func DiffProcessLaunchSpecs(left, right ProcessLaunchSpec) ProcessLaunchDiff {
	leftEvidence := left.SafeEvidence()
	rightEvidence := right.SafeEvidence()
	changed := make([]ProcessFieldDiff, 0)
	appendDiff := func(field string, l, r any) {
		lRaw, _ := json.Marshal(l)
		rRaw, _ := json.Marshal(r)
		if string(lRaw) != string(rRaw) {
			changed = append(changed, ProcessFieldDiff{Field: field, Left: l, Right: r})
		}
	}
	appendDiff("executable", leftEvidence.Executable, rightEvidence.Executable)
	appendDiff("argv", leftEvidence.Argv, rightEvidence.Argv)
	appendDiff("working_directory", leftEvidence.WorkingDirectory, rightEvidence.WorkingDirectory)
	appendDiff("environment", leftEvidence.Environment, rightEvidence.Environment)
	appendDiff("environment_policy", leftEvidence.EnvironmentPolicy, rightEvidence.EnvironmentPolicy)
	appendDiff("inherited_environment_policy", leftEvidence.InheritedEnvironment, rightEvidence.InheritedEnvironment)
	appendDiff("stdio", []string{leftEvidence.StdinMode, leftEvidence.StdoutMode, leftEvidence.StderrMode, leftEvidence.PipeSetupMode}, []string{rightEvidence.StdinMode, rightEvidence.StdoutMode, rightEvidence.StderrMode, rightEvidence.PipeSetupMode})
	appendDiff("process_creation_flags", leftEvidence.ProcessCreationFlags, rightEvidence.ProcessCreationFlags)
	appendDiff("process_group_policy", leftEvidence.ProcessGroupPolicy, rightEvidence.ProcessGroupPolicy)
	appendDiff("job_object_policy", leftEvidence.JobObjectPolicy, rightEvidence.JobObjectPolicy)
	appendDiff("runtime_root", leftEvidence.RuntimeRoot, rightEvidence.RuntimeRoot)
	appendDiff("runtime_home", leftEvidence.RuntimeHome, rightEvidence.RuntimeHome)
	appendDiff("temp_directory", leftEvidence.TempDirectory, rightEvidence.TempDirectory)
	appendDiff("evidence_root", leftEvidence.EvidenceRoot, rightEvidence.EvidenceRoot)
	appendDiff("runtime_directory_rules", []string{leftEvidence.RuntimeRootRule, leftEvidence.RuntimeHomeRule, leftEvidence.TempDirectoryRule, leftEvidence.EvidenceRootRule}, []string{rightEvidence.RuntimeRootRule, rightEvidence.RuntimeHomeRule, rightEvidence.TempDirectoryRule, rightEvidence.EvidenceRootRule})
	appendDiff("launcher", leftEvidence.Launcher, rightEvidence.Launcher)
	appendDiff("pre_launch_lifecycle", leftEvidence.PreLaunchLifecycle, rightEvidence.PreLaunchLifecycle)
	return ProcessLaunchDiff{SchemaVersion: ProcessLaunchSpecSchema, Changed: changed, Equal: len(changed) == 0}
}

func BuildControlledEnvironment(parent []string, home, tempDirectory string) []string {
	if tempDirectory == "" {
		tempDirectory = filepath.Join(home, "tmp")
	}
	values := []string{
		"HOME=" + home,
		"CODEX_HOME=" + home,
		"USERPROFILE=" + home,
		"TEMP=" + tempDirectory,
		"TMP=" + tempDirectory,
	}
	if runtime.GOOS == "windows" {
		systemRoot := environmentValue(parent, "SystemRoot")
		if systemRoot == "" {
			systemRoot = environmentValue(parent, "WINDIR")
		}
		values = append(values, "SystemRoot="+systemRoot, "WINDIR="+systemRoot, "PATH="+filepath.Join(systemRoot, "System32")+";"+systemRoot)
	} else {
		values = append(values, "PATH=/usr/bin:/bin", "LANG=C.UTF-8")
	}
	return values
}

func NewLaunchFailure(phase, reason string, cause error, processCreated bool, pid int) *LaunchFailure {
	code := osErrorCode(cause)
	return &LaunchFailure{Phase: phase, ReasonCode: reason, OSErrorCode: code, ProcessCreated: processCreated, PIDPresent: pid > 0, SafeMessage: safeErrorMessage(cause), Cause: cause}
}

func ClassifyProcessCreateFailure(err error, workingDirectory string) *LaunchFailure {
	if workingDirectory != "" && errors.Is(err, os.ErrNotExist) {
		return NewLaunchFailure(LaunchPhaseWorkingDirectory, LaunchReasonCWDMissing, err, false, 0)
	}
	code := osErrorCode(err)
	switch {
	case errors.Is(err, os.ErrNotExist) || errorCodeIs(code, 2, 3, 193):
		return NewLaunchFailure(LaunchPhaseProcessCreate, LaunchReasonExecutableMissing, err, false, 0)
	case errors.Is(err, os.ErrPermission) || errorCodeIs(code, 5, 13):
		return NewLaunchFailure(LaunchPhaseProcessCreate, LaunchReasonAccessDenied, err, false, 0)
	case errorCodeIs(code, 22, 87):
		return NewLaunchFailure(LaunchPhaseProcessCreate, LaunchReasonInvalidArgument, err, false, 0)
	default:
		return NewLaunchFailure(LaunchPhaseProcessCreate, LaunchReasonProcessNotCreated, err, false, 0)
	}
}

func ClassifyPipeSetupFailure(err error) *LaunchFailure {
	return NewLaunchFailure(LaunchPhaseStdioPipeSetup, LaunchReasonPipeSetupFailed, err, false, 0)
}

func ClassifyPostStartFailure(err error, processCreated bool, pid int, childExited bool) *LaunchFailure {
	reason := LaunchReasonRuntimeCrashed
	if childExited {
		reason = LaunchReasonChildExitedEarly
	}
	return NewLaunchFailure(LaunchPhaseProtocolClientSetup, reason, err, processCreated, pid)
}

func ClassifyRuntimePreparationFailure(err error) *LaunchFailure {
	reason := LaunchReasonEnvironmentInvalid
	if errors.Is(err, os.ErrNotExist) || strings.Contains(strings.ToLower(err.Error()), "code-mode host") || strings.Contains(strings.ToLower(err.Error()), "helper") {
		reason = LaunchReasonHelperMissing
	}
	return NewLaunchFailure(LaunchPhaseRuntimePreparation, reason, err, false, 0)
}

func safeEnvironment(environment []string) []SafeEnvironmentEntry {
	entries := make([]SafeEnvironmentEntry, 0, len(environment))
	for _, item := range environment {
		name, value, ok := strings.Cut(item, "=")
		if !ok || name == "" || sensitiveEnvironmentName(name) {
			continue
		}
		entries = append(entries, SafeEnvironmentEntry{Name: name, ValueHash: digestString(normalizeEnvironmentValue(name, value))})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

func normalizeEnvironmentValue(name, value string) string {
	switch strings.ToUpper(name) {
	case "HOME", "CODEX_HOME", "USERPROFILE":
		return "<controlled-runtime-home>"
	case "TEMP", "TMP":
		return "<controlled-runtime-temp>"
	case "SYSTEMROOT", "WINDIR":
		return "<windows-system-root>"
	case "PATH":
		return "<controlled-system-path>"
	default:
		return value
	}
}

func safeArgv(argv []string) []string {
	return append([]string(nil), argv...)
}

func safePath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func sensitiveEnvironmentName(name string) bool {
	upper := strings.ToUpper(name)
	return strings.Contains(upper, "KEY") || strings.Contains(upper, "TOKEN") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "PASSWORD") || strings.HasPrefix(upper, "AUTH")
}

func environmentValue(environment []string, wanted string) string {
	for _, item := range environment {
		name, value, ok := strings.Cut(item, "=")
		if ok && strings.EqualFold(name, wanted) {
			return value
		}
	}
	return ""
}

func digestString(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func osErrorCode(err error) *uint32 {
	if err == nil {
		return nil
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return nil
	}
	value := uint32(errno)
	return &value
}

func errorCodeIs(code *uint32, values ...uint32) bool {
	if code == nil {
		return false
	}
	for _, value := range values {
		if *code == value {
			return true
		}
	}
	return false
}

func safeErrorMessage(err error) string {
	if err == nil {
		return "unknown launch error"
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Err != nil {
		return strings.ToLower(pathErr.Err.Error())
	}
	text := strings.TrimSpace(err.Error())
	if index := strings.LastIndex(text, ": "); index >= 0 {
		text = text[index+2:]
	}
	if strings.ContainsAny(text, `\\/`) || strings.Contains(text, ":") {
		return "launch operation failed"
	}
	if len(text) > 240 {
		text = text[:240]
	}
	return text
}

func launchFailureJSON(failure *LaunchFailure) []byte {
	if failure == nil {
		return nil
	}
	raw, err := json.Marshal(failure)
	if err != nil {
		return nil
	}
	return raw
}
