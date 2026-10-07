// pattern: Imperative Shell
package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"polis/internal/core"
	"polis/internal/runner"
	"polis/internal/taskvalidation"
	"strconv"
)

type ToolResult struct {
	Receipt *Receipt `json:"receipt,omitempty"`
	Data    any      `json:"data,omitempty"`
	Error   string   `json:"error,omitempty"`
	Detail  string   `json:"detail,omitempty"`
}
type CheckRunner interface {
	Check(string, string) (runner.Report, error)
}

// EmployeeTools is constructed by the trusted adapter. No payload can select
// company, employee, attempt, epoch or task. callID comes from the native bridge.
type EmployeeTools struct {
	Kernel                             *Kernel
	Binding                            Binding
	Checker                            CheckRunner
	Phase                              string
	ReadOnly                           bool
	ProductSurface                     bool
	MissionChangeAssessmentSurface     bool
	CSVInputRangeSurface               bool
	EnvironmentStatusSurface           bool
	EnvironmentEnsureSurface           bool
	ReadOnlyJobsSurface                bool
	BorrowerLeaseSurface               bool
	BrowserRunSurface                  bool
	ResearchOperationSurface           bool
	DirectMessagingSurface             bool
	SharedArtifactSurface              bool
	WorkspaceTreeSurface               bool
	WorkspaceSnapshotRevocationSurface bool
	SkillLoadSurface                   bool
	SkillDirectorySurface              bool
	GuidanceSurface                    bool
	ControlledMCPSurface               bool
	ControlledStdioMCPEnabled          bool
	StreamableHTTPMCPEnabled           bool
}

func strictArgs(raw []byte, v any) error {
	return strictArgsLimit(raw, v, 8192)
}

func strictArgsLimit(raw []byte, v any, maxBytes int) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return core.Malformed
	}
	if maxBytes <= 0 || len(raw) > maxBytes {
		return core.TooLarge
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e := dec.Decode(v); e != nil {
		return core.Malformed
	}
	if e := dec.Decode(new(any)); e != io.EOF {
		return core.Malformed
	}
	return nil
}
func (t EmployeeTools) Call(ctx context.Context, name, callID string, raw []byte) ToolResult {
	if len(callID) == 0 || len(callID) > 256 {
		return ToolResult{Error: string(core.Malformed)}
	}
	key := "tool-" + fingerprint([]string{t.Binding.session, callID})[:60]
	budget, budgetErr := t.Kernel.TXConsumeToolCall(ctx, t.Binding, key+"-budget", name)
	if budgetErr != nil {
		if errors.Is(budgetErr, core.StaleEpoch) || errors.Is(budgetErr, core.Denied) {
			_ = t.Kernel.RecordHistorical(ctx, t.Binding, name, raw, budgetErr.Error())
		}
		var known core.Code
		if errors.As(budgetErr, &known) {
			return ToolResult{Data: budget, Error: known.Error()}
		}
		return ToolResult{Data: budget, Error: "OUTCOME_UNKNOWN", Detail: budgetErr.Error()}
	}
	result, e := t.call(ctx, name, key, raw)
	if e != nil {
		if errors.Is(e, core.StaleEpoch) || errors.Is(e, core.Denied) {
			_ = t.Kernel.RecordHistorical(ctx, t.Binding, name, raw, e.Error())
		}
		var policyErr peerToolError
		if errors.As(e, &policyErr) {
			return ToolResult{Data: policyErr.Rejection, Error: policyErr.Code.Error()}
		}
		var known core.Code
		if errors.As(e, &known) {
			return ToolResult{Error: known.Error()}
		}
		var pathErr *PaginationRuntimePathError
		if errors.As(e, &pathErr) {
			return ToolResult{Error: "INFRASTRUCTURE_FAILED", Detail: e.Error()}
		}
		return ToolResult{Error: "OUTCOME_UNKNOWN", Detail: e.Error()}
	}
	return result
}
func (t EmployeeTools) call(ctx context.Context, name, key string, raw []byte) (ToolResult, error) {
	k, b := t.Kernel, t.Binding
	switch name {
	case "environment_ensure":
		if !t.ProductSurface || !t.EnvironmentEnsureSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			RevisionID string `json:"revision_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		requestID := "env-ensure-" + fingerprint([]string{b.SessionID(), key, args.RevisionID})[:48]
		run, e := k.ProductTaskEnvironmentEnsure(ctx, b, args.RevisionID, requestID)
		return ToolResult{Data: run}, e
	case "jobs_status":
		if !t.ProductSurface || !t.ReadOnlyJobsSurface {
			return ToolResult{}, core.Denied
		}
		var args struct {
			JobID string `json:"job_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		status, e := k.ProductTaskJobStatus(ctx, b, args.JobID)
		return ToolResult{Data: status}, e
	case "jobs_logs":
		if !t.ProductSurface || !t.ReadOnlyJobsSurface {
			return ToolResult{}, core.Denied
		}
		var args struct {
			JobID string `json:"job_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		logs, e := k.ProductTaskJobLogs(ctx, b, args.JobID)
		return ToolResult{Data: logs}, e
	case "jobs_borrow":
		if !t.ProductSurface || !t.BorrowerLeaseSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			JobID      string `json:"job_id"`
			Generation int    `json:"generation"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		requestID := "job-borrow-" + fingerprint([]string{b.SessionID(), key, args.JobID, strconv.Itoa(args.Generation)})[:48]
		lease, e := k.TXAcquireServiceBorrowerLease(ctx, b, ServiceBorrowerLeaseAcquireInput{JobID: args.JobID, Generation: args.Generation}, requestID)
		return ToolResult{Data: lease}, e
	case "jobs_touch":
		if !t.ProductSurface || !t.BorrowerLeaseSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			LeaseID string `json:"lease_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		requestID := "job-touch-" + fingerprint([]string{b.SessionID(), key, args.LeaseID})[:48]
		lease, e := k.TXTouchServiceBorrowerLease(ctx, b, args.LeaseID, requestID)
		return ToolResult{Data: lease}, e
	case "jobs_release":
		if !t.ProductSurface || !t.BorrowerLeaseSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			LeaseID string `json:"lease_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		requestID := "job-release-" + fingerprint([]string{b.SessionID(), key, args.LeaseID})[:48]
		lease, e := k.TXReleaseServiceBorrowerLease(ctx, b, args.LeaseID, requestID)
		return ToolResult{Data: lease}, e
	case "browser_run":
		if !t.ProductSurface || !t.BrowserRunSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ServiceJobID         string `json:"service_job_id"`
			ServiceGeneration    int    `json:"service_generation"`
			TargetOrigin         string `json:"target_origin"`
			PlanSHA256           string `json:"plan_sha256"`
			BrowserBuild         string `json:"browser_build"`
			ExecutionEnvironment string `json:"execution_environment"`
			InputRevision        string `json:"input_revision"`
			ViewportWidth        int    `json:"viewport_width"`
			ViewportHeight       int    `json:"viewport_height"`
			Locale               string `json:"locale"`
			Timezone             string `json:"timezone"`
		}
		if e := strictArgsLimit(raw, &args, 4096); e != nil {
			return ToolResult{}, e
		}
		input := BrowserRunRequest{ServiceJobID: args.ServiceJobID, ServiceGeneration: args.ServiceGeneration, TargetOrigin: args.TargetOrigin, PlanSHA256: args.PlanSHA256, BrowserBuild: args.BrowserBuild, ExecutionEnvironment: args.ExecutionEnvironment, InputRevision: args.InputRevision, ViewportWidth: args.ViewportWidth, ViewportHeight: args.ViewportHeight, Locale: args.Locale, Timezone: args.Timezone}
		requestID := "browser-run-" + fingerprint(struct {
			Session string
			Input   BrowserRunRequest
		}{b.SessionID(), input})[:48]
		run, e := k.TXRequestBrowserRun(ctx, b, input, requestID)
		return ToolResult{Data: run}, e
	case "browser_results":
		if !t.ProductSurface || !t.BrowserRunSurface {
			return ToolResult{}, core.Denied
		}
		var args struct {
			RunID string `json:"run_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		run, e := k.ProductTaskBrowserRunResults(ctx, b, args.RunID)
		return ToolResult{Data: run}, e
	case "research_search":
		if !t.ProductSurface || !t.ResearchOperationSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			SourceID string `json:"source_id"`
			Query    string `json:"query"`
		}
		if e := strictArgsLimit(raw, &args, 4096); e != nil {
			return ToolResult{}, e
		}
		requestID := "research-search-" + fingerprint([]string{b.SessionID(), key, args.SourceID, args.Query})[:48]
		record, e := k.TXRequestResearchOperation(ctx, b, ResearchOperationRequest{Kind: ResearchOperationKindSearch, SourceID: args.SourceID, Query: args.Query}, requestID)
		return ToolResult{Data: record}, e
	case "research_fetch":
		if !t.ProductSurface || !t.ResearchOperationSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			SourceID  string `json:"source_id"`
			TargetURL string `json:"target_url"`
		}
		if e := strictArgsLimit(raw, &args, 4096); e != nil {
			return ToolResult{}, e
		}
		requestID := "research-fetch-" + fingerprint([]string{b.SessionID(), key, args.SourceID, args.TargetURL})[:48]
		record, e := k.TXRequestResearchOperation(ctx, b, ResearchOperationRequest{Kind: ResearchOperationKindFetch, SourceID: args.SourceID, TargetURL: args.TargetURL}, requestID)
		return ToolResult{Data: record}, e
	case "environment_status":
		if !t.ProductSurface || !t.EnvironmentStatusSurface {
			return ToolResult{}, core.Denied
		}
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		status, e := k.ProductTaskEnvironmentStatus(ctx, b)
		return ToolResult{Data: status}, e
	case "csv_read_range":
		if !t.ProductSurface || !t.CSVInputRangeSurface {
			return ToolResult{}, core.Denied
		}
		var args struct {
			InputID        string `json:"input_id"`
			Revision       int64  `json:"revision"`
			SourceSHA256   string `json:"source_sha256"`
			ManifestSHA256 string `json:"manifest_sha256"`
			StartRow       int    `json:"start_row"`
			MaxRows        int    `json:"max_rows"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		rows, e := k.ReadProductTaskCSVRange(ctx, b, args.InputID, args.Revision, args.SourceSHA256, args.ManifestSHA256, args.StartRow, args.MaxRows)
		return ToolResult{Data: rows}, e
	case "mission_change_request_read":
		if !t.ProductSurface || !t.MissionChangeAssessmentSurface {
			return ToolResult{}, core.Denied
		}
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		planningContext, e := k.MissionChangePlanningContext(ctx, b)
		return ToolResult{Data: planningContext}, e
	case "mission_change_impact_assess":
		if !t.ProductSurface || !t.MissionChangeAssessmentSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args MissionChangePlanningAssessmentInput
		if e := strictArgsLimit(raw, &args, maxMissionChangeAssessmentBytes); e != nil {
			return ToolResult{}, e
		}
		receipt, e := k.TXAssessMissionChangeRequest(ctx, b, args, key)
		return ToolResult{Receipt: &receipt}, e
	case "work_current", "context_read", "workspace_read":
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		var h HandoverBundle
		var e error
		// Optional projections use separate read APIs; do not return them as one
		// handover if a Company write committed between those reads.
		for attempt := 0; attempt < 3; attempt++ {
			h, e = k.Handover(ctx, b)
			if e == nil && name == "work_current" && t.ProductSurface && t.DirectMessagingSurface {
				h.DirectMessageTargets, h.DirectMessageTargetsTruncated, e = k.ProductDirectMessageTargets(ctx, b)
			}
			if e == nil && name != "workspace_read" && t.SharedArtifactSurface {
				h.SharedArtifactReads, h.SharedArtifactReadsTruncated, e = k.SharedMissionArtifactReadHistory(ctx, b)
			}
			if e == nil && t.ControlledMCPSurface && name != "workspace_read" {
				h.MCPToolSets, e = k.BoundMCPToolSets(ctx, b, t.ControlledStdioMCPEnabled, t.StreamableHTTPMCPEnabled)
			}
			if e != nil {
				return ToolResult{}, e
			}
			var currentCompanySeq int64
			if e = k.pool.QueryRow(ctx, "SELECT company_seq FROM companies WHERE id=$1", b.scope.company).Scan(&currentCompanySeq); e != nil {
				return ToolResult{}, e
			}
			if currentCompanySeq == h.CompanySeq {
				break
			}
			if attempt == 2 {
				return ToolResult{Error: core.Conflict.Error(), Detail: "Company state changed while assembling the handover snapshot; read current work again."}, nil
			}
		}
		if !t.SkillLoadSurface {
			h.SkillCatalog = nil
			h.SkillCatalogTruncated = false
			h.SkillLoads = nil
			h.SkillLoadsTruncated = false
		}
		if t.SkillDirectorySurface {
			for index := range h.SkillCatalog {
				h.SkillCatalog[index].References = nil
			}
		}
		if name == "workspace_read" {
			return ToolResult{Data: h.Workspace}, e
		}
		return ToolResult{Data: h}, e
	case "mission_artifacts_list":
		if !t.ProductSurface || !t.SharedArtifactSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			AfterArtifactID string `json:"after_artifact_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		page, e := k.ListSharedMissionArtifacts(ctx, b, args.AfterArtifactID)
		return ToolResult{Data: page}, e
	case "workspace_files_list":
		if !t.ProductSurface || !t.WorkspaceTreeSurface {
			return ToolResult{}, core.Denied
		}
		var args struct {
			AfterCursor string `json:"after_cursor"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		page, e := k.ListProductWorkspaceTree(ctx, b, args.AfterCursor)
		return ToolResult{Data: page}, e
	case "workspace_files_search":
		if !t.ProductSurface || !t.WorkspaceTreeSurface {
			return ToolResult{}, core.Denied
		}
		var args struct {
			Query       string `json:"query"`
			AfterCursor string `json:"after_cursor"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		page, e := k.SearchProductWorkspaceTree(ctx, b, args.Query, args.AfterCursor)
		return ToolResult{Data: page}, e
	case "workspace_file_read":
		if !t.ProductSurface || !t.WorkspaceTreeSurface {
			return ToolResult{}, core.Denied
		}
		var args struct {
			RelativePath string `json:"relative_path"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		file, e := k.ReadProductWorkspaceFile(ctx, b, args.RelativePath)
		return ToolResult{Data: file}, e
	case "workspace_file_write":
		if !t.ProductSurface || !t.WorkspaceTreeSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ExpectedRevision int64  `json:"expected_revision"`
			RelativePath     string `json:"relative_path"`
			Content          string `json:"content"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		receipt, e := k.WriteProductWorkspaceFile(ctx, b, key, args.RelativePath, args.ExpectedRevision, args.Content)
		return ToolResult{Receipt: &receipt}, e
	case "workspace_file_delete":
		if !t.ProductSurface || !t.WorkspaceTreeSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ExpectedRevision int64  `json:"expected_revision"`
			RelativePath     string `json:"relative_path"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		receipt, e := k.DeleteProductWorkspaceFile(ctx, b, key, args.RelativePath, args.ExpectedRevision)
		return ToolResult{Receipt: &receipt}, e
	case "workspace_snapshot":
		if !t.ProductSurface || !t.WorkspaceTreeSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ExpectedRevision int64 `json:"expected_revision"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		receipt, snapshot, e := k.CreateProductWorkspaceSnapshot(ctx, b, key, args.ExpectedRevision)
		return ToolResult{Receipt: &receipt, Data: snapshot}, e
	case "workspace_snapshot_revoke":
		if !t.ProductSurface || !t.WorkspaceTreeSurface || !t.WorkspaceSnapshotRevocationSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ArtifactID string `json:"artifact_id"`
			Reason     string `json:"reason"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		receipt, e := k.RevokeProductWorkspaceSnapshot(ctx, b, key, args.ArtifactID, args.Reason)
		return ToolResult{Receipt: &receipt}, e
	case "workspace_snapshot_read":
		if !t.ProductSurface || !t.WorkspaceTreeSurface {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ArtifactID string `json:"artifact_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		snapshot, e := k.ReadProductWorkspaceSnapshot(ctx, b, args.ArtifactID)
		return ToolResult{Data: snapshot}, e
	case "workspace_snapshot_file_read":
		if !t.ProductSurface || !t.WorkspaceTreeSurface {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ArtifactID   string `json:"artifact_id"`
			RelativePath string `json:"relative_path"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		file, e := k.ReadProductWorkspaceSnapshotFile(ctx, b, args.ArtifactID, args.RelativePath)
		return ToolResult{Data: file}, e
	case "mission_artifact_read":
		if !t.ProductSurface || !t.SharedArtifactSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ArtifactID string `json:"artifact_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		receipt, document, e := k.TXReadSharedMissionArtifact(ctx, b, key, args.ArtifactID)
		return ToolResult{Receipt: &receipt, Data: document}, e
	case "collab_send":
		if !t.ProductSurface || !t.DirectMessagingSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ToEmployeeID string `json:"to_employee_id"`
			ToTaskID     string `json:"to_task_id"`
			Body         string `json:"body"`
			Actionable   *bool  `json:"actionable"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		if args.Actionable == nil {
			return ToolResult{}, core.Malformed
		}
		message, e := k.TXProductDirectMessage(ctx, b, ProductDirectMessageInput{
			ToEmployeeID: args.ToEmployeeID, ToTaskID: args.ToTaskID, Body: args.Body, Actionable: *args.Actionable,
		}, key)
		return ToolResult{Data: message}, e
	case "collab_inbox":
		if !t.ProductSurface || !t.DirectMessagingSurface {
			return ToolResult{}, core.Denied
		}
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		inbox, e := k.ProductDirectInbox(ctx, b)
		return ToolResult{Data: inbox}, e
	case "collab_ack":
		if !t.ProductSurface || !t.DirectMessagingSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			MessageID string `json:"message_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		return ToolResult{}, k.TXProductDirectMessageAck(ctx, b, args.MessageID, key)
	case "collab_apply":
		if !t.ProductSurface || !t.DirectMessagingSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args ProductDirectApplyRequest
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		receipt, e := k.TXProductDirectApply(ctx, b, args, key)
		return ToolResult{Receipt: &receipt}, e
	case "obligation_resolve":
		if !t.ProductSurface || !t.DirectMessagingSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ObligationID string `json:"obligation_id"`
			ArtifactID   string `json:"artifact_id"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		return ToolResult{}, k.TXProductDirectResolve(ctx, b, args.ObligationID, args.ArtifactID, key)
	case "guidance_read":
		if !t.ProductSurface || !t.GuidanceSurface {
			return ToolResult{}, core.Denied
		}
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		inbox, e := k.ReadPendingOperatorInstructions(ctx, b)
		return ToolResult{Data: inbox}, e
	case "guidance_respond":
		if !t.ProductSurface || !t.GuidanceSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			InstructionID string                     `json:"instruction_id"`
			Outcome       OperatorInstructionOutcome `json:"outcome"`
			Summary       string                     `json:"summary"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		receipt, e := k.TXRespondToOperatorInstruction(ctx, b, OperatorInstructionResponseInput{InstructionID: args.InstructionID, Outcome: args.Outcome, Summary: args.Summary}, key)
		return ToolResult{Receipt: &receipt, Data: receipt}, e
	case "skills_load":
		if !t.ProductSurface || !t.SkillLoadSurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			SkillID      string `json:"skill_id"`
			RelativePath string `json:"relative_path"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		receipt, document, e := k.TXLoadBoundReadOnlySkill(ctx, b, key, SkillLoadRequest{SkillID: args.SkillID, RelativePath: args.RelativePath})
		return ToolResult{Receipt: &receipt, Data: document}, e
	case "skills_list":
		if !t.ProductSurface || !t.SkillLoadSurface || !t.SkillDirectorySurface || t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			SkillID           string `json:"skill_id"`
			AfterRelativePath string `json:"after_relative_path"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		page, e := k.ListBoundReadOnlySkillFiles(ctx, b, args.SkillID, args.AfterRelativePath)
		return ToolResult{Data: page}, e
	case "workspace_replace":
		if t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct {
			ExpectedDigest   string `json:"expected_digest"`
			ExpectedRevision int64  `json:"expected_revision"`
			Content          string `json:"content"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		var r Receipt
		var e error
		if t.ProductSurface {
			r, e = k.TXReplaceAtRevision(ctx, b, key, args.ExpectedDigest, args.ExpectedRevision, args.Content)
		} else {
			r, e = k.TXReplace(ctx, b, key, args.ExpectedDigest, args.Content)
		}
		return ToolResult{Receipt: &r}, e
	case "workspace_check":
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		if t.ProductSurface {
			r, report, e := k.TXProductWorkspaceCheck(ctx, b, key)
			return ToolResult{Receipt: &r, Data: report}, e
		}
		w, e := k.Workspace(ctx, b)
		if e != nil {
			return ToolResult{}, e
		}
		report, e := t.Checker.Check(w.Content, t.Phase)
		if e != nil {
			return ToolResult{}, e
		}
		r, e := k.TXWrite(ctx, b.scope, &b, key, "workspace.check", w.Digest, func(tx pgx.Tx) (Receipt, error) {
			id := newID()
			body, e := json.Marshal(report)
			if e != nil {
				return Receipt{}, e
			}
			_, e = tx.Exec(ctx, "INSERT INTO worker_checks VALUES($1,$2,$3,$4,$5,$6,$7)", b.scope.company, id, b.session, w.Digest, t.Phase, report.Passed, body)
			return Receipt{ID: id, Status: "persisted"}, e
		})
		return ToolResult{Receipt: &r, Data: report}, e
	case "work_checkpoint":
		if t.ProductSurface {
			checkpoint, rejection, code := parseProductCheckpointRequest(raw)
			if code != "" {
				return ToolResult{Data: rejection, Error: code.Error()}, nil
			}
			r, e := k.TXProductCheckpoint(ctx, b, key, checkpoint)
			if e != nil {
				var policyErr peerToolError
				if errors.As(e, &policyErr) {
					return ToolResult{Data: policyErr.Rejection, Error: policyErr.Code.Error()}, nil
				}
				if rejection, code := productCheckpointCodeRejection(e); code != "" {
					return ToolResult{Data: rejection, Error: code.Error()}, nil
				}
			}
			return ToolResult{Receipt: &r}, e
		}
		var args Checkpoint
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		if args.Summary == "" || len(args.Facts) == 0 || len(args.Decisions) == 0 || len(args.Rejected) == 0 || len(args.EvidenceRefs) == 0 {
			return ToolResult{}, core.Malformed
		}
		var r Receipt
		var e error
		if t.ProductSurface {
			r, e = k.TXProductCheckpoint(ctx, b, key, args)
		} else {
			r, e = k.TXCheckpoint(ctx, b, key, args)
		}
		return ToolResult{Receipt: &r}, e
	case "task_submit":
		if !t.ProductSurface {
			return ToolResult{}, core.Denied
		}
		if t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		if rejection, code := ParseProductTaskDeliveryRequest(raw); code != "" {
			return ToolResult{Data: rejection, Error: code.Error()}, nil
		}
		h, e := k.Handover(ctx, b)
		if e != nil {
			return ToolResult{}, e
		}
		r, delivery, e := k.TXSubmitTaskDelivery(ctx, b, h.Task, key, []byte(h.Workspace.Content))
		if e != nil {
			if rejection, code := productDeliveryCodeRejection(e); code != "" {
				return ToolResult{Data: rejection, Error: code.Error()}, nil
			}
		}
		return ToolResult{Receipt: &r, Data: delivery}, e
	case "artifact_submit":
		if t.ProductSurface {
			if t.ReadOnly {
				return ToolResult{}, core.Denied
			}
			if rejection, code := ParseProductTaskDeliveryRequest(raw); code != "" {
				return ToolResult{Data: rejection, Error: code.Error()}, nil
			}
			h, e := k.Handover(ctx, b)
			if e != nil {
				return ToolResult{}, e
			}
			r, delivery, e := k.TXSubmitTaskDelivery(ctx, b, h.Task, key, []byte(h.Workspace.Content))
			if e != nil {
				if rejection, code := productDeliveryCodeRejection(e); code != "" {
					return ToolResult{Data: rejection, Error: code.Error()}, nil
				}
			}
			return ToolResult{Receipt: &r, Data: delivery}, e
		}
		if t.ReadOnly {
			return ToolResult{}, core.Denied
		}
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		h, e := k.Handover(ctx, b)
		if e != nil {
			return ToolResult{}, e
		}
		qualified := HasQualifiedCheckpoint(h.Checkpoints, h.Workspace)
		if t.ProductSurface {
			bindingDigest := ""
			if h.Task.ValidationBinding != nil {
				bindingDigest = h.Task.ValidationBinding.ConfigurationDigest
			}
			qualified = HasQualifiedCheckpointForBinding(h.Checkpoints, h.Workspace, bindingDigest)
		}
		if !qualified {
			return ToolResult{}, core.Denied
		}
		var r Receipt
		if t.ProductSurface {
			r, e = k.TXSubmitProduct(ctx, b, h.Task, key, []byte(h.Workspace.Content))
		} else {
			r, e = k.TXSubmit(ctx, b, h.Task, key, []byte(h.Workspace.Content))
		}
		return ToolResult{Receipt: &r}, e
	default:
		return ToolResult{}, core.Denied
	}
}

func productCheckpointCodeRejection(err error) (PeerToolRejection, core.Code) {
	rejection := newCheckpointRejection("checkpoint_not_allowed_in_task_state", "the current authorized Task/session cannot create this checkpoint.")
	switch {
	case errors.Is(err, core.StaleEpoch):
		rejection.ReasonCode = "writer_fenced"
		rejection.ActionableSummary = "the worker session is fenced; use the current authorized writer session."
		rejection.FailingField = "session"
		rejection.ExpectedPublicShape = "active current worker session"
		rejection.ActualCategory = "stale_or_fenced_writer"
	case errors.Is(err, core.Conflict):
		rejection.ReasonCode = "stale_workspace_revision"
		rejection.ActionableSummary = "re-read the current workspace and validation receipt before checkpointing."
		rejection.FailingField = "workspace_revision"
		rejection.ExpectedPublicShape = "current workspace revision and digest"
		rejection.ActualCategory = "stale_workspace"
	case errors.Is(err, core.Denied):
		rejection.FailingField = "task_state"
		rejection.ExpectedPublicShape = "authorized compat Task in working state with an active session"
		rejection.ActualCategory = "checkpoint_not_allowed"
	default:
		return PeerToolRejection{}, ""
	}
	return rejection, core.Denied
}

func (k *Kernel) TXCheckpoint(ctx context.Context, b Binding, key string, c Checkpoint) (Receipt, error) {
	return k.txCheckpoint(ctx, b, key, c, false)
}

func (k *Kernel) txCheckpoint(ctx context.Context, b Binding, key string, c Checkpoint, product bool) (Receipt, error) {
	if c.Kind == "" {
		c.Kind = CheckpointQualified
	}
	if c.Kind != CheckpointProgress && c.Kind != CheckpointQualified {
		return Receipt{}, core.Malformed
	}
	if c.Kind == CheckpointProgress && c.NextAction == "" {
		return Receipt{}, core.Malformed
	}
	if rejection, code := validateEvidenceRefs(c.EvidenceRefs); code != "" {
		return Receipt{}, peerToolError{Code: code, Rejection: rejection}
	}
	return k.TXWrite(ctx, b.scope, &b, key, "work.checkpoint", c, func(tx pgx.Tx) (Receipt, error) {
		var digest string
		var workspaceRevision, epoch int64
		var task, mission string
		var taskKind core.TaskKind
		e := tx.QueryRow(ctx, "SELECT s.task_id,t.mission_id,t.kind,w.digest,w.revision,s.epoch FROM worker_workspaces w JOIN worker_sessions s ON w.company_id=s.company_id AND w.task_id=s.task_id JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id WHERE s.company_id=$1 AND s.id=$2", b.scope.company, b.session).Scan(&task, &mission, &taskKind, &digest, &workspaceRevision, &epoch)
		if e != nil {
			return Receipt{}, e
		}
		bindingDigest, runnerRevision := "", ""
		if product {
			if taskKind != core.TaskKindCompat {
				return Receipt{}, core.Denied
			}
			currentTaskID, stateErr := k.requireProductTaskWorking(ctx, tx, b)
			if stateErr != nil {
				return Receipt{}, stateErr
			}
			if currentTaskID != task {
				return Receipt{}, core.Denied
			}
			if c.Kind == CheckpointQualified {
				if stateErr = k.requireMemoryTaskCleanTX(ctx, tx, b.scope.company, currentTaskID); stateErr != nil {
					return Receipt{}, stateErr
				}
			}
			e = tx.QueryRow(ctx, "SELECT configuration_digest,runner_revision FROM task_validation_bindings WHERE company_id=$1 AND task_id=$2", b.scope.company, task).Scan(&bindingDigest, &runnerRevision)
			if errors.Is(e, pgx.ErrNoRows) {
				if c.Kind == CheckpointQualified {
					return Receipt{}, peerCheckpointDenied("validation_not_configured", "a Task without TaskValidationBinding may be explored but cannot be qualified.", "", workspaceRevision)
				}
				bindingDigest, runnerRevision = "", ""
			} else if e != nil {
				return Receipt{}, e
			}
		}
		contractID := ""
		var hasContracts bool
		if e = tx.QueryRow(ctx, "SELECT to_regclass('public.contract_revisions') IS NOT NULL").Scan(&hasContracts); e != nil {
			return Receipt{}, e
		}
		if hasContracts {
			e = tx.QueryRow(ctx, "SELECT id FROM contract_revisions WHERE company_id=$1 AND mission_id=$2 AND state='accepted' ORDER BY revision DESC LIMIT 1", b.scope.company, mission).Scan(&contractID)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return Receipt{}, e
			}
		}
		if c.ContractRevisionID != "" && c.ContractRevisionID != contractID {
			return Receipt{}, core.Conflict
		}
		for _, id := range c.EvidenceRefs {
			var passed bool
			var checkRaw []byte
			e = tx.QueryRow(ctx, "SELECT passed,report FROM worker_checks WHERE company_id=$1 AND id=$2 AND session_id=$3 AND digest=$4", b.scope.company, id, b.session, digest).Scan(&passed, &checkRaw)
			if errors.Is(e, pgx.ErrNoRows) {
				if c.Kind == CheckpointQualified {
					return Receipt{}, peerCheckpointDenied("qualified_checkpoint_requires_check_receipt", "a qualified checkpoint must reference a current workspace_check receipt.", contractID, workspaceRevision)
				}
				_, valid, refErr := peerEvidenceRefKind(ctx, tx, b, id, digest)
				if refErr != nil {
					return Receipt{}, refErr
				}
				if !valid {
					return Receipt{}, peerCheckpointDenied("evidence_ref_invalid", "each evidence_refs[] value must be a persisted receipt for this worker and workspace.", contractID, workspaceRevision)
				}
				continue
			}
			if e != nil {
				return Receipt{}, e
			}
			if c.Kind == CheckpointQualified && !passed {
				return Receipt{}, peerCheckpointDenied("qualified_check_not_passed", "a qualified checkpoint requires a passed workspace_check receipt.", contractID, workspaceRevision)
			}
			if product && c.Kind == CheckpointQualified {
				var check taskvalidation.Result
				if json.Unmarshal(checkRaw, &check) != nil || check.Status != taskvalidation.StatusPass || check.TaskID != task || check.MissionID != mission || check.ConfigurationDigest != bindingDigest || check.WorkspaceDigest != digest || check.WorkspaceRevision != workspaceRevision || check.SessionID != b.session || check.Epoch != epoch {
					return Receipt{}, peerCheckpointDenied("qualified_check_stale", "a qualified product checkpoint requires a passing receipt for the current TaskValidationBinding, session and workspace revision.", contractID, workspaceRevision)
				}
			}
			if c.Kind == CheckpointQualified && contractID != "" {
				var checked PeerCheckReceipt
				if json.Unmarshal(checkRaw, &checked) != nil || checked.RelevantContractRevision != contractID || checked.RelevantWorkspaceRevision != workspaceRevision || checked.AcceptanceCheckerRevision != core.PeerAcceptanceCheckerRevision {
					return Receipt{}, peerCheckpointDenied("qualified_check_stale", "the qualified checkpoint evidence must reference the current contract, workspace revision and checker revision.", contractID, workspaceRevision)
				}
			}
		}
		if c.Kind == CheckpointQualified && len(c.EvidenceRefs) == 0 {
			return Receipt{}, peerCheckpointDenied("missing_evidence_refs", "a qualified checkpoint requires at least one workspace_check receipt.", contractID, workspaceRevision)
		}
		c.WorkspaceDigest = digest
		c.WorkspaceRevision = workspaceRevision
		c.ContractRevisionID = contractID
		if product {
			c.TaskValidationBindingDigest = bindingDigest
			if c.Kind == CheckpointQualified {
				c.AcceptanceCheckerRevision = runnerRevision
				c.ValidationStatus = string(taskvalidation.StatusPass)
			}
		} else {
			c.AcceptanceCheckerRevision = core.PeerAcceptanceCheckerRevision
		}
		c.CheckpointPolicyRevision = core.CheckpointPolicyRevision
		c.ArtifactEligibilityPolicyRevision = core.ArtifactEligibilityPolicyRevision
		c.ContractSupersessionPolicyRevision = core.PeerContractSupersessionPolicyRevision
		c.FinalizationState = "not_qualified"
		if c.Kind == CheckpointQualified {
			c.FinalizationState = "current"
		}
		c.SessionID = b.session
		c.Epoch = epoch
		if hasContracts {
			_ = tx.QueryRow(ctx, `SELECT COALESCE(m.id,''),COALESCE(m.delivery_state,''),o.id,o.state
FROM obligations o
LEFT JOIN messages m ON m.company_id=o.company_id AND m.id=o.id
WHERE o.company_id=$1 AND o.state IN ('pending','observed','applied')
  AND (o.task_id=$2 OR (m.sender=$3 AND m.mission_id=$4))
			ORDER BY o.id LIMIT 1`, b.scope.company, task, b.employee, mission).Scan(&c.PendingMessageID, &c.PendingMessageState, &c.PendingObligationID, &c.PendingObligationState)
		} else {
			_ = tx.QueryRow(ctx, `SELECT COALESCE(m.id,''),o.id,o.state
FROM obligations o
LEFT JOIN messages m ON m.company_id=o.company_id AND m.id=o.id
WHERE o.company_id=$1 AND o.state IN ('pending','observed','applied')
  AND (o.task_id=$2 OR (m.sender=$3 AND m.mission_id=$4))
ORDER BY o.id LIMIT 1`, b.scope.company, task, b.employee, mission).Scan(&c.PendingMessageID, &c.PendingObligationID, &c.PendingObligationState)
		}
		raw, e := json.Marshal(c)
		if e != nil {
			return Receipt{}, e
		}
		if len(raw) > 4096 {
			return Receipt{}, core.TooLarge
		}
		id := newID()
		_, e = tx.Exec(ctx, "INSERT INTO worker_checkpoints VALUES($1,$2,$3,$4,$5)", b.scope.company, id, b.session, digest, raw)
		return Receipt{ID: id, Status: "persisted"}, e
	})
}
func (k *Kernel) RecordHistorical(ctx context.Context, b Binding, name string, raw []byte, reason string) error {
	if len(raw) > 8192 {
		return core.TooLarge
	}
	_, e := k.TXWrite(ctx, b.scope, nil, newID(), "worker.historical", b.session, func(tx pgx.Tx) (Receipt, error) {
		data, e := json.Marshal(struct {
			Name      string
			Arguments json.RawMessage
		}{name, raw})
		if e != nil {
			return Receipt{}, e
		}
		id := newID()
		_, e = tx.Exec(ctx, "INSERT INTO worker_observations VALUES($1,$2,$3,$4,$5)", b.scope.company, id, b.session, reason, data)
		return Receipt{ID: id, Status: "historical_only"}, e
	})
	return e
}

// VerifyProbe is a trusted control-side, non-author acceptance entry, never a
// dynamic tool. The supplied checker is frozen by the executable, not the model.
func (k *Kernel) VerifyProbe(ctx context.Context, s Scope, artifact, phase string, v CheckRunner) (runner.Report, error) {
	var digest, author, task string
	e := k.pool.QueryRow(ctx, "SELECT digest,author,task_id FROM artifacts WHERE company_id=$1 AND id=$2 AND artifact_kind='deliverable' AND state='ready' AND contract='signed-zero@1'", s.company, artifact).Scan(&digest, &author, &task)
	if e != nil {
		return runner.Report{}, e
	}
	if author == "emp-review" {
		return runner.Report{}, core.Denied
	}
	content, e := readBlob(k.root, s.company, digest)
	if e != nil {
		return runner.Report{}, e
	}
	report, e := v.Check(string(content), phase)
	if e != nil {
		return report, e
	}
	if report.Digest != digest {
		return report, core.Integrity
	}
	_, e = k.TXWrite(ctx, s, nil, newID(), "probe.independent_review", []string{artifact, digest, phase}, func(tx pgx.Tx) (Receipt, error) {
		verdict := "failed"
		if report.Passed {
			verdict = "passed"
		}
		_, e := tx.Exec(ctx, "UPDATE artifacts SET verifier='emp-review',verdict=$3 WHERE company_id=$1 AND id=$2 AND digest=$4", s.company, artifact, verdict, digest)
		if e != nil {
			return Receipt{}, e
		}
		if report.Passed {
			if e = k.requireMemoryTaskCleanTX(ctx, tx, s.company, task); e != nil {
				return Receipt{}, e
			}
			_, e = tx.Exec(ctx, "UPDATE tasks SET state='completed' WHERE company_id=$1 AND id=$2", s.company, task)
			if e != nil {
				return Receipt{}, e
			}
			_, e = tx.Exec(ctx, "UPDATE obligations SET state='fulfilled',evidence_id=$3 WHERE company_id=$1 AND task_id=$2", s.company, task, artifact)
			if e != nil {
				return Receipt{}, e
			}
		}
		return Receipt{ID: artifact, Status: verdict}, nil
	})
	if e != nil {
		return report, e
	}
	if !report.Passed {
		return report, fmt.Errorf("independent acceptance failed")
	}
	return report, nil
}
