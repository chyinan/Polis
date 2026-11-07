package kernel

import (
	"encoding/json"
	"strings"

	"polis/internal/core"
)

type SkillReferenceView struct {
	Reference       string
	ContentDigest   string
	Body            string
	ScriptsExecuted bool
}

func safeSkillReference(name string) bool {
	rest, ok := strings.CutPrefix(name, "references/")
	if !ok || strings.Contains(name, "..") || strings.ContainsAny(name, `\`) {
		return false
	}
	return safeReferenceName(rest)
}

func skillReferenceView(manifest json.RawMessage, reference string) (SkillReferenceView, error) {
	if !safeSkillReference(reference) {
		return SkillReferenceView{}, core.Malformed
	}
	var document struct {
		Snapshots []skillReferenceSnapshot `json:"referenceSnapshots"`
	}
	if len(manifest) > 0 && json.Unmarshal(manifest, &document) != nil {
		return SkillReferenceView{}, core.Malformed
	}
	for _, item := range document.Snapshots {
		if item.Name != reference {
			continue
		}
		if !safeSkillReference(item.Name) || contentDigest([]byte(item.Body)) != item.Digest || item.Digest == "" {
			return SkillReferenceView{}, core.Integrity
		}
		return SkillReferenceView{Reference: item.Name, ContentDigest: item.Digest, Body: item.Body, ScriptsExecuted: false}, nil
	}
	return SkillReferenceView{}, core.OutOfScope
}
