// pattern: Functional Core
package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"

	"polis/internal/core"
	"polis/internal/taskvalidation"
)

const MissionChangeImpactSchema = "polis-mission-change-impact@1"
const MissionChangePlanningAssessmentSchema = "polis-mission-change-planning-assessment@1"
const maxMissionChangeImpactItems = 512
const maxMissionChangeWorkspaceBytes = 64 * 1024

type MissionChangeRequestInput struct {
	ChangeSummary              string                             `json:"changeSummary"`
	ProposedTitle              string                             `json:"proposedTitle,omitempty"`
	ProposedGoal               string                             `json:"proposedGoal,omitempty"`
	ProposedAcceptanceContract *taskvalidation.AcceptanceContract `json:"proposedAcceptanceContract,omitempty"`
	BlockPreviousResults       bool                               `json:"blockPreviousResults"`
}

type MissionChangeInputRevision struct {
	InputID       string `json:"inputId"`
	Revision      int64  `json:"revision"`
	ContentDigest string `json:"contentDigest"`
	State         string `json:"state"`
}

type MissionChangeTaskImpact struct {
	TaskID            string  `json:"taskId"`
	OwnerEmployeeID   string  `json:"ownerEmployeeId"`
	Kind              string  `json:"kind"`
	State             string  `json:"state"`
	Generation        int64   `json:"generation"`
	WorkspaceDigest   *string `json:"workspaceDigest"`
	WorkspaceRevision *int64  `json:"workspaceRevision"`
}

type MissionChangeArtifactImpact struct {
	ArtifactID string `json:"artifactId"`
	TaskID     string `json:"taskId"`
	Digest     string `json:"digest"`
	Verdict    string `json:"verdict"`
}

type MissionChangeWorkerImpact struct {
	SessionID  string `json:"sessionId"`
	TaskID     string `json:"taskId"`
	EmployeeID string `json:"employeeId"`
	State      string `json:"state"`
}

type MissionChangeJobImpact struct {
	JobID     string `json:"jobId"`
	TaskID    string `json:"taskId"`
	State     string `json:"state"`
	Readiness string `json:"readiness"`
}

type MissionChangeServiceEndpointImpact struct {
	JobID      string `json:"jobId"`
	Generation int64  `json:"generation"`
	Readiness  string `json:"readiness"`
}

type MissionChangeTakeoverLeaseImpact struct {
	LeaseID                string `json:"leaseId"`
	TaskID                 string `json:"taskId"`
	BaseRequirementsSHA256 string `json:"baseRequirementsSha256"`
	BaseWorkspaceDigest    string `json:"baseWorkspaceDigest"`
	BaseWorkspaceRevision  int64  `json:"baseWorkspaceRevision"`
}

type MissionChangeTakeoverSnapshotImpact struct {
	LeaseID            string `json:"leaseId"`
	TaskID             string `json:"taskId"`
	InputID            string `json:"inputId"`
	InputRevision      int64  `json:"inputRevision"`
	ContentDigest      string `json:"contentDigest"`
	ByteSize           int64  `json:"byteSize"`
	HumanEffortSeconds *int64 `json:"humanEffortSeconds"`
}

type MissionChangeImpact struct {
	SchemaVersion                  string                                `json:"schemaVersion"`
	MissionID                      string                                `json:"missionId"`
	BaseRequirementsSHA256         string                                `json:"baseRequirementsSha256"`
	InputRevisions                 []MissionChangeInputRevision          `json:"inputRevisions"`
	Tasks                          []MissionChangeTaskImpact             `json:"tasks"`
	Artifacts                      []MissionChangeArtifactImpact         `json:"artifacts"`
	ActiveWorkerSessions           []MissionChangeWorkerImpact           `json:"activeWorkerSessions"`
	NonterminalJobRuns             []MissionChangeJobImpact              `json:"nonterminalJobRuns"`
	ActiveServiceEndpoints         []MissionChangeServiceEndpointImpact  `json:"activeServiceEndpoints"`
	ActiveTaskTakeoverLeases       []MissionChangeTakeoverLeaseImpact    `json:"activeTaskTakeoverLeases"`
	ReturnedHumanTakeoverSnapshots []MissionChangeTakeoverSnapshotImpact `json:"returnedHumanTakeoverSnapshots"`
	NaturalLanguageImpactStatus    string                                `json:"naturalLanguageImpactStatus"`
}

type MissionChangePlanningAssessmentInput struct {
	ChangeRequestID     string   `json:"change_request_id"`
	AnalysisBasisSHA256 string   `json:"analysis_basis_sha256"`
	RiskLevel           string   `json:"risk_level"`
	Summary             string   `json:"summary"`
	AffectedTaskIDs     []string `json:"affected_task_ids"`
	UnaffectedTaskIDs   []string `json:"unaffected_task_ids"`
	UncertainTaskIDs    []string `json:"uncertain_task_ids"`
	Questions           []string `json:"questions"`
	RecommendedControls []string `json:"recommended_controls"`
}

type MissionChangePlanningAssessment struct {
	SchemaVersion       string   `json:"schemaVersion"`
	AssessmentID        string   `json:"assessmentId"`
	Revision            int64    `json:"revision"`
	Status              string   `json:"status"`
	AnalysisBasisSHA256 string   `json:"analysisBasisSha256"`
	AssessmentSHA256    string   `json:"assessmentSha256"`
	RiskLevel           string   `json:"riskLevel"`
	Summary             string   `json:"summary"`
	AffectedTaskIDs     []string `json:"affectedTaskIds"`
	UnaffectedTaskIDs   []string `json:"unaffectedTaskIds"`
	UncertainTaskIDs    []string `json:"uncertainTaskIds"`
	Questions           []string `json:"questions"`
	RecommendedControls []string `json:"recommendedControls"`
	WorkerSessionID     string   `json:"workerSessionId"`
	WorkerTaskID        string   `json:"workerTaskId"`
	WorkerEpoch         int64    `json:"workerEpoch"`
	CreatedAt           string   `json:"createdAt"`
}

type MissionChangePlanningContext struct {
	ChangeRequest       MissionChangeRequest `json:"changeRequest"`
	AnalysisBasisSHA256 string               `json:"analysisBasisSha256"`
}

type MissionChangeInputRevisionMap struct {
	Origin            string `json:"origin"`
	SourceTaskID      string `json:"sourceTaskId,omitempty"`
	PreviousInputID   string `json:"previousInputId"`
	PreviousRevision  int64  `json:"previousRevision"`
	SuccessorInputID  string `json:"successorInputId"`
	SuccessorRevision int64  `json:"successorRevision"`
	ContentDigest     string `json:"contentDigest"`
}

type MissionChangeRequestEvent struct {
	EventID                  string                          `json:"eventId"`
	State                    string                          `json:"state"`
	ImpactRevision           *int64                          `json:"impactRevision"`
	SuccessorMissionID       *string                         `json:"successorMissionId"`
	ReasonCode               string                          `json:"reasonCode"`
	CreatedAt                string                          `json:"createdAt"`
	InputRevisionMap         []MissionChangeInputRevisionMap `json:"inputRevisionMap"`
	PlanningAssessmentID     string                          `json:"planningAssessmentId,omitempty"`
	PlanningAssessmentSHA256 string                          `json:"planningAssessmentSha256,omitempty"`
	PlanningRiskLevel        string                          `json:"planningRiskLevel,omitempty"`
}

type MissionChangeRequest struct {
	ID                         string                             `json:"changeRequestId"`
	MissionID                  string                             `json:"missionId"`
	ClientRequestID            string                             `json:"clientRequestId"`
	BaseRequirementsSHA256     string                             `json:"baseRequirementsSha256"`
	ChangeSummary              string                             `json:"changeSummary"`
	ProposedTitle              string                             `json:"proposedTitle"`
	ProposedGoal               string                             `json:"proposedGoal"`
	ProposedAcceptanceContract *taskvalidation.AcceptanceContract `json:"proposedAcceptanceContract"`
	BlockPreviousResults       bool                               `json:"blockPreviousResults"`
	State                      string                             `json:"state"`
	ImpactRevision             int64                              `json:"impactRevision"`
	ImpactSHA256               string                             `json:"impactSha256"`
	Impact                     MissionChangeImpact                `json:"impact"`
	PlanningAssessment         *MissionChangePlanningAssessment   `json:"planningAssessment"`
	SuccessorMissionID         *string                            `json:"successorMissionId"`
	InputRevisionMap           []MissionChangeInputRevisionMap    `json:"inputRevisionMap"`
	CreatedAt                  string                             `json:"createdAt"`
	Events                     []MissionChangeRequestEvent        `json:"events"`
}

type missionChangeRequirementsBasis struct {
	MissionID          string                             `json:"missionId"`
	Title              string                             `json:"title"`
	Goal               string                             `json:"goal"`
	AcceptanceContract *taskvalidation.AcceptanceContract `json:"acceptanceContract"`
	InputRevisions     []MissionChangeInputRevision       `json:"inputRevisions"`
}

func normalizeMissionChangeRequestInput(input MissionChangeRequestInput, currentTitle, currentGoal string) (MissionChangeRequestInput, error) {
	input.ChangeSummary = strings.TrimSpace(input.ChangeSummary)
	input.ProposedTitle = strings.TrimSpace(input.ProposedTitle)
	input.ProposedGoal = strings.TrimSpace(input.ProposedGoal)
	if input.ProposedTitle == "" {
		input.ProposedTitle = strings.TrimSpace(currentTitle)
	}
	if input.ProposedGoal == "" {
		input.ProposedGoal = strings.TrimSpace(currentGoal)
	}
	if !utf8.ValidString(input.ChangeSummary) || !utf8.ValidString(input.ProposedTitle) || !utf8.ValidString(input.ProposedGoal) ||
		input.ChangeSummary == "" || len(input.ChangeSummary) > core.MaxContent ||
		input.ProposedTitle == "" || len(input.ProposedTitle) > 200 ||
		input.ProposedGoal == "" || len(input.ProposedGoal) > core.MaxContent ||
		taskvalidation.ValidateContract(input.ProposedAcceptanceContract) != nil {
		return MissionChangeRequestInput{}, core.Malformed
	}
	if input.ProposedAcceptanceContract != nil {
		encoded, err := json.Marshal(input.ProposedAcceptanceContract)
		if err != nil {
			return MissionChangeRequestInput{}, core.Malformed
		}
		var copied taskvalidation.AcceptanceContract
		if err = json.Unmarshal(encoded, &copied); err != nil {
			return MissionChangeRequestInput{}, core.Malformed
		}
		input.ProposedAcceptanceContract = &copied
	}
	return input, nil
}

func missionChangeRequirementsDigest(missionID, title, goal string, acceptance *taskvalidation.AcceptanceContract, inputs []MissionChangeInputRevision) (string, error) {
	orderedInputs := append([]MissionChangeInputRevision(nil), inputs...)
	sort.Slice(orderedInputs, func(left, right int) bool {
		if orderedInputs[left].InputID != orderedInputs[right].InputID {
			return orderedInputs[left].InputID < orderedInputs[right].InputID
		}
		return orderedInputs[left].Revision < orderedInputs[right].Revision
	})
	basis := missionChangeRequirementsBasis{
		MissionID: missionID, Title: title, Goal: goal, AcceptanceContract: acceptance, InputRevisions: orderedInputs,
	}
	encoded, err := json.Marshal(basis)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func missionChangeImpactDigest(impact MissionChangeImpact) (string, error) {
	if impact.SchemaVersion != MissionChangeImpactSchema || impact.MissionID == "" || !validTaskInputDigest(impact.BaseRequirementsSHA256) || impact.NaturalLanguageImpactStatus != "not_assessed" ||
		impact.InputRevisions == nil || impact.Tasks == nil || impact.Artifacts == nil || impact.ActiveWorkerSessions == nil || impact.NonterminalJobRuns == nil ||
		impact.ActiveServiceEndpoints == nil || impact.ActiveTaskTakeoverLeases == nil || impact.ReturnedHumanTakeoverSnapshots == nil {
		return "", core.Malformed
	}
	encoded, err := json.Marshal(impact)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validMissionChangeRequestTransition(current, next string) bool {
	switch current {
	case "":
		return next == "received"
	case "received":
		return next == "queued" || next == "considered" || next == "declined" || next == "superseded"
	case "queued":
		return next == "considered" || next == "declined" || next == "superseded"
	case "considered":
		return next == "considered" || next == "applied" || next == "declined" || next == "superseded"
	default:
		return false
	}
}
