// pattern: Imperative Shell
package workbench

import (
	"context"
	"os"
	"testing"

	"polis/internal/kernel"
)

func TestPostgresLocalNotificationDeliveryIsReadable(t *testing.T) {
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
	scope, err := runtime.TXCreateCompany(ctx, "r06-notifications")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXConfigureNotificationRoute(ctx, scope, kernel.NotificationRouteInput{Adapter: "local", Destination: "local://workbench", Enabled: true}, "route-1"); err != nil {
		t.Fatal(err)
	}
	receipt, err := runtime.TXTestNotification(ctx, scope, "test-1")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != "delivered" {
		t.Fatalf("notification test receipt = %+v", receipt)
	}
	store, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	view, err := store.ListNotifications(ctx, "r06-notifications")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Routes) != 1 || !view.Routes[0].Enabled || len(view.Deliveries) != 1 || view.Deliveries[0].State != "delivered" {
		t.Fatalf("notifications view = %+v", view)
	}
}
