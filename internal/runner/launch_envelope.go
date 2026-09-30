// pattern: Functional Core
package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	NativeLaunchModeWindowsDirect = "windows_native_direct"
	NativeLaunchModeWSLBwrap      = "wsl_bwrap_isolated"
	NativeLaunchEnvelopeSchema    = "r03a-native-launch-envelope@2"
)

type NativeLaunchEnvelope struct {
	SchemaVersion              string                       `json:"schema_version"`
	LaunchMode                 string                       `json:"launch_mode"`
	RuntimeOS                  string                       `json:"runtime_os"`
	ProcessExecutable          string                       `json:"process_executable"`
	BinaryPath                 string                       `json:"binary_path"`
	HelperPath                 string                       `json:"helper_path"`
	BinarySHA256               string                       `json:"binary_sha256"`
	HelperSHA256               string                       `json:"helper_sha256"`
	Argv                       []string                     `json:"argv"`
	WorkingDirectory           string                       `json:"working_directory"`
	Home                       string                       `json:"home"`
	CodexHome                  string                       `json:"codex_home"`
	Stdio                      string                       `json:"stdio"`
	Boundary                   string                       `json:"boundary"`
	EnvironmentPolicy          string                       `json:"environment_policy"`
	InheritedEnvironmentPolicy string                       `json:"inherited_environment_policy"`
	Environment                []SafeEnvironmentEntry       `json:"environment"`
	StdinMode                  string                       `json:"stdin_mode"`
	StdoutMode                 string                       `json:"stdout_mode"`
	StderrMode                 string                       `json:"stderr_mode"`
	PipeSetupMode              string                       `json:"pipe_setup_mode"`
	ProcessCreationFlags       []string                     `json:"process_creation_flags"`
	ProcessGroupPolicy         string                       `json:"process_group_policy"`
	JobObjectPolicy            string                       `json:"job_object_policy"`
	RuntimeRoot                string                       `json:"runtime_root"`
	RuntimeHome                string                       `json:"runtime_home"`
	TempDirectory              string                       `json:"temp_directory"`
	EvidenceDirectory          string                       `json:"evidence_directory"`
	RuntimeRootRule            string                       `json:"runtime_root_rule"`
	RuntimeHomeRule            string                       `json:"runtime_home_rule"`
	TempDirectoryRule          string                       `json:"temp_directory_rule"`
	EvidenceRootRule           string                       `json:"evidence_root_rule"`
	TransportPolicy            NativeTransportPolicyBinding `json:"transport_policy"`
	Fingerprint                string                       `json:"fingerprint"`
}

type NativeTransportPolicyBinding struct {
	Revision               string `json:"revision"`
	InitializeTimeoutMS    int64  `json:"initialize_timeout_ms"`
	StartAcknowledgementMS int64  `json:"start_acknowledgement_ms"`
	FirstOutputDeadlineMS  int64  `json:"first_output_deadline_ms"`
	ReconnectGraceMS       int64  `json:"reconnect_grace_ms"`
	StreamingIdleMS        int64  `json:"streaming_idle_ms"`
	TotalTurnDeadlineMS    int64  `json:"total_turn_deadline_ms"`
	ReconciliationMS       int64  `json:"reconciliation_ms"`
}

func NativeLaunchMode(binary string) string {
	if runtime.GOOS == "windows" {
		return NativeLaunchModeWindowsDirect
	}
	if strings.HasSuffix(strings.ToLower(binary), ".exe") {
		return NativeLaunchModeWSLBwrap + "_windows_interop"
	}
	return NativeLaunchModeWSLBwrap
}

func DescribeNativeLaunch(binary, helper, home string, argv, environment []string, workingDirectory string) (NativeLaunchEnvelope, error) {
	return DescribeNativeLaunchWithDirectories(binary, helper, home, argv, environment, workingDirectory, ProcessLaunchDirectories{RuntimeHome: home, TempDirectory: filepath.Join(home, "tmp")})
}

func DescribeNativeLaunchWithDirectories(binary, helper, home string, argv, environment []string, workingDirectory string, directories ProcessLaunchDirectories) (NativeLaunchEnvelope, error) {
	if binary == "" || helper == "" || len(argv) == 0 {
		return NativeLaunchEnvelope{}, errors.New("native launch envelope inputs are incomplete")
	}
	binarySHA, err := sha256File(binary)
	if err != nil {
		return NativeLaunchEnvelope{}, err
	}
	helperSHA, err := sha256File(helper)
	if err != nil {
		return NativeLaunchEnvelope{}, err
	}
	env := map[string]string{}
	for _, value := range environment {
		name, item, ok := strings.Cut(value, "=")
		if ok && (name == "HOME" || name == "CODEX_HOME") {
			env[name] = item
		}
	}
	if env["HOME"] == "" {
		env["HOME"] = home
	}
	if env["CODEX_HOME"] == "" {
		env["CODEX_HOME"] = home
	}
	envelope := NativeLaunchEnvelope{
		SchemaVersion:     NativeLaunchEnvelopeSchema,
		LaunchMode:        NativeLaunchMode(binary),
		RuntimeOS:         runtime.GOOS,
		ProcessExecutable: filepath.Base(argv[0]),
		BinaryPath:        binary,
		HelperPath:        helper,
		BinarySHA256:      binarySHA,
		HelperSHA256:      helperSHA,
		Argv:              append([]string(nil), argv...),
		WorkingDirectory:  workingDirectory,
		Home:              env["HOME"],
		CodexHome:         env["CODEX_HOME"],
		Stdio:             "stdin_stdout_jsonl;stderr_separate_bounded",
		Boundary:          map[bool]string{true: "Windows native process handle", false: "WSL process group / bubblewrap namespace"}[runtime.GOOS == "windows"],
		EnvironmentPolicy: ControlledEnvironmentPolicy, InheritedEnvironmentPolicy: InheritedEnvironmentPolicy,
		Environment: safeEnvironment(environment), StdinMode: "redirected_pipe_write", StdoutMode: "redirected_pipe_read", StderrMode: "redirected_pipe_read_bounded",
		PipeSetupMode: "stdin_stdout_stderr_separate_pipes", ProcessCreationFlags: processCreationFlags(), ProcessGroupPolicy: processGroupPolicy(), JobObjectPolicy: "none",
		RuntimeRoot: directories.RuntimeRoot, RuntimeHome: directories.RuntimeHome, TempDirectory: directories.TempDirectory, EvidenceDirectory: directories.EvidenceRoot,
		RuntimeRootRule: "dedicated runtime root per qualification", RuntimeHomeRule: "runtime_root/session_id/home", TempDirectoryRule: "runtime_home/tmp", EvidenceRootRule: "dedicated evidence root per qualification/session",
	}
	return recomputeLaunchEnvelopeFingerprint(envelope, binary, helper, home, workingDirectory), nil
}

func BindTransportPolicy(envelope NativeLaunchEnvelope, policy NativeTransportPolicyBinding) NativeLaunchEnvelope {
	envelope.TransportPolicy = policy
	return recomputeLaunchEnvelopeFingerprint(envelope, envelope.BinaryPath, envelope.HelperPath, envelope.Home, envelope.WorkingDirectory)
}

func recomputeLaunchEnvelopeFingerprint(envelope NativeLaunchEnvelope, binary, helper, home, workingDirectory string) NativeLaunchEnvelope {
	identity := struct {
		SchemaVersion              string                       `json:"schema_version"`
		LaunchMode                 string                       `json:"launch_mode"`
		RuntimeOS                  string                       `json:"runtime_os"`
		ProcessExecutable          string                       `json:"process_executable"`
		BinarySHA256               string                       `json:"binary_sha256"`
		HelperSHA256               string                       `json:"helper_sha256"`
		Argv                       []string                     `json:"argv"`
		WorkingDirectory           string                       `json:"working_directory"`
		Home                       string                       `json:"home"`
		CodexHome                  string                       `json:"codex_home"`
		Stdio                      string                       `json:"stdio"`
		Boundary                   string                       `json:"boundary"`
		EnvironmentPolicy          string                       `json:"environment_policy"`
		InheritedEnvironmentPolicy string                       `json:"inherited_environment_policy"`
		Environment                []SafeEnvironmentEntry       `json:"environment"`
		StdinMode                  string                       `json:"stdin_mode"`
		StdoutMode                 string                       `json:"stdout_mode"`
		StderrMode                 string                       `json:"stderr_mode"`
		PipeSetupMode              string                       `json:"pipe_setup_mode"`
		ProcessCreationFlags       []string                     `json:"process_creation_flags"`
		ProcessGroupPolicy         string                       `json:"process_group_policy"`
		JobObjectPolicy            string                       `json:"job_object_policy"`
		RuntimeRootRule            string                       `json:"runtime_root_rule"`
		RuntimeHomeRule            string                       `json:"runtime_home_rule"`
		TempDirectoryRule          string                       `json:"temp_directory_rule"`
		EvidenceRootRule           string                       `json:"evidence_root_rule"`
		TransportPolicy            NativeTransportPolicyBinding `json:"transport_policy"`
	}{
		SchemaVersion: envelope.SchemaVersion, LaunchMode: envelope.LaunchMode, RuntimeOS: envelope.RuntimeOS,
		ProcessExecutable: envelope.ProcessExecutable, BinarySHA256: envelope.BinarySHA256, HelperSHA256: envelope.HelperSHA256,
		Argv:             normalizeLaunchArgs(envelope.Argv, binary, helper, home, workingDirectory),
		WorkingDirectory: "<controlled-runtime-cwd>", Home: "<controlled-runtime-home>", CodexHome: "<controlled-runtime-home>",
		Stdio: envelope.Stdio, Boundary: envelope.Boundary, EnvironmentPolicy: envelope.EnvironmentPolicy, InheritedEnvironmentPolicy: envelope.InheritedEnvironmentPolicy,
		Environment: envelope.Environment, StdinMode: envelope.StdinMode, StdoutMode: envelope.StdoutMode, StderrMode: envelope.StderrMode, PipeSetupMode: envelope.PipeSetupMode,
		ProcessCreationFlags: envelope.ProcessCreationFlags, ProcessGroupPolicy: envelope.ProcessGroupPolicy, JobObjectPolicy: envelope.JobObjectPolicy,
		RuntimeRootRule: envelope.RuntimeRootRule, RuntimeHomeRule: envelope.RuntimeHomeRule, TempDirectoryRule: envelope.TempDirectoryRule, EvidenceRootRule: envelope.EvidenceRootRule,
		TransportPolicy: envelope.TransportPolicy,
	}
	identityRaw, err := json.Marshal(identity)
	if err != nil {
		return envelope
	}
	digest := sha256.Sum256(identityRaw)
	envelope.Fingerprint = hex.EncodeToString(digest[:])
	return envelope
}

func processCreationFlags() []string {
	if runtime.GOOS == "windows" {
		return []string{"windows_hide_window"}
	}
	return []string{"setpgid", "pdeathsig_sigkill"}
}

func processGroupPolicy() string {
	if runtime.GOOS == "windows" {
		return "windows_process_handle"
	}
	return "process_group_pdeathsig"
}

func normalizeLaunchArgs(args []string, binary, helper, home, workingDirectory string) []string {
	normalized := make([]string, len(args))
	for i, arg := range args {
		switch arg {
		case binary:
			normalized[i] = "<codex-binary>"
		case helper:
			normalized[i] = "<code-mode-host>"
		case home:
			normalized[i] = "<controlled-runtime-home>"
		case workingDirectory:
			normalized[i] = "<controlled-runtime-cwd>"
		default:
			normalized[i] = arg
		}
	}
	return normalized
}

func sha256File(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}
