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
	TurnDelay         time.Duration
	Model             string
	Effort            string
	Profile           string
	Purpose           string
	ExecutionEnvelope string
	ToolCallLimit     int
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
	}
	if config.ExecutionEnvelope == "" {
		config.ExecutionEnvelope = OfflineExecutionEnvelope
	}
	if config.ToolCallLimit == 0 {
		config.ToolCallLimit = 16
	}
	profile := ExecutionProfile{Model: config.Model, Effort: config.Effort, Profile: config.Profile, ToolCallLimit: config.ToolCallLimit, Purpose: config.Purpose, ExactSurfaceExecutionFingerprint: ProductExactSurfaceExecutionFingerprint, ProductProviderL2Fingerprint: ProductProviderL2Fingerprint, ExecutionEnvelope: config.ExecutionEnvelope, ToolSurfaceQualification: ProductToolSurfaceQualification, TransportPolicy: codex.DefaultTransportPolicy()}
	return &FakeRuntime{config: config, surface: ProductToolSurface(), profile: profile}
}

func (r *FakeRuntime) Mode() string                       { return "fake" }
func (r *FakeRuntime) ToolSurface() ToolSurface           { return r.surface }
func (r *FakeRuntime) ExecutionProfile() ExecutionProfile { return r.profile }
func (r *FakeRuntime) Readiness(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.profile.Model == "" || r.profile.Effort != "medium" || r.profile.Profile != r.profile.Model+"/"+r.profile.Effort || r.profile.ToolCallLimit <= 0 || r.profile.Purpose == "" || r.profile.ExactSurfaceExecutionFingerprint != ProductExactSurfaceExecutionFingerprint || r.profile.ProductProviderL2Fingerprint != ProductProviderL2Fingerprint {
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
	process, err := runner.Start(options.SessionID, fakeHelperCommand(), nil)
	if err != nil {
		return nil, err
	}
	return &fakeSession{process: process, delay: r.config.TurnDelay, missionID: options.MissionID, taskID: options.TaskID}, nil
}

type fakeSession struct {
	process   *runner.Process
	delay     time.Duration
	missionID string
	taskID    string
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
	current, err := call("work_current", map[string]any{})
	if err != nil {
		return TurnResult{State: "failed", Outcome: "tool_failure", ToolCalls: calls, StartedAt: started, FinishedAt: time.Now().UTC()}, err
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

func (s *fakeSession) Stop(_ context.Context) (runner.StopProof, error) { return s.process.Stop() }

func fakeHelperCommand() []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd.exe", "/c", "ping", "127.0.0.1", "-n", "60"}
	}
	return []string{"/bin/sh", "-c", "sleep 60"}
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
