// pattern: Imperative Shell
package workbench

import (
	"context"
	"errors"
	"os"
	"testing"

	"polis/internal/kernel"
)

var errStopStreamTest = errors.New("stop stream test")

func TestPostgresStreamReplaysCompanySequenceEvents(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	company := "r06-sse-replay"
	scope, err := runtime.TXCreateCompany(ctx, company)
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.TXCreateMission(ctx, scope, "mission-1"); err != nil {
		t.Fatal(err)
	}
	store, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var received []ActivityEvent
	err = store.StreamActivity(ctx, company, 0, func(event ActivityEvent) error {
		received = append(received, event)
		return errStopStreamTest
	})
	if !errors.Is(err, errStopStreamTest) {
		t.Fatalf("StreamActivity error = %v, want callback stop", err)
	}
	if len(received) != 1 || received[0].CompanySeq == "" || received[0].Kind != "mission_created" {
		t.Fatalf("stream events = %+v", received)
	}
}
