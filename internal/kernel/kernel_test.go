// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"polis/internal/core"
)

func TestPostgresSlice(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required: run scripts/test.sh")
	}
	ctx := context.Background()
	root := t.TempDir()
	k, err := Open(ctx, dsn, root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { k.Close() }()
	a, err := k.TXCreateCompany(ctx, "company-a")
	must(t, err)
	b, err := k.TXCreateCompany(ctx, "company-b")
	must(t, err)
	must(t, k.TXCreateMission(ctx, a, "mission-a"))
	must(t, k.TXCreateMission(ctx, a, "mission-other"))
	must(t, k.TXCreateMission(ctx, b, "mission-b"))

	t.Run("FT-57-concurrent-start", func(t *testing.T) {
		var wg sync.WaitGroup
		ids := make(chan string, 12)
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r, e := k.TXStartMission(ctx, a, "mission-a", "start")
				if e != nil {
					t.Error(e)
					return
				}
				ids <- r.ID
			}()
		}
		wg.Wait()
		close(ids)
		first := ""
		for id := range ids {
			if first == "" {
				first = id
			}
			if id != first {
				t.Fatal("duplicate activation")
			}
		}
		r, e := k.TXStartMission(ctx, a, "mission-a", "another-start-key")
		must(t, e)
		if r.ID != first {
			t.Fatal("new request key created a second activation")
		}
		s, e := k.Snapshot(ctx, a, "mission-a")
		must(t, e)
		if len(s.Tasks) != 1 || len(s.Employees) != 4 {
			t.Fatalf("unexpected bootstrap: %+v", s)
		}
	})
	t.Run("FT-58-paused-mission-keeps-slot", func(t *testing.T) {
		must(t, k.TXSetPaused(ctx, a, "mission-a", true))
		_, e := k.TXStartMission(ctx, a, "mission-other", "other")
		wantCode(t, e, core.Conflict)
		planner, e := k.BindFake(ctx, a, "emp-planning")
		must(t, e)
		_, e = k.TXClaim(ctx, planner)
		wantCode(t, e, core.Denied)
		must(t, k.TXSetPaused(ctx, a, "mission-a", false))
	})
	planner, e := k.BindFake(ctx, a, "emp-planning")
	must(t, e)
	planning, e := k.TXClaim(ctx, planner)
	must(t, e)
	var request Receipt
	t.Run("FT-01-62-message-idempotency-and-conflict", func(t *testing.T) {
		request, e = k.TXSend(ctx, planner, planning, "send-fixed", "compute fixed sum")
		must(t, e)
		r, e := k.TXSend(ctx, planner, planning, "send-fixed", "compute fixed sum")
		must(t, e)
		if r.ID != request.ID {
			t.Fatal("duplicate message")
		}
		_, e = k.TXSend(ctx, planner, planning, "send-fixed", "changed body")
		wantCode(t, e, core.Conflict)
		_, e = k.TXSend(ctx, planner, planning, "oversized", strings.Repeat("x", 4097))
		wantCode(t, e, core.TooLarge)
		s, e := k.Snapshot(ctx, a, "mission-a")
		must(t, e)
		if len(s.Messages) != 1 || len(s.Obligations) != 1 || len(s.Tasks) != 2 || s.FakeClaims != 1 {
			t.Fatalf("duplicate state: %+v", s)
		}
	})
	t.Run("FT-16-scope-deny-allow-neighbors", func(t *testing.T) {
		_, e := k.Snapshot(ctx, b, "mission-a")
		wantCode(t, e, core.OutOfScope)
		_, e = k.TXStartMission(ctx, b, "mission-a", "foreign")
		wantCode(t, e, core.OutOfScope)
		_, e = k.Snapshot(ctx, a, "mission-a")
		must(t, e)
	})
	old := planner
	t.Run("FT-02-27-restart-recovers-pending-responsibility", func(t *testing.T) {
		k.Close()
		k, e = Open(ctx, dsn, root)
		must(t, e)
		a = k.LocalScope("company-a")
		b = k.LocalScope("company-b")
		s, e := k.Snapshot(ctx, a, "mission-a")
		must(t, e)
		if len(s.Obligations) != 1 || s.Obligations[0].State != "pending" || len(s.Messages) != 1 {
			t.Fatal("lost committed responsibility")
		}
		_, e = k.TXSend(ctx, old, planning, "old", "compute fixed sum")
		wantCode(t, e, core.StaleEpoch)
	})
	backend, e := k.BindFake(ctx, a, "emp-backend")
	must(t, e)
	work, e := k.TXClaim(ctx, backend)
	must(t, e)
	var artifact Receipt
	t.Run("FT-06-stale-epoch-valid-neighbor", func(t *testing.T) {
		stale := backend
		stale.epoch--
		_, e = k.TXSubmit(ctx, stale, work, "artifact-old", []byte("Polis R0: 2 + 3 = 5\n"))
		wantCode(t, e, core.StaleEpoch)
		artifact, e = k.TXSubmit(ctx, backend, work, "artifact", []byte("Polis R0: 2 + 3 = 5\n"))
		must(t, e)
	})
	t.Run("FT-73-response-requires-evidence", func(t *testing.T) {
		_, e = k.TXResolve(ctx, backend, request.ID, "missing", "resolve-bad")
		wantCode(t, e, core.OutOfScope)
		_, e = k.TXResolve(ctx, backend, request.ID, artifact.ID, "resolve")
		must(t, e)
		_, e = k.TXResolve(ctx, backend, request.ID, artifact.ID, "resolve")
		must(t, e)
		s, e := k.Snapshot(ctx, a, "mission-a")
		must(t, e)
		if s.Obligations[0].State != "fulfilled" || len(s.Messages) != 2 {
			t.Fatal("response not atomically persisted")
		}
	})
	t.Run("FT-38-independent-fixed-verification", func(t *testing.T) {
		_, e := k.TXVerify(ctx, backend, artifact.ID, "self")
		wantCode(t, e, core.Denied)
		review, e := k.BindFake(ctx, a, "emp-review")
		must(t, e)
		r, e := k.TXVerify(ctx, review, artifact.ID, "review")
		must(t, e)
		if r.Status != "passed" {
			t.Fatalf("valid result: %+v", r)
		}
	})
	t.Run("FT-69-fixed-artifact-survives-restart", func(t *testing.T) {
		before, e := k.Snapshot(ctx, a, "mission-a")
		must(t, e)
		k.Close()
		k, e = Open(ctx, dsn, root)
		must(t, e)
		a = k.LocalScope("company-a")
		after, e := k.Snapshot(ctx, a, "mission-a")
		must(t, e)
		if len(after.Artifacts) != 1 || after.Artifacts[0].Digest != before.Artifacts[0].Digest || after.Obligations[0].State != "fulfilled" {
			t.Fatal("lost artifact or resolution")
		}
	})
	t.Run("FT-31-81-idle-no-repeated-claims", func(t *testing.T) {
		before, e := k.Snapshot(ctx, a, "mission-a")
		must(t, e)
		for i := 0; i < 20; i++ {
			worked, e := k.Step(ctx, a)
			must(t, e)
			if worked {
				t.Fatal("idle work claimed")
			}
		}
		after, e := k.Snapshot(ctx, a, "mission-a")
		must(t, e)
		if before.FakeClaims != after.FakeClaims {
			t.Fatal("idle work billed")
		}
	})
	t.Run("FT-69-corrupt-file-invalidates-acceptance", func(t *testing.T) {
		s, e := k.Snapshot(ctx, a, "mission-a")
		must(t, e)
		path := filepath.Join(root, "company-a", s.Artifacts[0].Digest)
		must(t, os.Chmod(path, 0600))
		must(t, os.WriteFile(path, []byte("bad"), 0600))
		k.Close()
		k, e = Open(ctx, dsn, root)
		must(t, e)
		a = k.LocalScope("company-a")
		s, e = k.Snapshot(ctx, a, "mission-a")
		must(t, e)
		if s.Artifacts[0].State != "corrupt" || s.Artifacts[0].Verdict == "passed" {
			t.Fatal("corrupt file retained acceptance")
		}
	})
}

func must(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
func wantCode(t *testing.T, e error, c core.Code) {
	t.Helper()
	if !errors.Is(e, c) {
		t.Fatalf("got %v, want %s", e, c)
	}
}
