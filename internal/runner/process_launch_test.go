// pattern: Functional Core
package runner

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestStartWithWorkingDirClassifiesMissingExecutable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-provider.exe")
	_, err := StartWithWorkingDir("missing-executable", []string{missing, "app-server", "--stdio"}, nil, "")
	if err == nil {
		t.Fatal("missing executable unexpectedly started")
	}
	var failure *LaunchFailure
	if !errors.As(err, &failure) {
		t.Fatalf("error type=%T, want *LaunchFailure: %v", err, err)
	}
	if failure.ReasonCode != LaunchReasonExecutableMissing || failure.Phase != LaunchPhaseProcessCreate || failure.ProcessCreated || failure.PIDPresent {
		t.Fatalf("unexpected launch failure: %+v", failure)
	}
	if failure.OSErrorCode == nil || *failure.OSErrorCode == 0 || failure.SafeMessage == "" {
		t.Fatalf("missing safe OS diagnostics: %+v", failure)
	}
}

func TestStartWithWorkingDirClassifiesMissingWorkingDirectory(t *testing.T) {
	missingDirectory := filepath.Join(t.TempDir(), "missing-cwd")
	_, err := StartWithWorkingDir("missing-cwd", []string{"/bin/echo", "unused"}, nil, missingDirectory)
	var failure *LaunchFailure
	if !errors.As(err, &failure) || failure.ReasonCode != LaunchReasonCWDMissing || failure.Phase != LaunchPhaseWorkingDirectory || failure.ProcessCreated || failure.PIDPresent {
		t.Fatalf("unexpected cwd failure: %v", err)
	}
}

func TestLaunchFailureClassificationsCoverAccessDeniedAndPipeSetup(t *testing.T) {
	access := ClassifyProcessCreateFailure(syscall.EACCES, "")
	if access.ReasonCode != LaunchReasonAccessDenied || access.ProcessCreated || access.PIDPresent {
		t.Fatalf("unexpected access-denied classification: %+v", access)
	}
	pipe := ClassifyPipeSetupFailure(errors.New("pipe allocation failed"))
	if pipe.ReasonCode != LaunchReasonPipeSetupFailed || pipe.Phase != LaunchPhaseStdioPipeSetup {
		t.Fatalf("unexpected pipe classification: %+v", pipe)
	}
	child := ClassifyPostStartFailure(errors.New("child exited"), true, 42, true)
	if child.ReasonCode != LaunchReasonChildExitedEarly || !child.ProcessCreated || !child.PIDPresent {
		t.Fatalf("unexpected early-exit classification: %+v", child)
	}
}

func TestRuntimePreparationClassifiesMissingHelperWithoutRawPath(t *testing.T) {
	failure := ClassifyRuntimePreparationFailure(errors.New("mandatory native code-mode host missing: C:\\secret\\helper.exe"))
	if failure.ReasonCode != LaunchReasonHelperMissing || strings.Contains(failure.SafeMessage, "secret") {
		t.Fatalf("unexpected helper classification: %+v", failure)
	}
}

func TestControlledEnvironmentDoesNotInheritRuntimeOverrides(t *testing.T) {
	env := BuildControlledEnvironment([]string{
		"PATH=C:\\ambient\\codex",
		"CODEX_VERSION=0.155.0-alpha.9.2",
		"CODEX_HOME=C:\\ambient\\home",
		"USERPROFILE=C:\\Users\\employee",
		"TEMP=C:\\ambient\\temp",
		"TMP=C:\\ambient\\tmp",
		"OPENAI_API_KEY=secret",
		"SystemRoot=C:\\Windows",
	}, `C:\runtime\home`, `C:\runtime\home\tmp`)
	joined := strings.Join(env, "\n")
	for _, forbidden := range []string{"0.155.0-alpha.9.2", "C:\\ambient\\codex", "C:\\Users\\employee", "secret"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("ambient or secret value leaked into controlled environment: %q", forbidden)
		}
	}
	for _, required := range []string{"HOME=C:\\runtime\\home", "CODEX_HOME=C:\\runtime\\home", "TEMP=C:\\runtime\\home\\tmp", "TMP=C:\\runtime\\home\\tmp"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("controlled environment missing %q: %s", required, joined)
		}
	}
}

func TestProcessLaunchSpecSafeEvidenceOmitsEnvironmentValues(t *testing.T) {
	spec := ProcessLaunchSpec{
		Argv:             []string{"C:\\runtime\\codex.exe", "app-server", "--stdio"},
		Environment:      []string{"PATH=C:\\Windows\\System32", "OPENAI_API_KEY=secret"},
		WorkingDirectory: `C:\runtime`,
		RuntimeRoot:      `C:\runtime`,
		EvidenceRoot:     `C:\evidence`,
	}
	raw, err := json.Marshal(spec.SafeEvidence())
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"secret", "C:\\Windows\\System32"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("launch evidence leaked environment value %q: %s", forbidden, text)
		}
	}
}

func TestProcessLaunchDiffIncludesLauncherAndDirectoryLifecycle(t *testing.T) {
	left, err := NewProcessLaunchSpec([]string{"C:\\runtime\\codex.exe", "app-server", "--stdio"}, []string{"PATH=C:\\Windows"}, "", ProcessLaunchDirectories{RuntimeRoot: "C:\\b11", RuntimeHome: "C:\\b11\\session\\home", TempDirectory: "C:\\b11\\session\\home\\tmp", EvidenceRoot: "C:\\b11\\evidence"})
	if err != nil {
		t.Fatal(err)
	}
	right := left
	right.RuntimeRoot = "C:\\b12"
	right.RuntimeHome = "C:\\b12\\session\\home"
	right.EvidenceRoot = "C:\\b12\\evidence"
	right.Launcher = "cmd/polis serve -> RealProviderWorkerAdapter -> CodexRuntime.Start"
	right.PreLaunchLifecycle = []string{"readiness_rebind", "canonical_process_launch", "protocol_client"}
	diff := DiffProcessLaunchSpecs(left, right)
	if diff.Equal || len(diff.Changed) == 0 {
		t.Fatalf("launch lifecycle diff was not recorded: %+v", diff)
	}
	changed := map[string]bool{}
	for _, field := range diff.Changed {
		changed[field.Field] = true
	}
	if !changed["runtime_root"] || !changed["launcher"] || !changed["pre_launch_lifecycle"] {
		t.Fatalf("launch diff omitted required fields: %+v", diff.Changed)
	}
}
