// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/kernel"
	"polis/internal/runner"
)

func TestDeterministicWorkerRetainsOwnershipWhenStopPersistenceFails(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("long-lived deterministic helper script is Linux-specific")
	}
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated temporary PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("r1-stop-retry-%x", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	helper := t.TempDir() + "/deterministic-worker"
	if err = os.WriteFile(helper, []byte("#!/bin/sh\nexec /bin/sleep 60\n"), 0700); err != nil {
		t.Fatal(err)
	}
	adapter := NewDeterministicWorkerAdapter(k)
	adapter.executable = helper
	defer adapter.Close()
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{Title: "retry stop", Goal: "retain worker ownership after a transient stop error", RequestID: companyID + "-create"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: companyID + "-start"}); err != nil {
		t.Fatal(err)
	}
	key := companyID + "/" + created.TargetID
	worker, exists := adapter.workers[key]
	if !exists {
		t.Fatal("started deterministic worker is missing from the adapter")
	}
	defer worker.process.Stop()
	if worker.process.HasExited() {
		t.Fatal("long-lived deterministic helper exited before the stop attempt")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err = adapter.Stop(canceled, companyID, created.TargetID); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled stop error=%v, want context.Canceled", err)
	}
	if retained, ok := adapter.workers[key]; !ok || retained.process != worker.process {
		t.Fatal("worker ownership was discarded after the stop-state write failed")
	}
	if worker.process.HasExited() {
		t.Fatal("failed stop-state write unexpectedly stopped the process")
	}
	if err = adapter.Stop(ctx, companyID, created.TargetID); err != nil {
		t.Fatalf("retry stop: %v", err)
	}
	if _, ok := adapter.workers[key]; ok {
		t.Fatal("worker ownership remained after a confirmed stop")
	}
	if !worker.process.HasExited() {
		t.Fatal("process remained alive after a confirmed stop")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var state string
	if err = pool.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1", companyID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "stopped" {
		t.Fatalf("worker session state=%q after successful retry, want stopped", state)
	}
}

func TestDeterministicStopRetriesProcessAttachmentBeforeConfirmation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("long-lived process fixture is Linux-specific")
	}
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated temporary PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("r1-stop-attach-retry-%x", time.Now().UTC().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	task, err := k.TXCreateProbe(ctx, scope, "r1-stop-attach-retry-task")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := k.TXNewWorker(ctx, scope, task.ID, "r1-stop-attach-retry")
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXBindWorkerProcessHost(ctx, binding, runner.CurrentProcessContainmentMetadata()); err != nil {
		t.Fatal(err)
	}
	process, err := runner.Start(binding.SessionID(), []string{"/bin/sh", "-c", "exec /bin/sleep 60"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Stop()
	adapter := NewDeterministicWorkerAdapter(k)
	key := companyID + "/r1-stop-attach-retry-mission"
	adapter.workers[key] = deterministicWorker{binding: binding, process: process}
	if err = adapter.Stop(ctx, companyID, "r1-stop-attach-retry-mission"); err != nil {
		t.Fatalf("stop retained deterministic Worker whose PID attach was not persisted: %v", err)
	}
	if _, exists := adapter.workers[key]; exists {
		t.Fatal("confirmed deterministic Worker remained owned by the adapter")
	}
	if !process.HasExited() {
		t.Fatal("process remained alive after confirmed deterministic stop")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var state string
	var pidAttached bool
	if err = pool.QueryRow(ctx, "SELECT state,process_pid IS NOT NULL FROM worker_sessions WHERE company_id=$1", companyID).Scan(&state, &pidAttached); err != nil {
		t.Fatal(err)
	}
	if state != "stopped" || !pidAttached {
		t.Fatalf("stopped deterministic session state=%q pid_attached=%v", state, pidAttached)
	}
}

func TestDeterministicStopReplaysConfirmationAfterAmbiguousCommit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("long-lived process fixture is Linux-specific")
	}
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated temporary PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("r1-stop-confirm-replay-%x", time.Now().UTC().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	task, err := k.TXCreateProbe(ctx, scope, "r1-stop-confirm-replay-task")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := k.TXNewWorker(ctx, scope, task.ID, "r1-stop-confirm-replay")
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXBindWorkerProcessHost(ctx, binding, runner.CurrentProcessContainmentMetadata()); err != nil {
		t.Fatal(err)
	}
	process, err := runner.Start(binding.SessionID(), []string{"/bin/sh", "-c", "exec /bin/sleep 60"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Stop()
	if err = k.TXAttachWorker(ctx, binding, process); err != nil {
		t.Fatal(err)
	}
	if err = k.TXBeginStop(ctx, binding); err != nil {
		t.Fatal(err)
	}
	proof, err := process.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXConfirmStopped(ctx, binding, proof); err != nil {
		t.Fatal(err)
	}
	adapter := NewDeterministicWorkerAdapter(k)
	key := companyID + "/r1-stop-confirm-replay-mission"
	adapter.workers[key] = deterministicWorker{binding: binding, process: process, processStopped: true, stopProof: proof}
	if err = adapter.Stop(ctx, companyID, "r1-stop-confirm-replay-mission"); err != nil {
		t.Fatalf("replay stop confirmation after ambiguous commit: %v", err)
	}
	if _, exists := adapter.workers[key]; exists {
		t.Fatal("worker ownership remained after replayed stop confirmation")
	}
}

func TestDeterministicStopRetainsWindowsHostReconciliationOwner(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated temporary PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("r1-host-reconcile-owner-%x", time.Now().UTC().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	task, err := k.TXCreateProbe(ctx, scope, "r1-host-reconcile-owner-task")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := k.TXNewWorker(ctx, scope, task.ID, "r1-host-reconcile-owner")
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXBindWorkerProcessHost(ctx, binding, runner.CurrentProcessContainmentMetadata()); err != nil {
		t.Fatal(err)
	}
	adapter := NewDeterministicWorkerAdapter(k)
	key := companyID + "/r1-host-reconcile-owner-mission"
	adapter.workers[key] = deterministicWorker{binding: binding, hostReconcileOnly: true}
	if err = adapter.Stop(ctx, companyID, "r1-host-reconcile-owner-mission"); err == nil {
		t.Fatal("unsupported host reconciliation was reported as confirmed")
	}
	if _, exists := adapter.workers[key]; !exists {
		t.Fatal("host-reconciliation-only Worker owner was discarded after an unverified stop")
	}
}
