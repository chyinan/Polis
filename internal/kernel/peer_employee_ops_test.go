// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"polis/internal/fixture"
	"polis/internal/runner"
	"testing"
)

func TestR03AT5BackendToolSurfaceInvokesEveryHandlerOnce(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "r03a-t5-tools-"+newID())
	must(t, err)
	fx, err := k.TXCreatePeerFixture(ctx, scope, "r03a-t5-tools")
	must(t, err)
	b, err := k.TXNewWorker(ctx, scope, fx.Backend.ID, "fake/backend")
	must(t, err)
	p, err := runner.Start(b.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, err)
	defer p.Stop()
	must(t, k.TXAttachWorker(ctx, b, p))
	must(t, k.TXValidateWorker(ctx, b))
	must(t, k.TXActivateWorker(ctx, b, "r03a-t5-capability"))
	tools := PeerEmployeeTools{Kernel: k, Binding: b, Role: "peer_backend"}
	called := map[string]bool{}
	call := func(name string, args any) ToolResult {
		raw, e := json.Marshal(args)
		must(t, e)
		result := tools.Call(ctx, name, "t5-"+name, raw)
		if result.Error != "" {
			t.Fatalf("%s returned %s: %s", name, result.Error, result.Detail)
		}
		called[name] = true
		return result
	}
	call("work_current", struct{}{})
	call("context_read", struct{}{})
	workspace := call("workspace_read", struct{}{}).Data.(Workspace)
	call("workspace_replace", map[string]any{"expected_digest": workspace.Digest, "content": fixture.PeerBackendV2})
	proposal := call("contract_propose", PeerContractProposal{Endpoint: "GET /items?cursor=...", Schema: `{"items":[],"next_cursor":""}`})
	var revision PeerContractRevision
	rawProposal, e := json.Marshal(proposal.Data)
	must(t, e)
	must(t, json.Unmarshal(rawProposal, &revision))
	call("contract_accept", map[string]string{"revision_id": revision.ID})
	call("contract_read", map[string]string{"revision_id": revision.ID})
	message := call("collab_send", map[string]any{"to_employee_id": "emp-frontend", "to_task_id": fx.Frontend.ID, "contract_revision_id": revision.ID, "body": "apply the accepted revision", "actionable": true})
	var sent PeerMessage
	rawMessage, e := json.Marshal(message.Data)
	must(t, e)
	must(t, json.Unmarshal(rawMessage, &sent))
	if sent.ID == "" || sent.ObligationID == "" {
		t.Fatal("collab.send did not return message and obligation identity")
	}
	check := call("workspace_check", struct{}{})
	if report, ok := check.Data.(PeerCheckReceipt); !ok || !report.Passed || check.Receipt == nil {
		t.Fatal("workspace_check did not return passed receipt")
	}
	call("work_checkpoint", Checkpoint{Summary: "backend candidate checked", Facts: []string{"v1 workspace updated"}, Decisions: []string{"accepted revision"}, Rejected: []string{"Planner relay"}, EvidenceRefs: []string{check.Receipt.ID}})
	call("artifact_submit", struct{}{})
	for _, name := range []string{"work_current", "context_read", "workspace_read", "workspace_replace", "contract_propose", "contract_accept", "contract_read", "collab_send", "workspace_check", "work_checkpoint", "artifact_submit"} {
		if !called[name] {
			t.Fatalf("registered backend tool was not invoked: %s", name)
		}
	}
}

func TestPeerWorkspaceCheckPersistsActionableBindingFailureReceipt(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "r03a-binding-feedback-"+newID())
	must(t, err)
	fx, err := k.TXCreatePeerFixture(ctx, scope, "r03a-binding-feedback")
	must(t, err)
	b, err := k.TXNewWorkerWithToolBudget(ctx, scope, fx.Backend.ID, "gpt-5.6-luna/medium", 48)
	must(t, err)
	p, err := runner.Start(b.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, err)
	defer p.Stop()
	must(t, k.TXAttachWorker(ctx, b, p))
	must(t, k.TXValidateWorker(ctx, b))
	must(t, k.TXActivateWorker(ctx, b, "binding-feedback-capability"))
	tools := PeerEmployeeTools{Kernel: k, Binding: b, Role: "peer_backend"}
	proposal := PeerContractProposal{Endpoint: "GET /items", Schema: fixture.PeerPaginationContractV3}
	currentHandover, err := k.Handover(ctx, b)
	must(t, err)
	proposed, err := k.TXProposePeerContract(ctx, b, currentHandover.Task, proposal, "binding-feedback-propose")
	must(t, err)
	must(t, k.TXAcceptPeerContract(ctx, b, proposed.ID, "binding-feedback-accept"))
	workspace := tools.Call(ctx, "workspace_read", "binding-feedback-read", []byte(`{}`))
	must(t, workspaceError(workspace))
	current := workspace.Data.(Workspace)
	wrongBinding := `package backend
const Endpoint = "GET /items"
func FetchPage(cursor string, limit int) string { return "{}" }
`
	replaced := tools.Call(ctx, "workspace_replace", "binding-feedback-replace", mustJSON(t, map[string]any{"expected_digest": current.Digest, "content": wrongBinding}))
	must(t, workspaceError(replaced))
	checked := tools.Call(ctx, "workspace_check", "binding-feedback-check", []byte(`{}`))
	if checked.Error != "" {
		t.Fatalf("binding failure returned transport-style error: %+v", checked)
	}
	if checked.Receipt == nil {
		t.Fatal("failed workspace check did not persist a check receipt")
	}
	report, ok := checked.Data.(PeerCheckReceipt)
	if !ok {
		t.Fatalf("workspace check returned unexpected data: %#v", checked.Data)
	}
	if report.Passed || len(report.Criteria) == 0 || report.Criteria[0].CriterionID != "public_binding_check" {
		t.Fatalf("binding failure was not a structured public criterion: %+v", report)
	}
	criterion := report.Criteria[0]
	if criterion.PublicReasonCode != "entrypoint_missing" || criterion.BindingContractRevision != fixture.BackendBindingContractRevisionV2 || criterion.CheckReceiptID != checked.Receipt.ID {
		t.Fatalf("binding feedback was not actionable and receipt-bound: %+v", criterion)
	}
	if criterion.ExpectedPublicBinding.Entrypoint != "FetchItems" || criterion.ActualPublicBinding.Entrypoint != "FetchPage" {
		t.Fatalf("expected/actual public binding missing: %+v", criterion)
	}
}

func workspaceError(result ToolResult) error {
	if result.Error != "" {
		return errors.New(result.Error + ": " + result.Detail)
	}
	return nil
}
