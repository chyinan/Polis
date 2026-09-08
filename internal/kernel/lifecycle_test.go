// pattern: Imperative Shell
package kernel

import (
	"context"
	"os"
	"polis/internal/core"
	"runtime"
	"testing"
	"time"
)

func TestLostLeaseCannotRecoverOverSuccessor(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("run scripts/test.sh")
	}
	ctx := context.Background()
	root := t.TempDir()
	old, e := Open(ctx, dsn, root)
	must(t, e)
	defer old.Close()
	must(t, old.lease.Conn().Close(ctx))
	next, e := Open(ctx, dsn, root)
	must(t, e)
	defer next.Close()
	_ = old.txRecover(ctx)
	var current string
	must(t, next.pool.QueryRow(ctx, "SELECT incarnation FROM runtime_control").Scan(&current))
	if current != next.incarnation {
		t.Fatal("retired controller overwrote successor incarnation")
	}
	_, e = next.TXCreateCompany(ctx, "new-controller-neighbor")
	must(t, e)
}

func TestCloseReleasesLeaseMutexBeforeWaitingForTransactions(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("run scripts/test.sh")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	k, e := Open(ctx, dsn, t.TempDir())
	must(t, e)
	tx, e := k.pool.Begin(ctx)
	must(t, e)
	defer tx.Rollback(context.Background())
	closed := make(chan struct{})
	go func() { k.Close(); close(closed) }()
	// A held transaction makes pool.Close wait; the lease mutex must still be observable.
	deadline := time.After(time.Second)
	for {
		if k.leaseMu.TryLock() {
			detached := k.lease == nil
			k.leaseMu.Unlock()
			if detached {
				break
			}
		}
		select {
		case <-deadline:
			t.Fatal("Close holds lease mutex while waiting for a transaction")
		default:
			runtime.Gosched()
		}
	}
	wantCode(t, k.guard(ctx, tx, Scope{"company"}, nil), core.StaleEpoch)
	must(t, tx.Rollback(ctx))
	select {
	case <-closed:
	case <-ctx.Done():
		t.Fatal("Close did not finish")
	}
}
