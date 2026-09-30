// pattern: Functional Core
package control

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/kernel"
	"polis/internal/runner"
)

func TestNormalizeStartProjectJobRequestBoundsBatchScriptExecution(t *testing.T) {
	valid := StartProjectJobRequest{
		TaskID: "task-1", SessionID: "session-1", EnvironmentRevisionID: "environment-1",
		Kind: "batch", ScriptPath: "scripts/build.mjs", Args: []string{"--check", "fixture"}, RequestID: "job-start-1",
	}
	got, err := normalizeStartProjectJobRequest(valid)
	if err != nil {
		t.Fatal(err)
	}
	if got.ScriptPath != valid.ScriptPath || len(got.Args) != len(valid.Args) || got.Args[0] != valid.Args[0] {
		t.Fatalf("normalized JobRun request=%+v", got)
	}
	got.Args[0] = "mutated"
	if valid.Args[0] != "--check" {
		t.Fatal("normalized request aliases caller-owned argv")
	}
	for name, mutate := range map[string]func(*StartProjectJobRequest){
		"service kind requires readiness qualification": func(value *StartProjectJobRequest) { value.Kind = "service" },
		"controlled input requires its contract":        func(value *StartProjectJobRequest) { value.Kind = "controlled_input" },
		"path traversal":                                func(value *StartProjectJobRequest) { value.ScriptPath = "../build.js" },
		"non-script extension":                          func(value *StartProjectJobRequest) { value.ScriptPath = "scripts/build.cmd" },
		"NUL argument":                                  func(value *StartProjectJobRequest) { value.Args = []string{"bad\x00arg"} },
		"unbounded argument":                            func(value *StartProjectJobRequest) { value.Args = []string{strings.Repeat("x", 9000)} },
		"invalid request id":                            func(value *StartProjectJobRequest) { value.RequestID = "not valid" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			candidate.Args = append([]string(nil), valid.Args...)
			mutate(&candidate)
			if _, err := normalizeStartProjectJobRequest(candidate); err == nil {
				t.Fatal("unsafe or unsupported JobRun request was accepted")
			}
		})
	}
	if _, err := normalizeStartProjectJobRequest(StartProjectJobRequest{}); err == nil || err != core.Malformed {
		t.Fatalf("malformed request error=%v, want %s", err, core.Malformed)
	}
}

func TestNormalizeStartProjectServiceJobPinsOnlyServiceID(t *testing.T) {
	valid := StartProjectJobRequest{
		TaskID: "task-1", SessionID: "session-1", EnvironmentRevisionID: "environment-1",
		Kind: "service", ServiceID: "web", RequestID: "service-start-1",
	}
	got, err := normalizeStartProjectJobRequest(valid)
	if err != nil || got.ServiceID != "web" || got.ScriptPath != "" || len(got.Args) != 0 {
		t.Fatalf("normalized service JobRun request=%+v error=%v", got, err)
	}
	for name, mutate := range map[string]func(*StartProjectJobRequest){
		"missing service id":    func(value *StartProjectJobRequest) { value.ServiceID = "" },
		"caller script path":    func(value *StartProjectJobRequest) { value.ScriptPath = "server.js" },
		"caller arguments":      func(value *StartProjectJobRequest) { value.Args = []string{"--port", "3000"} },
		"batch with service id": func(value *StartProjectJobRequest) { value.Kind = "batch" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			candidate.Args = append([]string(nil), valid.Args...)
			mutate(&candidate)
			if _, err := normalizeStartProjectJobRequest(candidate); err == nil {
				t.Fatal("caller-controlled service execution was accepted")
			}
		})
	}
}

func TestBuildProjectJobLogManifestPreservesBoundedRawStreams(t *testing.T) {
	stdout := []byte{0xff, 'o', 'k'}
	stderr := []byte("warn")
	manifest, err := buildProjectJobLogManifest("company-1", "job-1", stdout, stderr, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest) == 0 || len(manifest) > maxProjectJobLogManifestBytes {
		t.Fatalf("manifest byte length=%d", len(manifest))
	}
	decoded, err := parseProjectJobLogManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.CompanyID != "company-1" || decoded.JobID != "job-1" || string(decoded.Stdout) != string(stdout) || string(decoded.Stderr) != string(stderr) || !decoded.LogsTruncated {
		t.Fatalf("manifest roundtrip=%+v", decoded)
	}
}

func TestProjectJobCgroupCleanupFailureOverridesCancellation(t *testing.T) {
	waitErr := errors.Join(context.DeadlineExceeded, environment.ErrLinuxNodeCgroupCleanupUnconfirmed)
	state, reason, exitCode := classifyProjectJobCompletion(waitErr, context.DeadlineExceeded, 0)
	if state != environment.JobOutcomeUnknown || reason != "project_job_cgroup_cleanup_unconfirmed" || exitCode != nil {
		t.Fatalf("cgroup cleanup failure classification=(%q,%q,%v)", state, reason, exitCode)
	}
}

func TestServiceEndpointFailurePreservesUnknownStopOutcome(t *testing.T) {
	state, reason, exitCode := classifyProjectServiceJobCompletion(
		errors.Join(context.Canceled, runner.ErrAppContainerProcessStopUnconfirmed),
		context.Canceled, 0, false, "", "service_endpoint_owner_unverified",
	)
	if state != environment.JobOutcomeUnknown || reason != "project_job_outcome_unknown" || exitCode != nil {
		t.Fatalf("service stop classification=(%q,%q,%v)", state, reason, exitCode)
	}
	state, reason, exitCode = classifyProjectServiceJobCompletion(context.Canceled, context.Canceled, 0, true, "service_endpoint_stop_unconfirmed", "service_endpoint_owner_unverified")
	if state != environment.JobOutcomeUnknown || reason != "service_endpoint_stop_unconfirmed" || exitCode != nil {
		t.Fatalf("explicit unconfirmed service stop classification=(%q,%q,%v)", state, reason, exitCode)
	}
}

func TestProjectServiceRevocationRequestIsStableAcrossRetries(t *testing.T) {
	service := &activeProjectService{
		definition:           environment.ProjectServiceDefinition{Probe: environment.ServiceProbeSpec{BindAddress: "127.0.0.1", Port: 43127}},
		sourceRevisionSHA256: strings.Repeat("a", 64), healthcheckSHA256: strings.Repeat("b", 64),
		endpointRecorded: true, endpointGeneration: 1,
	}
	active := &activeProjectJob{jobID: "job-1", service: service}
	first, ok := pendingProjectServiceRevocationInput(active)
	if !ok {
		t.Fatal("recorded endpoint did not produce a revoke request")
	}
	time.Sleep(time.Millisecond)
	second, ok := pendingProjectServiceRevocationInput(active)
	if !ok || first != second {
		t.Fatalf("revoke retry changed immutable request payload: first=%+v second=%+v present=%v", first, second, ok)
	}
	if _, ok = pendingProjectServiceRevocationInput(&activeProjectJob{jobID: "job-2", service: &activeProjectService{}}); ok {
		t.Fatal("unrecorded endpoint produced a revoke request")
	}
}

func TestProjectJobTerminalRetryRetainsTheOriginalIdempotentEvent(t *testing.T) {
	active := &activeProjectJob{companyID: "company-1", jobID: "job-1"}
	input := kernel.JobRunEventInput{JobID: "job-1", State: string(environment.JobExited), ReasonCode: "none", ExitCode: intPointer(0)}
	first := active.prepareTerminalEvent(input)
	second := active.prepareTerminalEvent(kernel.JobRunEventInput{JobID: "job-1", State: string(environment.JobCancelled), ReasonCode: "replacement"})
	pending, ok := active.pendingTerminal()
	if !ok || first != second || second != pending || first.RequestID == "" || first.State != string(environment.JobExited) {
		t.Fatalf("terminal retry did not preserve its first immutable payload: first=%+v second=%+v pending=%+v present=%v", first, second, pending, ok)
	}
}

func intPointer(value int) *int { return &value }
