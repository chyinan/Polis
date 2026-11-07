package kernel

import (
	"encoding/json"
	"strings"
	"testing"

	"polis/internal/core"
)

func TestVisibleSkillPageKeepsApprovedSummariesOnly(t *testing.T) {
	manifest := json.RawMessage(`{"description":"colors","body":"Use the palette.","referenceSnapshots":[{"body":"secret"}]}`)
	sources := []skillSummarySource{
		{ID: "skill-a", PackageID: "brand", Revision: "1", DisplayName: "Brand", SourceRef: "library://skills/brand/SKILL.md", ContentDigest: "sha256:abc", Status: "approved", Manifest: manifest},
		{ID: "skill-b", PackageID: "other", Revision: "2", DisplayName: "Other", SourceRef: "library://skills/other/SKILL.md", ContentDigest: "sha256:def", Status: "revoked", Manifest: manifest},
		{ID: "skill-c", PackageID: "third", Revision: "3", DisplayName: "Third", SourceRef: "library://skills/third/SKILL.md", ContentDigest: "sha256:ghi", Status: "approved", Manifest: manifest},
	}
	draft, err := visibleSkillPage("draft", []string{"skill-a"}, sources, "", 32)
	if err != nil || len(draft.Items) != 0 || draft.NextCursor != nil {
		t.Fatalf("draft = %+v %v", draft, err)
	}
	empty, err := visibleSkillPage("approved", nil, sources, "", 32)
	if err != nil || len(empty.Items) != 0 {
		t.Fatalf("empty binding = %+v %v", empty, err)
	}
	first, err := visibleSkillPage("approved", []string{"skill-a", "skill-b", "skill-c"}, sources, "", 1)
	if err != nil || len(first.Items) != 1 || first.Items[0].ID != "skill-a" || first.Items[0].Purpose != "colors" || first.Items[0].ScriptsExecuted || first.Items[0].ExecutionStatus != "unverified" || first.NextCursor == nil || *first.NextCursor != "skill-a" {
		t.Fatalf("first page = %+v %v", first, err)
	}
	encoded, err := json.Marshal(first.Items[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "Use the palette") || strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), `"body"`) {
		t.Fatalf("summary leaked body: %s", encoded)
	}
	second, err := visibleSkillPage("approved", []string{"skill-a", "skill-b", "skill-c"}, sources, *first.NextCursor, 1)
	if err != nil || len(second.Items) != 1 || second.Items[0].ID != "skill-c" || second.NextCursor != nil {
		t.Fatalf("second page = %+v %v", second, err)
	}
	if _, err = visibleSkillPage("approved", []string{"skill-a"}, sources, "../escape", 32); err != core.Malformed {
		t.Fatalf("cursor err = %v", err)
	}
}
