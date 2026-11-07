// pattern: Imperative Shell
package kernel

import (
	"encoding/json"
	"fmt"
	"polis/internal/core"
	"polis/internal/fixture"
	"strings"
	"testing"
)

func freshnessCheckpoint(t *testing.T, env peerHardeningEnv, kind, key string) Receipt {
	t.Helper()
	check, err := env.k.TXPeerCheck(env.ctx, env.backend, key+"-check")
	must(t, err)
	c := Checkpoint{Kind: kind, Summary: "checkpoint", Facts: []string{"workspace inspected"}, Decisions: []string{"retain progress"}, Rejected: []string{"unverified finalization"}, EvidenceRefs: []string{check.Receipt.ID}, NextAction: "continue"}
	r, err := env.k.TXCheckpoint(env.ctx, env.backend, key, c)
	must(t, err)
	return r
}
func freshnessReady(t *testing.T, env peerHardeningEnv) PeerContractRevision {
	t.Helper()
	rev := acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":""}`, "ready")
	w, err := env.k.Workspace(env.ctx, env.backend)
	must(t, err)
	_, err = env.k.TXReplace(env.ctx, env.backend, "ready-content", w.Digest, fixture.PeerBackendV2)
	must(t, err)
	return rev
}
func freshnessSubmit(t *testing.T, env peerHardeningEnv, key string) (Receipt, error) {
	t.Helper()
	h, err := env.k.Handover(env.ctx, env.backend)
	must(t, err)
	return env.k.TXSubmit(env.ctx, env.backend, h.Task, key, []byte(h.Workspace.Content))
}
func TestQualifiedCheckpointFreshness(t *testing.T) {
	for _, scenario := range []string{"current", "workspace", "checker", "policy", "contract-recovery", "progress-history", "old-check"} {
		t.Run(scenario, func(t *testing.T) {
			env := newPeerHardeningEnv(t, 48)
			old := freshnessReady(t, env)
			kind := CheckpointQualified
			if scenario == "progress-history" {
				kind = CheckpointProgress
			}
			cp := freshnessCheckpoint(t, env, kind, "checkpoint")
			switch scenario {
			case "policy":
				_, e := env.k.pool.Exec(env.ctx, `UPDATE worker_checkpoints SET data=jsonb_set(data,'{artifact_eligibility_policy_revision}','"previous-policy"') WHERE company_id=$1 AND id=$2`, env.scope.company, cp.ID)
				must(t, e)
			case "workspace":
				w, e := env.k.Workspace(env.ctx, env.backend)
				must(t, e)
				_, e = env.k.TXReplace(env.ctx, env.backend, "change", w.Digest, w.Content+"\n")
				must(t, e)
			case "checker":
				// Model a stored certificate produced by a prior checker deployment.
				_, e := env.k.pool.Exec(env.ctx, `UPDATE worker_checkpoints SET data=jsonb_set(data,'{acceptance_checker_revision}','"previous-checker"') WHERE company_id=$1 AND id=$2`, env.scope.company, cp.ID)
				must(t, e)
			case "contract-recovery", "progress-history", "old-check":
				acceptPeerRevision(t, env, "GET /items?cursor=...", `{"items":[],"next_cursor":"","total":0}`, "new")
			}
			r, e := freshnessSubmit(t, env, "final")
			if scenario == "current" {
				must(t, e)
				var binding string
				must(t, env.k.pool.QueryRow(env.ctx, "SELECT contract_revision_id FROM artifact_qualifications WHERE company_id=$1 AND artifact_id=$2", env.scope.company, r.ID).Scan(&binding))
				if binding != old.ID {
					t.Fatal("artifact lost commit qualification binding")
				}
				return
			}
			wantCode(t, e, core.Denied)
			var count int
			must(t, env.k.pool.QueryRow(env.ctx, "SELECT count(*) FROM artifacts WHERE company_id=$1", env.scope.company).Scan(&count))
			if count != 0 {
				t.Fatal("stale artifact persisted")
			}
			if scenario == "contract-recovery" || scenario == "progress-history" {
				h, e := env.k.Handover(env.ctx, env.backend)
				must(t, e)
				if len(h.Checkpoints) != 1 || h.Checkpoints[0].ContractRevisionID != old.ID {
					t.Fatal("historical checkpoint discarded or rebound")
				}
				if scenario == "progress-history" {
					if h.Checkpoints[0].Kind != CheckpointProgress {
						t.Fatal("progress kind changed")
					}
					return
				}
				if h.Checkpoints[0].FinalizationState != "superseded_for_finalization" {
					t.Fatal("old qualification still current")
				}
				failed, e := env.k.TXPeerCheck(env.ctx, env.backend, "new-fail")
				must(t, e)
				if failed.Passed {
					t.Fatal("old content passed stricter contract")
				}
				updated := strings.Replace(h.Workspace.Content, "items,next_cursor", "items,next_cursor,total", 1)
				_, e = env.k.TXReplace(env.ctx, env.backend, "fix", h.Workspace.Digest, updated)
				must(t, e)
				freshnessCheckpoint(t, env, CheckpointQualified, "new-checkpoint")
				_, e = freshnessSubmit(t, env, "new-final")
				must(t, e)
			}
			if scenario == "old-check" {
				var data []byte
				must(t, env.k.pool.QueryRow(env.ctx, "SELECT data FROM worker_checkpoints WHERE company_id=$1 AND id=$2", env.scope.company, cp.ID).Scan(&data))
				var c Checkpoint
				must(t, json.Unmarshal(data, &c))
				c.ContractRevisionID = "" // Worker cannot requalify an old check by omitting its contract.
				_, e = env.k.TXCheckpoint(env.ctx, env.backend, "reuse-old-check", c)
				wantCode(t, e, core.Denied)
			}
		})
	}
}

func TestCheckpointFinalizationConcurrentContractAcceptance(t *testing.T) {
	for i := 0; i < 8; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			env := newPeerHardeningEnv(t, 48)
			freshnessReady(t, env)
			freshnessCheckpoint(t, env, CheckpointQualified, "cp")
			h, e := env.k.Handover(env.ctx, env.backend)
			must(t, e)
			next, e := env.k.TXProposePeerContract(env.ctx, env.backend, h.Task, PeerContractProposal{Endpoint: "GET /items?cursor=...", Schema: `{"items":[],"next_cursor":"","total":0}`}, "proposal")
			must(t, e)
			start := make(chan struct{})
			accepted := make(chan error, 1)
			submitted := make(chan error, 1)
			go func() { <-start; accepted <- env.k.TXAcceptPeerContract(env.ctx, env.backend, next.ID, "accept") }()
			go func() {
				<-start
				_, err := env.k.TXSubmit(env.ctx, env.backend, h.Task, "submit", []byte(h.Workspace.Content))
				submitted <- err
			}()
			close(start)
			must(t, <-accepted)
			submitErr := <-submitted
			if submitErr != nil {
				wantCode(t, submitErr, core.Denied)
				return
			}
			var ordered bool
			must(t, env.k.pool.QueryRow(env.ctx, `SELECT
   (SELECT max(company_seq) FROM events WHERE company_id=$1 AND kind='artifact.submit') <
   (SELECT max(company_seq) FROM events WHERE company_id=$1 AND kind='contract.accept')`, env.scope.company).Scan(&ordered))
			if !ordered {
				t.Fatal("TOCTOU: old qualification artifact committed after newer contract")
			}
		})
	}
}
