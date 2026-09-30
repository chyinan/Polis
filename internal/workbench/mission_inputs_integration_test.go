// pattern: Imperative Shell
package workbench

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPostgresReadStoreListsMissionInputRevisionsInCompanyScope(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	connection, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(ctx)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	companyID := "input-read-" + suffix
	missionID := "mission-1"
	if _, err = connection.Exec(ctx, "INSERT INTO companies(id,name,workspace_root,state) VALUES($1,$1,'.','active')", companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = connection.Exec(ctx, "INSERT INTO missions(company_id,id,title,goal,contract) VALUES($1,$2,'goal','goal','r0-arithmetic@1')", companyID, missionID); err != nil {
		t.Fatal(err)
	}
	for _, revision := range []struct {
		number int
		digest string
		name   string
	}{
		{number: 1, digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", name: "goal.md"},
		{number: 2, digest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", name: "goal.md"},
	} {
		requestID := fmt.Sprintf("read-input-%d-%s", revision.number, suffix)
		_, err = connection.Exec(ctx, "INSERT INTO mission_inputs(company_id,input_id,mission_id,revision,request_id,source_kind,display_name,media_type,byte_size,content_digest,state) VALUES($1,'input-1',$2,$3,$4,'upload',$5,'text/markdown',7,$6,'usable')", companyID, missionID, revision.number, requestID, revision.name, revision.digest)
		if err != nil {
			t.Fatal(err)
		}
	}

	store, err := NewPostgresReadStoreWithBlobRoot(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	items, err := store.ListMissionInputs(ctx, companyID, missionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].InputID != "input-1" || items[0].Revision != "2" || items[0].ContentDigest != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" || items[1].Revision != "1" {
		t.Fatalf("mission input revisions = %+v", items)
	}
	if items[0].CompanyID != companyID || items[0].MissionID != missionID || items[0].State != "usable" {
		t.Fatalf("mission input scope/state = %+v", items[0])
	}
	if _, err = store.ListMissionInputs(ctx, "other-company-"+suffix, missionID); !errors.Is(err, errCompanyNotFound) {
		t.Fatalf("cross-company input query error = %v, want not found", err)
	}
}
