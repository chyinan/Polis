// pattern: Imperative Shell
//go:build windows

package kernel

import (
	"context"
	"os"
	"testing"
	"time"

	"polis/internal/runner"
)

func TestWindowsWorkerProcessHelper(t *testing.T) {
	if os.Getenv("POLIS_KERNEL_WORKER_HELPER") == "1" {
		time.Sleep(5 * time.Minute)
	}
}

func TestWindowsKernelReapsRestartedWorkerSessionWithoutRetryingTask(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "windows-worker-recovery-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	task, err := k.TXCreateProbe(ctx, scope, "windows-worker-recovery-mission")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := k.TXNewWorker(ctx, scope, task.ID, "offline-host-recovery-helper")
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	process, err := runner.Start(binding.SessionID(), []string{executable, "-test.run=^TestWindowsWorkerProcessHelper$"}, []string{"POLIS_KERNEL_WORKER_HELPER=1"})
	if err != nil {
		t.Fatal(err)
	}
	defer process.Stop()
	if err = k.TXAttachWorker(ctx, binding, process); err != nil {
		t.Fatal(err)
	}
	if err = k.TXValidateWorker(ctx, binding); err != nil {
		t.Fatal(err)
	}
	if err = k.TXActivateWorker(ctx, binding, "offline-host-recovery-helper"); err != nil {
		t.Fatal(err)
	}
	if err = k.txResetFakeState(ctx); err != nil {
		t.Fatal(err)
	}
	summary, err := k.ReconcileWindowsWorkerSessions(ctx)
	if err != nil || summary.Candidates != 1 || summary.Stopped != 1 || summary.Unresolved != 0 {
		t.Fatalf("host reconciliation summary=%+v err=%v", summary, err)
	}
	if waitErr := process.WaitError(); waitErr == nil {
		t.Fatal("reconciled Worker helper exited successfully instead of being stopped")
	}
	var sessionState, taskState, missionState string
	if err = k.pool.QueryRow(ctx, `SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2`, scope.company, binding.SessionID()).Scan(&sessionState); err != nil {
		t.Fatal(err)
	}
	if err = k.pool.QueryRow(ctx, `SELECT state FROM tasks WHERE company_id=$1 AND id=$2`, scope.company, task.ID).Scan(&taskState); err != nil {
		t.Fatal(err)
	}
	if err = k.pool.QueryRow(ctx, `SELECT state FROM missions WHERE company_id=$1 AND id=$2`, scope.company, task.Mission).Scan(&missionState); err != nil {
		t.Fatal(err)
	}
	if sessionState != "stopped" || taskState != "working" || missionState != "active" {
		t.Fatalf("recovery replayed or reopened work: session=%s task=%s mission=%s", sessionState, taskState, missionState)
	}
}
