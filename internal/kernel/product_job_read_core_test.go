// pattern: Functional Core
package kernel

import (
	"context"
	"errors"
	"testing"

	"polis/internal/core"
)

func TestValidateProductTaskJobReadScopeRequiresCurrentTaskAndWorkerSession(t *testing.T) {
	binding := Binding{scope: Scope{company: "company"}, task: "task", session: "session"}
	record := JobRunRecord{CompanyID: "company", JobID: "job", TaskID: "task", SessionID: "session"}
	if err := validateProductTaskJobReadScope(binding, record, "task", "session"); err != nil {
		t.Fatalf("current task/session was rejected: %v", err)
	}
	for name, input := range map[string]struct {
		record        JobRunRecord
		activeTask    string
		activeSession string
	}{
		"other task job":       {record: JobRunRecord{CompanyID: "company", JobID: "job", TaskID: "other-task", SessionID: "session"}, activeTask: "task", activeSession: "session"},
		"other company job":    {record: JobRunRecord{CompanyID: "other-company", JobID: "job", TaskID: "task", SessionID: "session"}, activeTask: "task", activeSession: "session"},
		"other session job":    {record: record, activeTask: "task", activeSession: "other-session"},
		"stale worker binding": {record: record, activeTask: "task", activeSession: ""},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateProductTaskJobReadScope(binding, input.record, input.activeTask, input.activeSession); !errors.Is(err, core.OutOfScope) {
				t.Fatalf("scope error=%v, want %v", err, core.OutOfScope)
			}
		})
	}
}

func TestEmployeeToolsReadOnlyJobsRequiresTheIsolatedProductSurface(t *testing.T) {
	for _, name := range []string{"jobs_status", "jobs_logs"} {
		_, err := (EmployeeTools{}).call(context.Background(), name, "budget-key", []byte(`{"job_id":"job"}`))
		if !errors.Is(err, core.Denied) {
			t.Fatalf("%s without the isolated product surface returned %v, want %v", name, err, core.Denied)
		}
	}
}

func TestEmployeeToolsBorrowerLeaseRequiresTheIsolatedWritableSurface(t *testing.T) {
	for _, name := range []string{"jobs_borrow", "jobs_touch", "jobs_release"} {
		_, err := (EmployeeTools{}).call(context.Background(), name, "budget-key", []byte(`{}`))
		if !errors.Is(err, core.Denied) {
			t.Fatalf("%s without the isolated borrower lease surface returned %v, want %v", name, err, core.Denied)
		}
	}
	for _, name := range []string{"jobs_borrow", "jobs_touch", "jobs_release"} {
		_, err := (EmployeeTools{ProductSurface: true, BorrowerLeaseSurface: true, ReadOnly: true}).call(context.Background(), name, "budget-key", []byte(`{}`))
		if !errors.Is(err, core.Denied) {
			t.Fatalf("%s on a read-only worker returned %v, want %v", name, err, core.Denied)
		}
	}
}

func TestEmployeeToolsBrowserRunRequiresTheIsolatedSurface(t *testing.T) {
	for _, name := range []string{"browser_run", "browser_results"} {
		_, err := (EmployeeTools{}).call(context.Background(), name, "budget-key", []byte(`{}`))
		if !errors.Is(err, core.Denied) {
			t.Fatalf("%s without the isolated BrowserRun surface returned %v, want %v", name, err, core.Denied)
		}
	}
	_, err := (EmployeeTools{ProductSurface: true, BrowserRunSurface: true, ReadOnly: true}).call(context.Background(), "browser_run", "budget-key", []byte(`{}`))
	if !errors.Is(err, core.Denied) {
		t.Fatalf("browser_run on a read-only worker returned %v, want %v", err, core.Denied)
	}
}

func TestEmployeeToolsResearchOperationsRequireTheIsolatedWritableSurface(t *testing.T) {
	for _, name := range []string{"research_search", "research_fetch"} {
		_, err := (EmployeeTools{}).call(context.Background(), name, "budget-key", []byte(`{}`))
		if !errors.Is(err, core.Denied) {
			t.Fatalf("%s without the isolated research surface returned %v, want %v", name, err, core.Denied)
		}
	}
}
