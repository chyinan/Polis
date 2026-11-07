// pattern: Functional Core
package kernel

import "testing"

func TestNotificationTestReceiptPreservesSelectedAdapter(t *testing.T) {
	for _, test := range []struct {
		adapter string
		state   string
	}{
		{adapter: "local", state: "delivered"},
		{adapter: "webhook", state: "accepted"},
	} {
		receipt := notificationTestReceipt(test.adapter, "intent-1", "delivery-1", test.state)
		if receipt.Adapter != test.adapter || receipt.State != test.state {
			t.Fatalf("receipt=%+v, want adapter=%s state=%s", receipt, test.adapter, test.state)
		}
	}
}
