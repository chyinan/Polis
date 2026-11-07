// pattern: Functional Core
package kernel

import (
	"encoding/json"

	"polis/internal/core"
)

type SkillLoadView struct {
	Body             string
	ContentDigest    string
	ScriptsExecuted  bool
	References       []string
	GrantsPermission bool
}

func authorizeSkillLoad(taskOwner, taskState, employeeID, skillID, skillStatus, bindingStatus string, boundSkillIDs []string) error {
	if !validEmployeeID(employeeID) || !core.ValidID(skillID) || employeeID != taskOwner {
		return core.Denied
	}
	if taskState != "ready" && taskState != "working" {
		return core.Denied
	}
	if skillStatus != "approved" || bindingStatus != "approved" {
		return core.Denied
	}
	for _, id := range boundSkillIDs {
		if id == skillID {
			return nil
		}
	}
	return core.Denied
}

func skillLoadView(manifest json.RawMessage, digest string) (SkillLoadView, error) {
	if digest == "" {
		return SkillLoadView{}, core.Malformed
	}
	body := ""
	if len(manifest) > 0 {
		var document struct {
			Body       string   `json:"body"`
			References []string `json:"references"`
		}
		if err := json.Unmarshal(manifest, &document); err != nil {
			return SkillLoadView{}, core.Malformed
		}
		body = document.Body
		if document.References != nil {
			return SkillLoadView{Body: body, ContentDigest: digest, ScriptsExecuted: false, References: document.References, GrantsPermission: false}, nil
		}
	}
	return SkillLoadView{Body: body, ContentDigest: digest, ScriptsExecuted: false, References: []string{}, GrantsPermission: false}, nil
}
