// pattern: Functional Core
package runner

import (
	"errors"
	"reflect"
	"testing"
)

func TestMergeLoopbackSIDStringsPreservesUnownedEntries(t *testing.T) {
	existing := []string{"S-1-5-18", "S-1-15-2-100", "S-1-15-2-200"}
	added := MergeLoopbackSIDStrings(existing, "S-1-15-2-300", true)
	if !reflect.DeepEqual(added, []string{"S-1-5-18", "S-1-15-2-100", "S-1-15-2-200", "S-1-15-2-300"}) {
		t.Fatalf("loopback allowlist after add = %v", added)
	}
	removed := MergeLoopbackSIDStrings(added, "S-1-15-2-100", false)
	if !reflect.DeepEqual(removed, []string{"S-1-5-18", "S-1-15-2-200", "S-1-15-2-300"}) {
		t.Fatalf("loopback allowlist after removing the owned SID = %v", removed)
	}
	if !reflect.DeepEqual(existing, []string{"S-1-5-18", "S-1-15-2-100", "S-1-15-2-200"}) {
		t.Fatalf("merge mutated the caller's snapshot: %v", existing)
	}
}

func TestEnableLoopbackSIDRollsBackCommittedChangeWhenCleanupFails(t *testing.T) {
	cleanupErr := errors.New("heap release failed")
	var calls []bool
	err := enableLoopbackSIDWithRollback(func(enabled bool) (bool, error) {
		calls = append(calls, enabled)
		if enabled {
			return true, cleanupErr
		}
		return true, nil
	})
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("enable loopback error = %v, want cleanup failure", err)
	}
	if !reflect.DeepEqual(calls, []bool{true, false}) {
		t.Fatalf("loopback operation sequence = %v, want enable then rollback", calls)
	}
}

func TestEnableLoopbackSIDDoesNotRollbackWhenSetDidNotCommit(t *testing.T) {
	setErr := errors.New("loopback set denied")
	var calls []bool
	err := enableLoopbackSIDWithRollback(func(enabled bool) (bool, error) {
		calls = append(calls, enabled)
		return false, setErr
	})
	if !errors.Is(err, setErr) {
		t.Fatalf("enable loopback error = %v, want set failure", err)
	}
	if !reflect.DeepEqual(calls, []bool{true}) {
		t.Fatalf("loopback operation sequence = %v, want enable only", calls)
	}
}

func TestMergeLoopbackSIDStringsIsIdempotentAndCaseInsensitive(t *testing.T) {
	existing := []string{"S-1-5-18", "S-1-15-2-AbCd"}
	added := MergeLoopbackSIDStrings(existing, "s-1-15-2-abcd", true)
	if !reflect.DeepEqual(added, existing) {
		t.Fatalf("adding an existing SID changed the allowlist: %v", added)
	}
	removed := MergeLoopbackSIDStrings(existing, "S-1-15-2-missing", false)
	if !reflect.DeepEqual(removed, existing) {
		t.Fatalf("removing an unowned SID changed the allowlist: %v", removed)
	}
}

func TestMergeLoopbackAppContainerEntriesPreservesAttributesForOtherSIDs(t *testing.T) {
	existing := []AppContainerLoopbackSID{
		{SID: "S-1-5-18", Attributes: 4},
		{SID: "S-1-15-2-100", Attributes: 8},
	}
	added := MergeLoopbackAppContainerSIDEntries(existing, "S-1-15-2-300", true)
	wantAdded := []AppContainerLoopbackSID{
		{SID: "S-1-5-18", Attributes: 4},
		{SID: "S-1-15-2-100", Attributes: 8},
		{SID: "S-1-15-2-300", Attributes: 0},
	}
	if !reflect.DeepEqual(added, wantAdded) {
		t.Fatalf("loopback SID entries after add = %+v", added)
	}
	removed := MergeLoopbackAppContainerSIDEntries(added, "S-1-15-2-100", false)
	if !reflect.DeepEqual(removed, []AppContainerLoopbackSID{
		{SID: "S-1-5-18", Attributes: 4},
		{SID: "S-1-15-2-300", Attributes: 0},
	}) {
		t.Fatalf("loopback SID entries after removal = %+v", removed)
	}
}
