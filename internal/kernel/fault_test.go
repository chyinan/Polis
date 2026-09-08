// pattern: Imperative Shell
package kernel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"os/exec"
	"path/filepath"
	"polis/internal/core"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFaultBoundaries(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("run scripts/test.sh with dedicated PG")
	}
	ctx := context.Background()
	root := t.TempDir()
	k, e := Open(ctx, dsn, root)
	must(t, e)
	defer func() { k.Close() }()
	scope, e := k.TXCreateCompany(ctx, "fault-company")
	must(t, e)
	must(t, k.TXCreateMission(ctx, scope, "fault-mission"))
	_, e = k.TXStartMission(ctx, scope, "fault-mission", "start")
	must(t, e)
	t.Run("singleton-controller", func(t *testing.T) {
		other, e := Open(ctx, dsn, root)
		if other != nil {
			other.Close()
		}
		wantCode(t, e, core.Denied)
	})
	t.Run("FT-45-transaction-rollback", func(t *testing.T) {
		before, e := k.Snapshot(ctx, scope, "fault-mission")
		must(t, e)
		injected := errors.New("injected before commit")
		_, e = k.TXWrite(ctx, scope, nil, "rollback", "fault", nil, func(tx pgx.Tx) (Receipt, error) {
			_, e := tx.Exec(ctx, "UPDATE missions SET state='paused' WHERE company_id=$1", scope.company)
			if e != nil {
				return Receipt{}, e
			}
			return Receipt{}, injected
		})
		if !errors.Is(e, injected) {
			t.Fatal(e)
		}
		after, e := k.Snapshot(ctx, scope, "fault-mission")
		must(t, e)
		if after.MissionState != before.MissionState || after.CompanySeq != before.CompanySeq {
			t.Fatal("partial commit")
		}
	})
	planner, e := k.BindFake(ctx, scope, "emp-planning")
	must(t, e)
	work, e := k.TXClaim(ctx, planner)
	must(t, e)
	t.Run("FT-01-concurrent-message-replay", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, e := k.TXSend(ctx, planner, work, "send", "compute fixed sum")
				if e != nil {
					t.Error(e)
				}
			}()
		}
		wg.Wait()
		s, e := k.Snapshot(ctx, scope, "fault-mission")
		must(t, e)
		if len(s.Messages) != 1 || len(s.Obligations) != 1 {
			t.Fatal("duplicate concurrent request")
		}
	})
	t.Run("FT-67-query-does-not-change-incarnation-or-state", func(t *testing.T) {
		before, e := k.Snapshot(ctx, scope, "fault-mission")
		must(t, e)
		after, e := ReadSnapshot(ctx, dsn, scope.company, "fault-mission")
		must(t, e)
		if after.CompanySeq != before.CompanySeq {
			t.Fatal("query changed state")
		}
		_, e = k.TXSend(ctx, planner, work, "send", "compute fixed sum")
		must(t, e)
	})
	backend, e := k.BindFake(ctx, scope, "emp-backend")
	must(t, e)
	w, e := k.TXClaim(ctx, backend)
	must(t, e)
	t.Run("FT-16-foreign-write-with-valid-identity", func(t *testing.T) {
		other, e := k.TXCreateCompany(ctx, "fault-foreign")
		must(t, e)
		foreign, e := k.BindFake(ctx, other, "emp-backend")
		must(t, e)
		_, e = k.TXSubmit(ctx, foreign, w, "foreign", []byte("Polis R0: 2 + 3 = 5\n"))
		wantCode(t, e, core.OutOfScope)
	})
	t.Run("FT-06-task-generation-neighbor", func(t *testing.T) {
		old := w
		old.Generation--
		_, e = k.TXSubmit(ctx, backend, old, "old-generation", []byte("bad"))
		wantCode(t, e, core.Conflict)
	})
	artifact, e := k.TXSubmit(ctx, backend, w, "wrong-artifact", []byte("Polis R0: 2 + 3 = 6\n"))
	must(t, e)
	t.Run("publication-has-durable-staging-record", func(t *testing.T) {
		var count int
		e := k.pool.QueryRow(ctx, "SELECT count(*) FROM artifact_staging WHERE company_id=$1 AND id=$2", scope.company, artifact.ID).Scan(&count)
		must(t, e)
		if count != 1 {
			t.Fatal("publication lacks staging provenance")
		}
	})
	t.Run("bad-artifact-is-never-accepted", func(t *testing.T) {
		reviewer, e := k.BindFake(ctx, scope, "emp-review")
		must(t, e)
		r, e := k.TXVerify(ctx, reviewer, artifact.ID, "reject-wrong")
		must(t, e)
		if r.Status != "failed" {
			t.Fatal("bad result passed")
		}
	})
	t.Run("missing-artifact-on-restart", func(t *testing.T) {
		s, e := k.Snapshot(ctx, scope, "fault-mission")
		must(t, e)
		must(t, os.Remove(filepath.Join(root, scope.company, s.Artifacts[0].Digest)))
		_, e = k.TXResolve(ctx, backend, s.Obligations[0].ID, artifact.ID, "missing-response")
		wantCode(t, e, core.Integrity)
		k.Close()
		k, e = Open(ctx, dsn, root)
		must(t, e)
		s, e = k.Snapshot(ctx, scope, "fault-mission")
		must(t, e)
		if s.Artifacts[0].State != "missing" || s.Artifacts[0].Verdict != "invalidated" {
			t.Fatal("missing artifact remained usable")
		}
	})
}

// The test helper is a separate OS process, killed without Close or final summary.
func TestCrashHelper(t *testing.T) {
	if os.Getenv("POLIS_CRASH_HELPER") != "1" {
		return
	}
	ctx := context.Background()
	k, e := Open(ctx, os.Getenv("POLIS_TEST_DSN"), os.Getenv("POLIS_CRASH_ROOT"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = k.Step(ctx, k.LocalScope("crash-company"))
	if e != nil {
		t.Fatal(e)
	}
	fmt.Println("COMMITTED")
	// Parent keeps stdin open and kills this process after observing the commit.
	_, _ = bufio.NewReader(os.Stdin).ReadByte()
	k.Close()
}
func TestProcessCrashRecovery(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("run scripts/test.sh")
	}
	ctx := context.Background()
	root := t.TempDir()
	k, e := Open(ctx, dsn, root)
	must(t, e)
	s, e := k.TXCreateCompany(ctx, "crash-company")
	must(t, e)
	must(t, k.TXCreateMission(ctx, s, "crash-mission"))
	_, e = k.TXStartMission(ctx, s, "crash-mission", "start")
	must(t, e)
	k.Close()
	for _, phase := range []string{"request", "artifact"} {
		exe, e := os.Executable()
		must(t, e)
		childCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		cmd := exec.CommandContext(childCtx, exe, "-test.run=^TestCrashHelper$")
		cmd.Env = append(os.Environ(), "POLIS_CRASH_HELPER=1", "POLIS_CRASH_ROOT="+root)
		stdout, e := cmd.StdoutPipe()
		must(t, e)
		stdin, e := cmd.StdinPipe()
		must(t, e)
		cmd.Stderr = os.Stderr
		must(t, cmd.Start())
		scan := bufio.NewScanner(stdout)
		committed := false
		for scan.Scan() {
			if strings.TrimSpace(scan.Text()) == "COMMITTED" {
				committed = true
				break
			}
		}
		if !committed {
			_ = cmd.Wait()
			cancel()
			t.Fatalf("child did not commit %s", phase)
		}
		must(t, cmd.Process.Kill())
		_ = stdin.Close()
		if e = cmd.Wait(); e == nil {
			t.Fatal("expected killed process")
		}
		cancel()
		k, e = Open(ctx, dsn, root)
		must(t, e)
		state, e := k.Snapshot(ctx, s, "crash-mission")
		must(t, e)
		if len(state.Obligations) != 1 || state.Obligations[0].State != "pending" || len(state.Messages) != 1 {
			t.Fatal("lost responsibility at process crash")
		}
		if phase == "artifact" && (len(state.Artifacts) != 1 || state.Artifacts[0].State != "ready") {
			t.Fatal("lost fixed artifact")
		}
		k.Close()
	}
	k, e = Open(ctx, dsn, root)
	must(t, e)
	defer k.Close()
	for i := 0; i < 5; i++ {
		_, e = k.Step(ctx, s)
		must(t, e)
	}
	state, e := k.Snapshot(ctx, s, "crash-mission")
	must(t, e)
	if state.Obligations[0].State != "fulfilled" || state.Artifacts[0].Verdict != "passed" || state.FakeClaims != 2 {
		t.Fatalf("incomplete recovery: %+v", state)
	}
}
