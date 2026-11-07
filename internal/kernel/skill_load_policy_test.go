package kernel

import (
	"encoding/json"
	"errors"
	"testing"

	"polis/internal/core"
)

func TestAuthorizeSkillLoadRequiresApprovedBindingAndActiveTask(t *testing.T) {
	if err := authorizeSkillLoad("emp-backend", "working", "emp-backend", "skill-1", "approved", "approved", []string{"skill-1"}); err != nil {
		t.Fatal(err)
	}
	if err := authorizeSkillLoad("emp-backend", "ready", "emp-backend", "skill-1", "candidate", "approved", []string{"skill-1"}); !errors.Is(err, core.Denied) {
		t.Fatalf("candidate skill err = %v", err)
	}
	if err := authorizeSkillLoad("emp-backend", "working", "emp-backend", "skill-1", "approved", "draft", []string{"skill-1"}); !errors.Is(err, core.Denied) {
		t.Fatalf("draft binding err = %v", err)
	}
	if err := authorizeSkillLoad("emp-backend", "working", "emp-frontend", "skill-1", "approved", "approved", []string{"skill-1"}); !errors.Is(err, core.Denied) {
		t.Fatalf("other employee err = %v", err)
	}
	if err := authorizeSkillLoad("emp-backend", "completed", "emp-backend", "skill-1", "approved", "approved", []string{"skill-1"}); !errors.Is(err, core.Denied) {
		t.Fatalf("completed task err = %v", err)
	}
	if err := authorizeSkillLoad("emp-backend", "working", "emp-backend", "skill-1", "approved", "approved", []string{"other"}); !errors.Is(err, core.Denied) {
		t.Fatalf("unbound skill err = %v", err)
	}
}

func TestSkillLoadViewReturnsBodyWithoutExecutionOrPermission(t *testing.T) {
	manifest := json.RawMessage(`{"body":"Use the palette.","scriptsPresent":true,"allowedTools":["shell"]}`)
	view, err := skillLoadView(manifest, "sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	if view.Body != "Use the palette." || view.ScriptsExecuted || view.GrantsPermission || len(view.References) != 0 || view.ContentDigest != "sha256:abc" {
		t.Fatalf("view = %+v", view)
	}
}
