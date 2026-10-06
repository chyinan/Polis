// pattern: Functional Core
package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"polis/internal/taskvalidation"
	"regexp"
	"strconv"
	"strings"
)

const (
	viewSchemaVersion = 3
	maxActivityLimit  = 100
)

var companyIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

var errInvalidActivityCursor = errors.New("invalid activity cursor")

type ViewMeta struct {
	SchemaVersion  int    `json:"schemaVersion"`
	CompanyID      string `json:"companyId"`
	EntityRevision string `json:"entityRevision"`
	SnapshotCursor string `json:"snapshotCursor"`
	ObservedAt     string `json:"observedAt"`
	DataMode       string `json:"dataMode"`
	Freshness      string `json:"freshness"`
	SourceLabel    string `json:"sourceLabel"`
	RecoveryState  string `json:"recoveryState"`
}

type EntityRef struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

type ActivityActor struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

type ToolBudgetView struct {
	Limit     *string `json:"limit"`
	Used      *string `json:"used"`
	Remaining *string `json:"remaining"`
	Quality   string  `json:"quality"`
}

type QualificationView struct {
	Status         string  `json:"status"`
	EvidenceID     *string `json:"evidenceId"`
	PolicyRevision *string `json:"policyRevision"`
}

type EmployeeStatusView struct {
	Primary             string `json:"primary"`
	Tone                string `json:"tone"`
	Reason              string `json:"reason"`
	ActiveModelRequests string `json:"activeModelRequests"`
	InFlightTools       string `json:"inFlightTools"`
	ObservedAt          string `json:"observedAt"`
}

type EmployeeScheduleView struct {
	State             string  `json:"state"`
	WorkGeneration    string  `json:"workGeneration"`
	CheckedGeneration string  `json:"checkedGeneration"`
	NextDueAt         *string `json:"nextDueAt"`
	PauseReason       string  `json:"pauseReason"`
}

type EmployeeRoleRevisionView struct {
	RevisionSHA256 string   `json:"revisionSha256"`
	EmployeeID     string   `json:"employeeId"`
	RoleName       string   `json:"roleName"`
	TaskTypes      []string `json:"taskTypes"`
	TaskKinds      []string `json:"taskKinds"`
	OwnerDecision  string   `json:"ownerDecision"`
	Qualification  string   `json:"qualification"`
}

type EmployeeSummary struct {
	EmployeeID          string                    `json:"employeeId"`
	DisplayName         string                    `json:"displayName"`
	Role                string                    `json:"role"`
	RoleRevision        *string                   `json:"roleRevision"`
	RoleRevisionDetail  *EmployeeRoleRevisionView `json:"roleRevisionDetail,omitempty"`
	Epoch               string                    `json:"epoch"`
	SessionID           *string                   `json:"sessionId"`
	SessionState        *string                   `json:"sessionState"`
	Profile             *string                   `json:"profile"`
	CurrentTask         *EntityRef                `json:"currentTask"`
	Status              EmployeeStatusView        `json:"status"`
	Schedule            *EmployeeScheduleView     `json:"schedule"`
	ToolBudget          ToolBudgetView            `json:"toolBudget"`
	Qualification       QualificationView         `json:"qualification"`
	OpenObligationCount string                    `json:"openObligationCount"`
}

type TaskSummary struct {
	TaskID             string                    `json:"taskId"`
	Title              string                    `json:"title"`
	Kind               string                    `json:"kind"`
	State              string                    `json:"state"`
	OwnerEmployeeID    string                    `json:"ownerEmployeeId"`
	Generation         string                    `json:"generation"`
	ContractRevisionID *string                   `json:"contractRevisionId"`
	WorkspaceRevision  *string                   `json:"workspaceRevision"`
	Acceptance         string                    `json:"acceptance"`
	DependencyLabel    *string                   `json:"dependencyLabel"`
	SemanticRevision   *TaskSemanticRevisionView `json:"semanticRevision,omitempty"`
}

type TaskSemanticRevisionView struct {
	TaskID             string `json:"taskId"`
	Revision           string `json:"revision"`
	BindingSHA256      string `json:"bindingSha256"`
	RoleRevisionSHA256 string `json:"roleRevisionSha256"`
	TaskType           string `json:"taskType"`
	TaskKind           string `json:"taskKind"`
	OwnerEmployeeID    string `json:"ownerEmployeeId"`
	Qualification      string `json:"qualification"`
	RequiresHuman      bool   `json:"requiresHuman"`
	ReasonCode         string `json:"reasonCode"`
	Decision           string `json:"decision"`
	DecisionRationale  string `json:"decisionRationale"`
	DecisionActor      string `json:"decisionActor"`
	DecisionRequestID  string `json:"decisionRequestId"`
	DecisionCreatedAt  string `json:"decisionCreatedAt"`
}

type ContractRevisionSummary struct {
	RevisionID         string  `json:"revisionId"`
	Revision           string  `json:"revision"`
	Endpoint           string  `json:"endpoint"`
	State              string  `json:"state"`
	Digest             string  `json:"digest"`
	ProposerEmployeeID string  `json:"proposerEmployeeId"`
	AccepterEmployeeID *string `json:"accepterEmployeeId"`
}

type ObligationSummary struct {
	ObligationID    string  `json:"obligationId"`
	MessageID       string  `json:"messageId"`
	OwnerEmployeeID string  `json:"ownerEmployeeId"`
	State           string  `json:"state"`
	EvidenceRef     *string `json:"evidenceRef"`
	Note            string  `json:"note"`
}

type ArtifactSummary struct {
	ArtifactID         string  `json:"artifactId"`
	TaskID             string  `json:"taskId"`
	AuthorEmployeeID   string  `json:"authorEmployeeId"`
	Digest             string  `json:"digest"`
	Bytes              string  `json:"bytes"`
	State              string  `json:"state"`
	Verdict            string  `json:"verdict"`
	ContractRevisionID *string `json:"contractRevisionId"`
	QualificationID    *string `json:"qualificationId"`
	CheckpointID       *string `json:"checkpointId"`
}

type CheckpointSummary struct {
	CheckpointID       string  `json:"checkpointId"`
	TaskID             string  `json:"taskId"`
	EmployeeID         string  `json:"employeeId"`
	SessionID          string  `json:"sessionId"`
	SessionState       string  `json:"sessionState"`
	SessionEpoch       string  `json:"sessionEpoch"`
	Kind               string  `json:"kind"`
	State              string  `json:"state"`
	QualificationState string  `json:"qualificationState"`
	WorkspaceRevision  *string `json:"workspaceRevision"`
	WorkspaceDigest    *string `json:"workspaceDigest"`
	ArtifactID         *string `json:"artifactId"`
}

type ResourceSummary struct {
	ToolCallsUsed     string  `json:"toolCallsUsed"`
	ToolCallsLimit    string  `json:"toolCallsLimit"`
	ToolBudgetQuality string  `json:"toolBudgetQuality"`
	MoneyQuality      string  `json:"moneyQuality"`
	MoneyAmount       *string `json:"moneyAmount"`
	Currency          *string `json:"currency"`
	AsOf              string  `json:"asOf"`
	Note              string  `json:"note"`
}

type AttentionItem struct {
	ID                string    `json:"id"`
	Tone              string    `json:"tone"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	Subject           EntityRef `json:"subject"`
	EvidenceRefs      []string  `json:"evidenceRefs"`
	WorkflowState     string    `json:"workflowState,omitempty"`
	NotificationState string    `json:"notificationState,omitempty"`
}

type Milestone struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	State string `json:"state"`
}

type MissionSummary struct {
	MissionID               string                             `json:"missionId"`
	Title                   string                             `json:"title"`
	Goal                    string                             `json:"goal"`
	State                   string                             `json:"state"`
	Contract                string                             `json:"contract"`
	AcceptanceContract      *taskvalidation.AcceptanceContract `json:"acceptanceContract"`
	CurrentContractRevision *ContractRevisionSummary           `json:"currentContractRevision"`
	NextMilestone           string                             `json:"nextMilestone"`
	VerifiedMilestones      string                             `json:"verifiedMilestones"`
	MilestoneTotal          string                             `json:"milestoneTotal"`
	Milestones              []Milestone                        `json:"milestones"`
	Closeout                *MissionCloseoutSummary            `json:"closeout,omitempty"`
}

type MissionCloseoutSummary struct {
	RequestedOutcome      string          `json:"requestedOutcome"`
	Rationale             string          `json:"rationale"`
	AcceptanceArtifactIDs []string        `json:"acceptanceArtifactIds"`
	RequestID             string          `json:"requestId"`
	OpenedAt              string          `json:"openedAt"`
	TerminalOutcome       *string         `json:"terminalOutcome"`
	Report                json.RawMessage `json:"report,omitempty"`
	FinishedAt            *string         `json:"finishedAt"`
}

type ActivityEvent struct {
	ID           string            `json:"id"`
	CompanySeq   string            `json:"companySeq"`
	OccurredAt   string            `json:"occurredAt"`
	Kind         string            `json:"kind"`
	Actor        ActivityActor     `json:"actor"`
	Subject      EntityRef         `json:"subject"`
	Summary      string            `json:"summary"`
	Detail       string            `json:"detail"`
	Tone         string            `json:"tone"`
	EvidenceRefs []string          `json:"evidenceRefs"`
	Metadata     map[string]string `json:"metadata"`
}

type OperatorInstructionView struct {
	InstructionID         string                            `json:"instructionId"`
	MissionID             *string                           `json:"missionId"`
	TaskID                *string                           `json:"taskId"`
	EmployeeID            *string                           `json:"employeeId"`
	Content               string                            `json:"content"`
	State                 string                            `json:"state"`
	CreatedAt             string                            `json:"createdAt"`
	ResponseOutcome       *string                           `json:"responseOutcome"`
	ResponseSummary       *string                           `json:"responseSummary"`
	RespondedByEmployeeID *string                           `json:"respondedByEmployeeId"`
	RespondedAt           *string                           `json:"respondedAt"`
	Responses             []OperatorInstructionResponseView `json:"responses"`
}

type OperatorInstructionResponseView struct {
	EmployeeID  string `json:"employeeId"`
	Outcome     string `json:"outcome"`
	Summary     string `json:"summary"`
	RespondedAt string `json:"respondedAt"`
}

type CollaborationItem struct {
	MessageID           string  `json:"messageId"`
	MissionID           string  `json:"missionId"`
	TaskID              string  `json:"taskId"`
	SenderEmployeeID    string  `json:"senderEmployeeId"`
	RecipientEmployeeID string  `json:"recipientEmployeeId"`
	Content             string  `json:"content"`
	Kind                string  `json:"kind"`
	DeliveryState       string  `json:"deliveryState"`
	ContractRevisionID  *string `json:"contractRevisionId"`
	ObligationID        *string `json:"obligationId"`
	ObligationState     *string `json:"obligationState"`
	EvidenceRef         *string `json:"evidenceRef"`
	TaskRevision        string  `json:"taskRevision"`
}

type WorkspaceFile struct {
	Path    string `json:"path"`
	Bytes   string `json:"bytes"`
	Content string `json:"content"`
}

type WorkspaceView struct {
	TaskID       string              `json:"taskId"`
	Revision     string              `json:"revision"`
	Digest       string              `json:"digest"`
	Files        []WorkspaceFile     `json:"files"`
	ChangedFiles []string            `json:"changedFiles"`
	Checkpoints  []CheckpointSummary `json:"checkpoints"`
}

type ArtifactDetailView struct {
	ArtifactID       string `json:"artifactId"`
	TaskID           string `json:"taskId"`
	Digest           string `json:"digest"`
	Bytes            string `json:"bytes"`
	State            string `json:"state"`
	Verdict          string `json:"verdict"`
	Content          string `json:"content"`
	ContentAvailable bool   `json:"contentAvailable"`
}

type OperationsView struct {
	CompanyID            string                    `json:"companyId"`
	ToolCallsUsed        string                    `json:"toolCallsUsed"`
	ToolCallsLimit       string                    `json:"toolCallsLimit"`
	ToolBudgetQuality    string                    `json:"toolBudgetQuality"`
	InputTokens          *string                   `json:"inputTokens"`
	OutputTokens         *string                   `json:"outputTokens"`
	ElapsedRuntime       *string                   `json:"elapsedRuntime"`
	WorkerCount          string                    `json:"workerCount"`
	PostgreSQLStatus     string                    `json:"postgresqlStatus"`
	CASStatus            string                    `json:"casStatus"`
	EventStreamStatus    string                    `json:"eventStreamStatus"`
	LastRuntimeError     *string                   `json:"lastRuntimeError"`
	GenericActionIntents []GenericActionIntentView `json:"genericActionIntents,omitempty"`
}

type GenericActionIntentView struct {
	IntentID       string `json:"intentId"`
	TaskID         string `json:"taskId"`
	SessionID      string `json:"sessionId"`
	ActionKind     string `json:"actionKind"`
	ResourceKey    string `json:"resourceKey"`
	TargetSHA256   string `json:"targetSha256"`
	InputSHA256    string `json:"inputSha256"`
	IdempotencyKey string `json:"idempotencyKey"`
	State          string `json:"state"`
	ReasonCode     string `json:"reasonCode"`
	Actor          string `json:"actor"`
	RequestID      string `json:"requestId"`
	CreatedAt      string `json:"createdAt"`
}

type NotificationRouteView struct {
	RouteID             string `json:"routeId"`
	Adapter             string `json:"adapter"`
	Enabled             bool   `json:"enabled"`
	Destination         string `json:"destination"`
	SafetyAlias         string `json:"safetyAlias"`
	CredentialRef       string `json:"credentialRef"`
	Status              string `json:"status"`
	RouteRevision       string `json:"routeRevision"`
	QualificationStatus string `json:"qualificationStatus"`
	QualifiedUntil      string `json:"qualifiedUntil"`
}

type NotificationDeliveryView struct {
	DeliveryID string  `json:"deliveryId"`
	IntentID   string  `json:"intentId"`
	Adapter    string  `json:"adapter"`
	State      string  `json:"state"`
	ErrorCode  *string `json:"errorCode"`
	CreatedAt  string  `json:"createdAt"`
}

type NotificationsView struct {
	Routes     []NotificationRouteView    `json:"routes"`
	Deliveries []NotificationDeliveryView `json:"deliveries"`
}

type CompanyOverviewView struct {
	Meta           ViewMeta            `json:"meta"`
	Company        CompanyView         `json:"company"`
	Mission        MissionSummary      `json:"mission"`
	Team           TeamSummary         `json:"team"`
	Employees      []EmployeeSummary   `json:"employees"`
	Tasks          []TaskSummary       `json:"tasks"`
	Obligations    []ObligationSummary `json:"obligations"`
	Artifacts      []ArtifactSummary   `json:"artifacts"`
	Checkpoints    []CheckpointSummary `json:"checkpoints"`
	Resources      ResourceSummary     `json:"resources"`
	Attention      []AttentionItem     `json:"attention"`
	RecentActivity []ActivityEvent     `json:"recentActivity"`
}

type CompanyView struct {
	CompanyID   string `json:"companyId"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type TeamSummary struct {
	Total    string `json:"total"`
	Working  string `json:"working"`
	Sleeping string `json:"sleeping"`
	Waiting  string `json:"waiting"`
	Stopped  string `json:"stopped"`
}

type ActivityView struct {
	Meta       ViewMeta        `json:"meta"`
	Items      []ActivityEvent `json:"items"`
	NextCursor *string         `json:"nextCursor"`
}

type ActivityQuery struct {
	CompanyID      string
	SnapshotCursor string
	Cursor         *string
	Limit          int
}

type ReadModel interface {
	GetCompanyOverview(ctx context.Context, companyID string) (CompanyOverviewView, error)
	ListActivity(ctx context.Context, query ActivityQuery) (ActivityView, error)
}

type ActivityStreamer interface {
	StreamActivity(ctx context.Context, companyID string, cursor int64, emit func(ActivityEvent) error) error
}

type InstructionReader interface {
	ListOperatorInstructions(ctx context.Context, companyID string) ([]OperatorInstructionView, error)
}

type CollaborationReader interface {
	ListCollaboration(ctx context.Context, companyID string) ([]CollaborationItem, error)
}

type WorkspaceReader interface {
	GetWorkspace(ctx context.Context, companyID, taskID string) (WorkspaceView, error)
	GetArtifact(ctx context.Context, companyID, artifactID string) (ArtifactDetailView, error)
}

type OperationsReader interface {
	GetOperations(ctx context.Context, companyID string) (OperationsView, error)
}

// ActiveWorkReader is a narrow authoritative read used by a desktop quit
// guard. It does not expose database credentials or raw product rows.
type ActiveWorkReader interface {
	HasActiveWork(ctx context.Context) (bool, error)
}

type NotificationsReader interface {
	ListNotifications(ctx context.Context, companyID string) (NotificationsView, error)
}

func validateCompanyID(companyID string) error {
	if !companyIDPattern.MatchString(companyID) {
		return fmt.Errorf("invalid company scope")
	}
	return nil
}

func validateActivityLimit(limit int) error {
	if limit < 1 || limit > maxActivityLimit {
		return fmt.Errorf("activity limit must be between 1 and %d", maxActivityLimit)
	}
	return nil
}

func parseSnapshotCursor(cursor string) (int64, error) {
	const prefix = "company-seq:"
	if !strings.HasPrefix(cursor, prefix) {
		return 0, fmt.Errorf("invalid snapshot cursor")
	}
	sequence, err := strconv.ParseInt(strings.TrimPrefix(cursor, prefix), 10, 64)
	if err != nil || sequence < 0 {
		return 0, fmt.Errorf("invalid snapshot cursor")
	}
	return sequence, nil
}

func parseActivityCursor(cursor *string) (*int64, error) {
	if cursor == nil {
		return nil, nil
	}
	if strings.TrimSpace(*cursor) == "" {
		return nil, fmt.Errorf("activity cursor cannot be blank")
	}
	sequence, err := strconv.ParseInt(*cursor, 10, 64)
	if err != nil || sequence < 0 {
		return nil, fmt.Errorf("invalid activity cursor")
	}
	return &sequence, nil
}

func cursorForSequence(sequence int64) string {
	return strconv.FormatInt(sequence, 10)
}

func snapshotCursorForSequence(sequence int64) string {
	return fmt.Sprintf("company-seq:%d", sequence)
}

func presentationForEventKind(rawKind string) (string, string) {
	switch {
	case rawKind == "mission.create" || rawKind == "mission.draft":
		return "mission_created", "info"
	case rawKind == "mission.start":
		return "mission_started", "success"
	case rawKind == "mission.cancel":
		return "mission_cancelled", "neutral"
	case rawKind == "provider.runtime.initialization_failed":
		return "provider_runtime_initialization_failed", "danger"
	case rawKind == "provider.runtime.thread_start_failed":
		return "provider_runtime_thread_start_failed", "danger"
	case strings.Contains(rawKind, "provider.turn.completed"):
		return "provider_turn_completed", "success"
	case strings.Contains(rawKind, "provider.turn.failed") || strings.Contains(rawKind, "provider.turn.inconclusive"):
		return "provider_turn_failed", "danger"
	case strings.Contains(rawKind, "worker") && strings.Contains(rawKind, "active"):
		return "employee_started_task", "info"
	case strings.Contains(rawKind, "contract") && strings.Contains(rawKind, "accept"):
		return "contract_revision_accepted", "success"
	case strings.Contains(rawKind, "contract") && strings.Contains(rawKind, "propos"):
		return "contract_revision_proposed", "info"
	case strings.Contains(rawKind, "collab") && strings.Contains(rawKind, "send"):
		return "peer_message_sent", "info"
	case strings.Contains(rawKind, "obligation") && strings.Contains(rawKind, "observ"):
		return "obligation_observed", "info"
	case strings.Contains(rawKind, "workspace"):
		return "workspace_updated", "info"
	case strings.Contains(rawKind, "checkpoint"):
		return "checkpoint_saved", "info"
	case strings.Contains(rawKind, "artifact"):
		return "artifact_submitted", "info"
	case strings.Contains(rawKind, "accept"):
		return "acceptance_passed", "success"
	case strings.Contains(rawKind, "successor") || strings.Contains(rawKind, "handover"):
		return "successor_resumed", "success"
	case strings.Contains(rawKind, "stop"):
		return "employee_stopped", "neutral"
	case strings.Contains(rawKind, "task") && strings.Contains(rawKind, "claim"):
		return "employee_started_task", "info"
	default:
		return "workspace_updated", "info"
	}
}

func stringValue(value int64) string {
	return strconv.FormatInt(value, 10)
}

func pointer(value string) *string {
	return &value
}

func selectCurrentCheckpoint(rows []checkpointRow) checkpointRow {
	if len(rows) == 0 {
		return checkpointRow{}
	}
	chosen := rows[0]
	for _, candidate := range rows[1:] {
		if checkpointPrecedes(chosen, candidate) {
			chosen = candidate
		}
	}
	return chosen
}

func checkpointPrecedes(left, right checkpointRow) bool {
	if left.SessionEpoch != right.SessionEpoch {
		return left.SessionEpoch < right.SessionEpoch
	}
	if left.WorkspaceRevision != right.WorkspaceRevision {
		return left.WorkspaceRevision < right.WorkspaceRevision
	}
	if checkpointKindRank(left.Kind) != checkpointKindRank(right.Kind) {
		return checkpointKindRank(left.Kind) < checkpointKindRank(right.Kind)
	}
	if (left.ArtifactID != "") != (right.ArtifactID != "") {
		return left.ArtifactID == ""
	}
	return left.ID < right.ID
}

func checkpointKindRank(kind string) int {
	if kind == "qualified" {
		return 1
	}
	return 0
}
