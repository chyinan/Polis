// pattern: Functional Core
package workbench

import "testing"

func TestSelectCurrentCheckpointPrefersNewerQualifiedCheckpoint(t *testing.T) {
	rows := []checkpointRow{
		{ID: "progress-1", SessionEpoch: 3, WorkspaceRevision: 4, Kind: "progress"},
		{ID: "qualified-1", SessionEpoch: 3, WorkspaceRevision: 5, Kind: "qualified"},
	}

	selected := selectCurrentCheckpoint(rows)
	if selected.ID != "qualified-1" {
		t.Fatalf("selected checkpoint = %q, want qualified-1", selected.ID)
	}
}

func TestSelectCurrentCheckpointDoesNotLetAnOlderSessionOverrideCurrentProgress(t *testing.T) {
	rows := []checkpointRow{
		{ID: "qualified-old", SessionEpoch: 2, WorkspaceRevision: 9, Kind: "qualified"},
		{ID: "progress-current", SessionEpoch: 3, WorkspaceRevision: 1, Kind: "progress"},
	}

	selected := selectCurrentCheckpoint(rows)
	if selected.ID != "progress-current" {
		t.Fatalf("selected checkpoint = %q, want progress-current", selected.ID)
	}
}

func TestSelectCurrentCheckpointIsIndependentOfInputOrder(t *testing.T) {
	first := selectCurrentCheckpoint([]checkpointRow{
		{ID: "progress-1", SessionEpoch: 3, WorkspaceRevision: 4, Kind: "progress"},
		{ID: "qualified-1", SessionEpoch: 3, WorkspaceRevision: 5, Kind: "qualified"},
	})
	second := selectCurrentCheckpoint([]checkpointRow{
		{ID: "qualified-1", SessionEpoch: 3, WorkspaceRevision: 5, Kind: "qualified"},
		{ID: "progress-1", SessionEpoch: 3, WorkspaceRevision: 4, Kind: "progress"},
	})

	if first.ID != second.ID {
		t.Fatalf("input order changed selection: first=%q second=%q", first.ID, second.ID)
	}
}
