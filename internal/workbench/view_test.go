// pattern: Functional Core
package workbench

import "testing"

func TestActivityCursorRoundTrip(t *testing.T) {
	const snapshot = "company-seq:70"
	sequence, err := parseSnapshotCursor(snapshot)
	if err != nil {
		t.Fatalf("parse snapshot cursor: %v", err)
	}
	if got := snapshotCursorForSequence(sequence); got != snapshot {
		t.Fatalf("snapshot cursor = %q, want %q", got, snapshot)
	}
	value := "67"
	page, err := parseActivityCursor(&value)
	if err != nil || page == nil || *page != 67 {
		t.Fatalf("activity cursor = %#v, err=%v, want 67", page, err)
	}
}

func TestUnknownRuntimeEventUsesNeutralPresentation(t *testing.T) {
	kind, tone := presentationForEventKind("future.runtime.event")
	if kind != "workspace_updated" || tone != "info" {
		t.Fatalf("unknown event presentation = %q/%q, want workspace_updated/info", kind, tone)
	}
}

func TestProviderInitializationFailureUsesDistinctPresentation(t *testing.T) {
	kind, tone := presentationForEventKind("provider.runtime.initialization_failed")
	if kind != "provider_runtime_initialization_failed" || tone != "danger" {
		t.Fatalf("provider initialization event presentation = %s/%s, want provider_runtime_initialization_failed/danger", kind, tone)
	}
}
