package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"polis/internal/core"
)

type productJobLifecycleFixture struct {
	startedBinding Binding
	start          ProductTaskJobStartCommand
	startCalls     int
	stoppedBinding Binding
	stoppedJobID   string
	stopRequestID  string
	stopCalls      int
}

type productEnvironmentLifecycleFixture struct {
	binding   Binding
	revision  string
	requestID string
	calls     int
}

func (fixture *productEnvironmentLifecycleFixture) EnsureProductTaskEnvironment(_ context.Context, binding Binding, revisionID, requestID string) (EnvironmentPreparationRun, error) {
	fixture.binding = binding
	fixture.revision = revisionID
	fixture.requestID = requestID
	fixture.calls++
	return EnvironmentPreparationRun{CompanyID: binding.CompanyID(), RevisionID: revisionID, RunID: "environment-run-fixture", State: "ready"}, nil
}

func (fixture *productJobLifecycleFixture) StartProductTaskJob(_ context.Context, binding Binding, input ProductTaskJobStartCommand) (JobRunRecord, error) {
	fixture.startedBinding = binding
	fixture.start = input
	fixture.startCalls++
	return JobRunRecord{CompanyID: binding.CompanyID(), TaskID: binding.TaskID(), SessionID: binding.SessionID(), JobID: "job-start-fixture"}, nil
}

func (fixture *productJobLifecycleFixture) StopProductTaskJob(_ context.Context, binding Binding, jobID, requestID string) (JobRunRecord, error) {
	fixture.stoppedBinding = binding
	fixture.stoppedJobID = jobID
	fixture.stopRequestID = requestID
	fixture.stopCalls++
	return JobRunRecord{CompanyID: binding.CompanyID(), TaskID: binding.TaskID(), SessionID: binding.SessionID(), JobID: jobID, State: "cancelled"}, nil
}

func TestProductProjectJobToolsRequireExactOptInAndTrustedLifecycle(t *testing.T) {
	binding := Binding{scope: Scope{company: "company-1"}, task: "task-1", session: "session-1"}
	startArgs := json.RawMessage(`{"environment_revision_id":"environment-1","kind":"batch","service_id":"","script_path":"scripts/test.mjs","args":["--offline"]}`)
	for _, tools := range []EmployeeTools{
		{Kernel: &Kernel{}, Binding: binding, ProductSurface: true, ProjectJobsSurface: true},
		{Kernel: &Kernel{}, Binding: binding, ProductSurface: true, ProjectJobsSurface: true, ReadOnly: true, ProjectJobLifecycle: &productJobLifecycleFixture{}},
		{Kernel: &Kernel{}, Binding: binding, ProductSurface: true, ProjectJobLifecycle: &productJobLifecycleFixture{}},
	} {
		if result, err := tools.call(context.Background(), "jobs_start", "call-key", startArgs); err == nil && result.Error == "" {
			t.Fatalf("jobs_start was enabled without the exact mutable product surface and lifecycle adapter: %+v", result)
		}
	}
}

func TestProductProjectJobToolsUseBoundTaskSessionAndStableRequestIDs(t *testing.T) {
	binding := Binding{scope: Scope{company: "company-1"}, task: "task-1", session: "session-1"}
	lifecycle := &productJobLifecycleFixture{}
	tools := EmployeeTools{Kernel: &Kernel{}, Binding: binding, ProductSurface: true, ProjectJobsSurface: true, ProjectJobLifecycle: lifecycle}
	startArgs := json.RawMessage(`{"environment_revision_id":"environment-1","kind":"batch","service_id":"","script_path":"scripts/test.mjs","args":["--offline"]}`)
	first, firstErr := tools.call(context.Background(), "jobs_start", "same-call", startArgs)
	second, secondErr := tools.call(context.Background(), "jobs_start", "same-call", startArgs)
	if firstErr != nil || secondErr != nil || first.Error != "" || second.Error != "" || lifecycle.startCalls != 2 {
		t.Fatalf("jobs_start results/calls=%+v/%+v/%d", first, second, lifecycle.startCalls)
	}
	if lifecycle.startedBinding.CompanyID() != binding.CompanyID() || lifecycle.startedBinding.TaskID() != binding.TaskID() || lifecycle.startedBinding.SessionID() != binding.SessionID() {
		t.Fatalf("jobs_start lifecycle binding=%+v, want exact trusted binding", lifecycle.startedBinding)
	}
	if lifecycle.start.EnvironmentRevisionID != "environment-1" || lifecycle.start.Kind != "batch" || lifecycle.start.ScriptPath != "scripts/test.mjs" || len(lifecycle.start.Args) != 1 || lifecycle.start.Args[0] != "--offline" {
		t.Fatalf("jobs_start input=%+v", lifecycle.start)
	}
	if lifecycle.start.RequestID == "" || lifecycle.start.RequestID != "worker-job-start-"+fingerprint([]string{binding.SessionID(), "same-call"})[:40] {
		t.Fatalf("jobs_start request identity is not stable and session-bound: %q", lifecycle.start.RequestID)
	}

	stopArgs := json.RawMessage(`{"job_id":"job-start-fixture"}`)
	stopped, stopErr := tools.call(context.Background(), "jobs_stop", "stop-call", stopArgs)
	if stopErr != nil || stopped.Error != "" || lifecycle.stopCalls != 1 || lifecycle.stoppedJobID != "job-start-fixture" || lifecycle.stoppedBinding.SessionID() != binding.SessionID() {
		t.Fatalf("jobs_stop result/lifecycle=%+v/%+v", stopped, lifecycle)
	}
	if lifecycle.stopRequestID != "worker-job-stop-"+fingerprint([]string{binding.SessionID(), "stop-call"})[:40] {
		t.Fatalf("jobs_stop request identity is not stable and session-bound: %q", lifecycle.stopRequestID)
	}
}

func TestProductProjectJobToolsRejectCrossModeAndMalformedStartArguments(t *testing.T) {
	binding := Binding{scope: Scope{company: "company-1"}, task: "task-1", session: "session-1"}
	lifecycle := &productJobLifecycleFixture{}
	base := EmployeeTools{Kernel: &Kernel{}, Binding: binding, ProductSurface: true, ProjectJobsSurface: true, ProjectJobLifecycle: lifecycle}
	tests := []struct {
		name string
		args json.RawMessage
	}{
		{name: "unknown field", args: json.RawMessage(`{"environment_revision_id":"environment-1","kind":"batch","service_id":"","script_path":"scripts/test.mjs","args":[],"command":"sh"}`)},
		{name: "batch with service", args: json.RawMessage(`{"environment_revision_id":"environment-1","kind":"batch","service_id":"web","script_path":"scripts/test.mjs","args":[]}`)},
		{name: "service with caller script", args: json.RawMessage(`{"environment_revision_id":"environment-1","kind":"service","service_id":"web","script_path":"scripts/test.mjs","args":[]}`)},
		{name: "unsupported kind", args: json.RawMessage(`{"environment_revision_id":"environment-1","kind":"shell","service_id":"","script_path":"scripts/test.mjs","args":[]}`)},
		{name: "invalid environment identity", args: json.RawMessage(`{"environment_revision_id":"../outside","kind":"batch","service_id":"","script_path":"scripts/test.mjs","args":[]}`)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := base.call(context.Background(), "jobs_start", "call-"+test.name, test.args)
			if err == nil && result.Error == "" {
				t.Fatalf("malformed start request succeeded: %+v", result)
			}
		})
	}
	if lifecycle.startCalls != 0 {
		t.Fatalf("malformed start requests reached lifecycle adapter %d times", lifecycle.startCalls)
	}
}

func TestProductEnvironmentEnsureRequiresControlBridgeAndUsesBoundWorkerRequest(t *testing.T) {
	binding := Binding{scope: Scope{company: "company-1"}, task: "task-1", session: "session-1"}
	args := json.RawMessage(`{"revision_id":"environment-1"}`)
	if _, err := (EmployeeTools{Kernel: &Kernel{}, Binding: binding, ProductSurface: true, EnvironmentEnsureSurface: true}).call(context.Background(), "environment_ensure", "call", args); !errors.Is(err, core.Denied) {
		t.Fatalf("environment_ensure without the Control executor returned %v, want denied", err)
	}
	lifecycle := &productEnvironmentLifecycleFixture{}
	tools := EmployeeTools{Kernel: &Kernel{}, Binding: binding, ProductSurface: true, EnvironmentEnsureSurface: true, EnvironmentEnsureExecutor: lifecycle}
	first, firstErr := tools.call(context.Background(), "environment_ensure", "same-call", args)
	second, secondErr := tools.call(context.Background(), "environment_ensure", "same-call", args)
	if firstErr != nil || secondErr != nil || first.Error != "" || second.Error != "" || lifecycle.calls != 2 {
		t.Fatalf("environment_ensure results/calls=%+v/%+v/%d errors=%v/%v", first, second, lifecycle.calls, firstErr, secondErr)
	}
	if lifecycle.binding.CompanyID() != binding.CompanyID() || lifecycle.binding.TaskID() != binding.TaskID() || lifecycle.binding.SessionID() != binding.SessionID() || lifecycle.revision != "environment-1" {
		t.Fatalf("environment ensure bridge lost trusted scope: binding=%+v revision=%s", lifecycle.binding, lifecycle.revision)
	}
	if lifecycle.requestID != "env-ensure-worker-"+fingerprint([]string{binding.SessionID(), "same-call"})[:40] {
		t.Fatalf("environment ensure request id is not stable and session-bound: %q", lifecycle.requestID)
	}
	if _, err := tools.call(context.Background(), "environment_ensure", "bad-revision", json.RawMessage(`{"revision_id":"../outside"}`)); !errors.Is(err, core.Malformed) || lifecycle.calls != 2 {
		t.Fatalf("malformed environment revision reached the Control executor: calls=%d err=%v", lifecycle.calls, err)
	}
}
