// pattern: Functional Core

package kernel

import "testing"

func TestSameContentClaimIDsPreservesOrderedDraftIdentity(t *testing.T) {
	if !sameContentClaimIDs([]string{"claim-a", "claim-b"}, []string{"claim-a", "claim-b"}) {
		t.Fatal("identical ordered claim list did not match")
	}
	if sameContentClaimIDs([]string{"claim-a", "claim-b"}, []string{"claim-b", "claim-a"}) {
		t.Fatal("claim ordering change was treated as an identical draft request")
	}
}
