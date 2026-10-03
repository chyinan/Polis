// pattern: Imperative Shell
package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"polis/internal/codex"
	"polis/internal/runner"
)

type FakeRuntimeConfig struct {
	TurnDelay                     time.Duration
	Model                         string
	Effort                        string
	Profile                       string
	Purpose                       string
	ExecutionEnvelope             string
	ToolCallLimit                 int
	ReadOnlySkillSurface          bool
	ReadOnlySkillDirectorySurface bool
	DirectMessagingSurface        bool
	SharedMissionArtifactSurface  bool
	ControlledMCPToolSurface      bool
	ControlledMCPToolSurfaceV2    bool
	ControlledMCPCallFixture      json.RawMessage
}

type FakeRuntime struct {
	config  FakeRuntimeConfig
	surface ToolSurface
	profile ExecutionProfile
	mu      sync.Mutex
	stats   RuntimeStats
}

const (
	OfflineExecutionEnvelope        = "polis-product-offline-runtime@1"
	ProductToolSurfaceQualification = "polis-product-tool-surface@4"
)

func NewFakeRuntime(config FakeRuntimeConfig) *FakeRuntime {
	if config.Model == "" {
		config.Model = "offline-model"
	}
	if config.Effort == "" {
		config.Effort = "medium"
	}
	if config.Profile == "" {
		config.Profile = config.Model + "/" + config.Effort
	}
	if config.Purpose == "" {
		config.Purpose = "product-artifact"
		if config.DirectMessagingSurface {
			config.Purpose = OfflineDirectMessagingToolSurfacePurpose
		} else if config.SharedMissionArtifactSurface {
			config.Purpose = OfflineSharedMissionArtifactToolSurfacePurpose
		} else if config.ReadOnlySkillDirectorySurface {
			config.Purpose = OfflineSkillDirectorySurfacePurpose
		} else if config.ReadOnlySkillSurface {
			config.Purpose = OfflineReadOnlySkillSurfacePurpose
		} else if config.ControlledMCPToolSurfaceV2 {
			config.Purpose = OfflineControlledMCPToolSurfaceV2Purpose
		} else if config.ControlledMCPToolSurface {
			config.Purpose = OfflineControlledMCPToolSurfacePurpose
		}
	}
	if config.ExecutionEnvelope == "" {
		config.ExecutionEnvelope = OfflineExecutionEnvelope
	}
	if config.ToolCallLimit == 0 {
		config.ToolCallLimit = 16
	}
	surface := ProductToolSurface()
	qualification := ProductToolSurfaceQualification
	exactSurfaceFingerprint := ProductExactSurfaceExecutionFingerprint
	providerFingerprint := ProductProviderL2Fingerprint
	if config.ControlledMCPToolSurfaceV2 {
		surface = ProductControlledMCPToolSurfaceV2()
		qualification = ProductControlledMCPToolSurfaceV2Qualification
		exactSurfaceFingerprint = OfflineControlledMCPToolSurfaceV2SimulationMarker
		providerFingerprint = OfflineControlledMCPToolSurfaceV2SimulationMarker
	} else if config.ControlledMCPToolSurface {
		surface = ProductControlledMCPToolSurface()
		qualification = ProductControlledMCPToolSurfaceQualification
		exactSurfaceFingerprint = OfflineControlledMCPToolSurfaceSimulationMarker
		providerFingerprint = OfflineControlledMCPToolSurfaceSimulationMarker
	} else if config.ReadOnlySkillDirectorySurface {
		surface = ProductSkillDirectoryToolSurface()
		qualification = ProductSkillDirectoryToolSurfaceQualification
		exactSurfaceFingerprint = OfflineSkillDirectorySurfaceSimulationMarker
		providerFingerprint = OfflineSkillDirectorySurfaceSimulationMarker
	} else if config.ReadOnlySkillSurface {
		surface = ProductSkillToolSurface()
		qualification = ProductSkillToolSurfaceQualification
		exactSurfaceFingerprint = OfflineSkillSurfaceSimulationMarker
		providerFingerprint = OfflineSkillSurfaceSimulationMarker
	} else if config.DirectMessagingSurface {
		surface = ProductDirectMessagingToolSurface()
		qualification = ProductDirectMessagingToolSurfaceQualification
		exactSurfaceFingerprint = OfflineDirectMessagingToolSurfaceSimulationMark
		providerFingerprint = OfflineDirectMessagingToolSurfaceSimulationMark
	} else if config.SharedMissionArtifactSurface {
		surface = ProductSharedMissionArtifactToolSurface()
		qualification = ProductSharedMissionArtifactToolSurfaceQualification
		exactSurfaceFingerprint = OfflineSharedMissionArtifactToolSurfaceSimulationMark
		providerFingerprint = OfflineSharedMissionArtifactToolSurfaceSimulationMark
	}
	profile := ExecutionProfile{Model: config.Model, Effort: config.Effort, Profile: config.Profile, ToolCallLimit: config.ToolCallLimit, Purpose: config.Purpose, ExactSurfaceExecutionFingerprint: exactSurfaceFingerprint, ProductProviderL2Fingerprint: providerFingerprint, ExecutionEnvelope: config.ExecutionEnvelope, ToolSurfaceQualification: qualification, TransportPolicy: codex.DefaultTransportPolicy()}
	return &FakeRuntime{config: config, surface: surface, profile: profile}
}

func (r *FakeRuntime) Mode() string                       { return "fake" }
func (r *FakeRuntime) ToolSurface() ToolSurface           { return r.surface }
func (r *FakeRuntime) ExecutionProfile() ExecutionProfile { return r.profile }
func (r *FakeRuntime) Readiness(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.config.ControlledMCPToolSurface && r.config.ControlledMCPToolSurfaceV2 {
		return errors.New("offline MCP runtime must select exactly one versioned tool surface")
	}
	if r.config.ReadOnlySkillSurface && r.config.ReadOnlySkillDirectorySurface {
		return errors.New("offline Skill runtime must select exactly one versioned tool surface")
	}
	if r.config.SharedMissionArtifactSurface && (r.config.DirectMessagingSurface || r.config.ReadOnlySkillSurface || r.config.ReadOnlySkillDirectorySurface || r.config.ControlledMCPToolSurface || r.config.ControlledMCPToolSurfaceV2) {
		return errors.New("offline shared-artifact access requires its isolated versioned tool surface")
	}
	if r.config.DirectMessagingSurface && (r.config.ReadOnlySkillSurface || r.config.ReadOnlySkillDirectorySurface || r.config.ControlledMCPToolSurface || r.config.ControlledMCPToolSurfaceV2) {
		return errors.New("offline direct messaging requires its isolated versioned tool surface")
	}
	if len(r.config.ControlledMCPCallFixture) > 0 && (!r.config.ControlledMCPToolSurface && !r.config.ControlledMCPToolSurfaceV2 || !json.Valid(r.config.ControlledMCPCallFixture)) {
		return errors.New("offline MCP call fixture requires the exact controlled-MCP surface and valid JSON")
	}
	qualifiedSurface := !r.config.ReadOnlySkillSurface && !r.config.ReadOnlySkillDirectorySurface && !r.config.DirectMessagingSurface && !r.config.ControlledMCPToolSurface && !r.config.ControlledMCPToolSurfaceV2 && r.profile.ToolSurfaceQualification == ProductToolSurfaceQualification && r.surface.ManifestDigest == ProductToolSurface().ManifestDigest && r.profile.Purpose != "" && r.profile.ExactSurfaceExecutionFingerprint == ProductExactSurfaceExecutionFingerprint && r.profile.ProductProviderL2Fingerprint == ProductProviderL2Fingerprint
	offlineSkillSurface := r.config.ReadOnlySkillSurface && ValidateOfflineFakeSkillSurface(r.Mode(), r.profile, r.surface) == nil
	offlineSkillDirectorySurface := r.config.ReadOnlySkillDirectorySurface && ValidateOfflineFakeSkillDirectorySurface(r.Mode(), r.profile, r.surface) == nil
	offlineDirectMessagingSurface := r.config.DirectMessagingSurface && ValidateOfflineFakeProductDirectMessagingSurface(r.Mode(), r.profile, r.surface) == nil
	offlineSharedMissionArtifactSurface := r.config.SharedMissionArtifactSurface && ValidateOfflineFakeSharedMissionArtifactSurface(r.Mode(), r.profile, r.surface) == nil
	offlineControlledMCPSurface := r.config.ControlledMCPToolSurface && !r.config.ReadOnlySkillSurface && !r.config.ReadOnlySkillDirectorySurface && ValidateOfflineFakeControlledMCPSurface(r.Mode(), r.profile, r.surface) == nil
	offlineControlledMCPSurfaceV2 := r.config.ControlledMCPToolSurfaceV2 && !r.config.ReadOnlySkillSurface && !r.config.ReadOnlySkillDirectorySurface && ValidateOfflineFakeControlledMCPSurfaceV2(r.Mode(), r.profile, r.surface) == nil
	if r.profile.Model == "" || r.profile.Effort != "medium" || r.profile.Profile != r.profile.Model+"/"+r.profile.Effort || r.profile.ToolCallLimit <= 0 || (!qualifiedSurface && !offlineSkillSurface && !offlineSkillDirectorySurface && !offlineDirectMessagingSurface && !offlineSharedMissionArtifactSurface && !offlineControlledMCPSurface && !offlineControlledMCPSurfaceV2) {
		return errors.New("offline provider runtime configuration is invalid")
	}
	return nil
}
func (r *FakeRuntime) Stats() RuntimeStats { r.mu.Lock(); defer r.mu.Unlock(); return r.stats }

func (r *FakeRuntime) Reserve(ctx context.Context, authorization ExecutionAuthorization) (Reservation, error) {
	if err := r.Readiness(ctx); err != nil {
		return Reservation{}, err
	}
	if err := validateRuntimeBinding(authorization, r.Mode(), r.profile, r.surface); err != nil {
		return Reservation{}, err
	}
	return Reservation{ID: "offline-no-provider-reservation"}, nil
}

func (r *FakeRuntime) CloseReservation(_ context.Context, _ Reservation, reason string) error {
	if reason == "" {
		return errors.New("reservation close reason is required")
	}
	return nil
}

func (r *FakeRuntime) Start(_ context.Context, options SessionStartOptions) (Session, error) {
	if options.SessionID == "" || options.Model != r.profile.Model || options.Effort != r.profile.Effort || options.Profile != r.profile.Profile || options.ToolSurface.ManifestDigest != r.surface.ManifestDigest {
		return nil, errors.New("fake provider session options are incomplete")
	}
	var process *runner.Process
	var err error
	if options.WorkerCgroup != nil {
		spec, specErr := runner.NewProcessLaunchSpec(fakeHelperCommand(), nil, "", runner.ProcessLaunchDirectories{})
		if specErr != nil {
			_ = options.WorkerCgroup.Cleanup()
			return nil, specErr
		}
		process, err = runner.StartWithLaunchSpecInWorkerCgroup(options.SessionID, spec, options.WorkerCgroup)
	} else {
		process, err = runner.Start(options.SessionID, fakeHelperCommand(), nil)
	}
	if err != nil {
		return nil, err
	}
	return &fakeSession{
		process: process, delay: r.config.TurnDelay, missionID: options.MissionID, taskID: options.TaskID,
		skillLoadSurface:      options.ToolSurface.ManifestDigest == ProductSkillToolSurface().ManifestDigest || options.ToolSurface.ManifestDigest == ProductSkillDirectoryToolSurface().ManifestDigest,
		skillDirectorySurface: options.ToolSurface.ManifestDigest == ProductSkillDirectoryToolSurface().ManifestDigest,
		mcpCallSurface:        options.ToolSurface.ManifestDigest == ProductControlledMCPToolSurface().ManifestDigest || options.ToolSurface.ManifestDigest == ProductControlledMCPToolSurfaceV2().ManifestDigest,
		mcpCallFixture:        append(json.RawMessage(nil), r.config.ControlledMCPCallFixture...),
	}, nil
}

type fakeSession struct {
	process               *runner.Process
	delay                 time.Duration
	missionID             string
	taskID                string
	skillLoadSurface      bool
	skillDirectorySurface bool
	mcpCallSurface        bool
	mcpCallFixture        json.RawMessage
}

func (s *fakeSession) Process() *runner.Process { return s.process }
func (s *fakeSession) Initialize(ctx context.Context, _ codex.TransportPolicy) error {
	return ctx.Err()
}
func (s *fakeSession) StartThread(ctx context.Context, _ ThreadStartOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "offline-thread", nil
}

func (s *fakeSession) Turn(ctx context.Context, _ string, _ string, _ codex.TurnOptions, handler ToolHandler) (TurnResult, error) {
	started := time.Now().UTC()
	calls := 0
	if s.delay > 0 {
		timer := time.NewTimer(s.delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return TurnResult{State: "interrupted", Outcome: "cancelled", StartedAt: started, FinishedAt: time.Now().UTC()}, ctx.Err()
		}
	}
	call := func(name string, raw any) (map[string]any, error) {
		calls++
		body, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		result, _ := handler(name, fmt.Sprintf("offline-call-%d", calls+1), body)
		var decoded map[string]any
		if err = json.Unmarshal(result, &decoded); err != nil {
			return nil, err
		}
		if errorText, ok := decoded["error"].(string); ok && errorText != "" {
			return decoded, errors.New(errorText)
		}
		return decoded, nil
	}
	if s.mcpCallSurface && len(s.mcpCallFixture) > 0 {
		current, contextErr := call("work_current", map[string]any{})
		if contextErr != nil {
			return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, contextErr
		}
		if !fakeMCPCallListed(current, s.mcpCallFixture) {
			return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, errors.New("offline MCP fixture call is absent from the current employee-bound tool context")
		}
		if _, callErr := call("mcp_call", s.mcpCallFixture); callErr != nil {
			return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, callErr
		}
		return TurnResult{State: "completed", Outcome: "controlled_mcp_fixture_completed", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, nil
	}
	current, err := call("work_current", map[string]any{})
	if err != nil {
		return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, err
	}
	if s.skillLoadSurface {
		skillID, relativePath := firstLoadableSkillReference(current)
		if s.skillDirectorySurface {
			skillID = firstBoundSkillID(current)
			if skillID != "" {
				cursor := ""
				for pageNumber := 0; pageNumber < 8; pageNumber++ {
					page, listErr := call("skills_list", map[string]any{"skill_id": skillID, "after_relative_path": cursor})
					if listErr != nil {
						return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, listErr
					}
					relativePath = firstLoadableSkillPath(page)
					if relativePath != "" {
						break
					}
					data, ok := page["data"].(map[string]any)
					truncated, _ := data["truncated"].(bool)
					next, _ := data["nextAfterRelativePath"].(string)
					if !ok || !truncated || next == "" || next == cursor {
						return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, errors.New("offline Skill directory omitted the required SKILL.md path")
					}
					cursor = next
				}
				if relativePath == "" {
					return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, errors.New("offline Skill directory exceeded its bounded page count")
				}
			}
		}
		if skillID != "" && relativePath != "" {
			loaded, loadErr := call("skills_load", map[string]any{"skill_id": skillID, "relative_path": relativePath})
			if loadErr != nil || nestedString(loaded, "data", "contentBoundary") != "approved_static_text_no_additional_permissions" || nestedString(loaded, "data", "contentDigest") == "" || nestedString(loaded, "data", "content") == "" {
				if loadErr == nil {
					loadErr = errors.New("offline Worker received an invalid bound Skill document")
				}
				return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, loadErr
			}
		}
	}
	workspace, err := call("workspace_read", map[string]any{})
	if err != nil {
		return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, err
	}
	workspaceDigest := nestedString(workspace, "data", "Digest")
	workspaceRevision := nestedNumber(workspace, "data", "Revision")
	taskID := nestedString(current, "data", "Task", "ID")
	if taskID == "" {
		taskID = s.taskID
	}
	content := fmt.Sprintf("Mission ID: %s\nTask ID: %s\nAcknowledgement: offline provider completed the assigned task.\nTask summary: completed using the current Task workspace.\n", s.missionID, taskID)
	if validationContract, ok := nestedMap(current, "data", "Task", "validation_binding", "contract"); ok {
		if required, ok := validationContract["required_text"].([]any); ok {
			for _, item := range required {
				if criterion, ok := item.(string); ok && !strings.Contains(content, criterion) {
					content += criterion + "\n"
				}
			}
		}
	}
	if _, err = call("workspace_replace", map[string]any{"expected_digest": workspaceDigest, "expected_revision": workspaceRevision, "content": content}); err != nil {
		return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, err
	}
	if _, staleErr := call("workspace_replace", map[string]any{"expected_digest": workspaceDigest, "expected_revision": workspaceRevision, "content": "stale writer must be rejected"}); staleErr == nil || staleErr.Error() != "REVISION_CONFLICT" {
		return TurnResult{State: "failed", Outcome: "stale_writer_not_rejected", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, errors.New("product workspace accepted a stale digest/revision writer")
	}
	check, err := call("workspace_check", map[string]any{})
	if err != nil {
		return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, err
	}
	if status := nestedString(check, "data", "status"); status != "PASS" {
		return TurnResult{State: "failed", Outcome: "offline_task_unqualified", ToolCalls: calls, ProviderEgress: 0, StartedAt: started, FinishedAt: time.Now().UTC()}, nil
	}
	checkID := nestedString(check, "receipt", "id")
	if checkID == "" {
		return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, errors.New("workspace_check did not return a receipt")
	}
	if _, err = call("task_submit", map[string]any{}); err != nil {
		return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, err
	}
	return TurnResult{State: "completed", Outcome: codex.OutcomeProviderTerminal, ToolCalls: calls, ProviderEgress: 0, StartedAt: started, FinishedAt: time.Now().UTC()}, nil
}

func fakeMCPCallListed(workCurrent map[string]any, rawCall json.RawMessage) bool {
	var request struct {
		CapabilityID     string `json:"capability_id"`
		ToolName         string `json:"tool_name"`
		ToolSchemaSHA256 string `json:"tool_schema_sha256"`
	}
	if json.Unmarshal(rawCall, &request) != nil || request.CapabilityID == "" || request.ToolName == "" || request.ToolSchemaSHA256 == "" {
		return false
	}
	data, ok := workCurrent["data"].(map[string]any)
	if !ok {
		return false
	}
	sets, ok := data["mcp_tool_sets"].([]any)
	if !ok {
		return false
	}
	for _, rawSet := range sets {
		set, ok := rawSet.(map[string]any)
		if !ok || set["capabilityId"] != request.CapabilityID || set["toolSchemaSha256"] != request.ToolSchemaSHA256 {
			continue
		}
		tools, ok := set["tools"].([]any)
		if !ok {
			return false
		}
		for _, rawTool := range tools {
			tool, ok := rawTool.(map[string]any)
			if ok && tool["name"] == request.ToolName {
				return true
			}
		}
	}
	return false
}

func firstLoadableSkillReference(current map[string]any) (string, string) {
	catalog, ok := current["data"].(map[string]any)
	if !ok {
		return "", ""
	}
	items, ok := catalog["skill_catalog"].([]any)
	if !ok {
		return "", ""
	}
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		skillID, _ := item["skillId"].(string)
		references, _ := item["references"].([]any)
		for _, rawReference := range references {
			reference, ok := rawReference.(map[string]any)
			if !ok {
				continue
			}
			loadable, _ := reference["loadable"].(bool)
			relativePath, _ := reference["relativePath"].(string)
			if skillID != "" && loadable && relativePath == "SKILL.md" {
				return skillID, relativePath
			}
		}
	}
	return "", ""
}

func firstBoundSkillID(current map[string]any) string {
	data, ok := current["data"].(map[string]any)
	if !ok {
		return ""
	}
	items, ok := data["skill_catalog"].([]any)
	if !ok {
		return ""
	}
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		if skillID, _ := item["skillId"].(string); skillID != "" {
			return skillID
		}
	}
	return ""
}

func firstLoadableSkillPath(page map[string]any) string {
	data, ok := page["data"].(map[string]any)
	if !ok {
		return ""
	}
	files, ok := data["files"].([]any)
	if !ok {
		return ""
	}
	for _, rawFile := range files {
		file, ok := rawFile.(map[string]any)
		if !ok {
			continue
		}
		loadable, _ := file["loadable"].(bool)
		relativePath, _ := file["relativePath"].(string)
		if loadable && relativePath == "SKILL.md" {
			return relativePath
		}
	}
	return ""
}

func (s *fakeSession) Stop(_ context.Context) (runner.StopProof, error) { return s.process.Stop() }

func fakeHelperCommand() []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd.exe", "/c", "ping", "127.0.0.1", "-n", "60"}
	}
	return []string{"/bin/sleep", "60"}
}

func nestedString(value map[string]any, path ...string) string {
	current := any(value)
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[key]
	}
	result, _ := current.(string)
	return result
}

func nestedMap(value map[string]any, path ...string) (map[string]any, bool) {
	current := any(value)
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current = object[key]
	}
	result, ok := current.(map[string]any)
	return result, ok
}

func nestedNumber(value map[string]any, path ...string) int64 {
	current := any(value)
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return 0
		}
		current = object[key]
	}
	number, ok := current.(float64)
	if !ok {
		return 0
	}
	return int64(number)
}

func DigestTools(tools []any) string {
	raw, _ := json.Marshal(tools)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
