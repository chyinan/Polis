package kernel

import (
	"encoding/json"
	"errors"
	"testing"

	"polis/internal/core"
)

func TestSkillReferenceViewReadsFrozenTextWithoutExecution(t *testing.T) {
	document := []byte("---\nname: brand-guidelines\ndescription: colors\n---\nUse the palette.\n")
	parsed, err := parseSkillDocument(document, true, []SkillReferenceFile{{Name: "references/palette.md", Body: []byte("Use blue.\n")}})
	if err != nil {
		t.Fatal(err)
	}
	view, err := skillReferenceView(parsed.Manifest, "references/palette.md")
	if err != nil || view.ScriptsExecuted || view.Body != "Use blue.\n" || view.ContentDigest != contentDigest([]byte("Use blue.\n")) {
		t.Fatalf("view = %+v %v", view, err)
	}
	if _, err = skillReferenceView(parsed.Manifest, "scripts/run.sh"); !errors.Is(err, core.Malformed) {
		t.Fatalf("script path err = %v", err)
	}
	if _, err = skillReferenceView(parsed.Manifest, "references/missing.md"); !errors.Is(err, core.OutOfScope) {
		t.Fatalf("missing err = %v", err)
	}
	var stored skillManifestDocument
	if err = json.Unmarshal(parsed.Manifest, &stored); err != nil {
		t.Fatal(err)
	}
	stored.ReferenceSnapshots[0].Digest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	tampered, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = skillReferenceView(tampered, "references/palette.md"); !errors.Is(err, core.Integrity) {
		t.Fatalf("tampered digest err = %v", err)
	}
}
