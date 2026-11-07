// pattern: Imperative Shell
package kernel

import (
	"encoding/json"
	"polis/internal/core"
	"polis/internal/fixture"
	"testing"
)

func TestT21CQualifiedCheckpointMustBindEffectiveContract(t *testing.T) {
	env := newPeerHardeningEnv(t, 48)
	revision := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "t21c-v2")
	w, err := env.k.Workspace(env.ctx, env.backend)
	must(t, err)
	_, err = env.k.TXReplace(env.ctx, env.backend, "t21c-replace", w.Digest, fixture.PeerBackendV2)
	must(t, err)
	checked, err := env.k.TXPeerCheck(env.ctx, env.backend, "t21c-check")
	must(t, err)
	if !checked.Passed || checked.Receipt == nil {
		t.Fatalf("qualified setup check: %+v", checked)
	}
	cp := Checkpoint{Kind: CheckpointQualified, Summary: "current response checked", Facts: []string{"checked current workspace"}, Decisions: []string{"use accepted contract"}, Rejected: []string{"unchecked delivery"}, EvidenceRefs: []string{checked.Receipt.ID}, NextAction: "submit", FailedChecks: []string{}}
	raw, err := json.Marshal(cp)
	must(t, err)
	tools := PeerEmployeeTools{Kernel: env.k, Binding: env.backend, Role: "peer_backend"}
	saved := tools.Call(env.ctx, "work_checkpoint", "t21c-qualified", raw)
	if saved.Error != "" || saved.Receipt == nil {
		t.Fatalf("checkpoint setup: %+v", saved)
	}
	next := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":"","total":0}`, "t21c-v3")
	t.Logf("qualified_contract=%s current_effective_contract=%s workspace_unchanged=true", revision.ID, next.ID)
	result := tools.Call(env.ctx, "artifact_submit", "t21c-stale-submit", []byte(`{}`))
	t.Logf("artifact_submit result=%+v", result)
	if result.Error != string(core.Denied) {
		t.Fatalf("POLICY_BINDING_FAILED: obsolete qualified checkpoint authorized current artifact: %+v", result)
	}
}
