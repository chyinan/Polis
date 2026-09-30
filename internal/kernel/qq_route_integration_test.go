// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"polis/internal/core"
)

func TestQQRouteCannotEnableBeforeQualificationAndDisableAdvancesRevision(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := "qq-route-" + newID()
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	draft := NotificationRouteInput{Adapter: "qq_official", Destination: "admin-openid", SafetyAlias: "Team A", CredentialRef: "default", Enabled: false}
	draftReceipt, err := runtime.TXConfigureNotificationRoute(ctx, scope, draft, "qq-route-draft")
	if err != nil {
		t.Fatal(err)
	}
	if draftReceipt.Status != "configured" || draftReceipt.Revision != 1 {
		t.Fatalf("QQ route draft receipt = %+v", draftReceipt)
	}
	active := draft
	active.Enabled = true
	if _, err = runtime.TXConfigureNotificationRoute(ctx, scope, active, "qq-route-enable-before-qualification"); !errors.Is(err, core.Conflict) {
		t.Fatalf("unqualified QQ route enable error = %v, want conflict", err)
	}
	if _, err = runtime.pool.Exec(ctx, `INSERT INTO qq_channel_qualifications(company_id,route_id,route_revision,target_openid,account_fingerprint,qualification_status,evidence_digest,qualified_until)
VALUES($1,$2,1,$3,$4,'qualified',$5,$6)`, companyID, "default-qq_official", draft.Destination, testFingerprint("account"), testFingerprint("qualification"), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	activeReceipt, err := runtime.TXConfigureNotificationRoute(ctx, scope, active, "qq-route-enable-qualified")
	if err != nil {
		t.Fatal(err)
	}
	if activeReceipt.Status != "ready" || activeReceipt.Revision != 1 {
		t.Fatalf("qualified QQ route receipt = %+v", activeReceipt)
	}
	if _, err = runtime.TXTestNotification(ctx, scope, "qq-route-no-fake-send"); err == nil {
		t.Fatal("QQ notification test faked a delivery without the sender IPC")
	}
	draft.Enabled = false
	revoked, err := runtime.TXConfigureNotificationRoute(ctx, scope, draft, "qq-route-disable")
	if err != nil {
		t.Fatal(err)
	}
	if revoked.Status != "configured" || revoked.Revision != 2 {
		t.Fatalf("disabled QQ route did not advance revision: %+v", revoked)
	}
	var enabled bool
	var revision int64
	var qualificationStatus string
	if err = runtime.pool.QueryRow(ctx, `SELECT enabled,route_revision,qualification_status FROM notification_routes WHERE company_id=$1 AND id='default-qq_official'`, companyID).Scan(&enabled, &revision, &qualificationStatus); err != nil {
		t.Fatal(err)
	}
	if enabled || revision != 2 || qualificationStatus != "revoked" {
		t.Fatalf("disabled route state enabled=%t revision=%d qualification=%s", enabled, revision, qualificationStatus)
	}
}

func testFingerprint(value string) string {
	return fingerprint(value)
}
