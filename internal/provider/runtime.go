// pattern: Functional Core
package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"polis/internal/codex"
	"polis/internal/runner"
)

type ToolHandler func(name, callID string, raw json.RawMessage) (json.RawMessage, bool)

type ToolSurface struct {
	Tools                 []any
	ToolCount             int
	ManifestDigest        string
	AggregateSchemaDigest string
	AggregateSchemaBytes  int
}

func ProductToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeTools())
}

// ProductSkillToolSurface is the separately versioned Skill-load contract.
// It is intentionally not accepted by the current provider authorization gate.
const ProductSkillToolSurfaceQualification = "polis-product-tool-surface@5"

func ProductSkillToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithReadOnlySkill())
}

// ProductGuidanceToolSurface adds the persisted operator guidance inbox and
// response tools. It is not accepted by the current provider gate.
const ProductGuidanceToolSurfaceQualification = "polis-product-tool-surface@6"

func ProductGuidanceToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithGuidance())
}

// ProductDirectMessagingToolSurface is the separately versioned direct-message
// extension. It is accepted only by the exact zero-egress fake validator below.
const ProductDirectMessagingToolSurfaceQualification = "polis-product-tool-surface@7"

const (
	OfflineDirectMessagingToolSurfacePurpose        = "offline-direct-messaging-tool-surface"
	OfflineDirectMessagingToolSurfaceSimulationMark = "offline-direct-messaging-tool-surface-unqualified"
	ProductDirectMessagingManifestDigest            = "82d7b2dbc41ff3dbed56813b3b3adcfad48818fb29653bcd2debf1f280e507eb"
	ProductDirectMessagingSchemaDigest              = "769f7c9f4afb1c1d0ebfb43037a06d8f2ddb661bc111970c4d199f48f962fd42"
	ProductDirectMessagingSchemaBytes               = 3503
)

func ProductDirectMessagingToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithDirectMessaging())
}

// ValidateOfflineFakeProductDirectMessagingSurface verifies the exact pinned
// @7 shape under an explicitly unqualified zero-egress simulation profile.
func ValidateOfflineFakeProductDirectMessagingSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductDirectMessagingToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductDirectMessagingToolSurfaceQualification ||
		profile.Purpose != OfflineDirectMessagingToolSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineDirectMessagingToolSurfaceSimulationMark ||
		profile.ProductProviderL2Fingerprint != OfflineDirectMessagingToolSurfaceSimulationMark || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 12 || expected.ManifestDigest != ProductDirectMessagingManifestDigest ||
		expected.AggregateSchemaBytes != ProductDirectMessagingSchemaBytes || expected.AggregateSchemaDigest != ProductDirectMessagingSchemaDigest ||
		surface.ToolCount != 12 || observed.ToolCount != 12 ||
		surface.ManifestDigest != ProductDirectMessagingManifestDigest || observed.ManifestDigest != ProductDirectMessagingManifestDigest ||
		surface.AggregateSchemaBytes != ProductDirectMessagingSchemaBytes || observed.AggregateSchemaBytes != ProductDirectMessagingSchemaBytes ||
		surface.AggregateSchemaDigest != ProductDirectMessagingSchemaDigest || observed.AggregateSchemaDigest != ProductDirectMessagingSchemaDigest {
		return fmt.Errorf("direct-message product surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

const (
	ProductControlledMCPToolSurfaceQualification           = "polis-product-tool-surface@mcp-v1"
	ProductControlledMCPToolSurfaceV2Qualification         = "polis-product-tool-surface@mcp-v2"
	OfflineControlledMCPToolSurfacePurpose                 = "offline-controlled-mcp-tool-surface"
	OfflineControlledMCPToolSurfaceSimulationMarker        = "offline-controlled-mcp-tool-surface-unqualified"
	OfflineControlledMCPToolSurfaceV2Purpose               = "offline-controlled-mcp-tool-surface-v2"
	OfflineControlledMCPToolSurfaceV2SimulationMarker      = "offline-controlled-mcp-tool-surface-v2-unqualified"
	ProductControlledMCPToolSurfaceV1ManifestDigest        = "4856eeb48a1b70dd71727223e0877c8a6f68c883616f72dcc747b0c18291f451"
	ProductControlledMCPToolSurfaceV1AggregateSchemaBytes  = 2647
	ProductControlledMCPToolSurfaceV1AggregateSchemaDigest = "699ad79b737f03ba5b954ebaef4a0dd8b1d3b90c4df6928d865dbc2b9c63ae14"
	ProductControlledMCPToolSurfaceV2ManifestDigest        = "11deb3d2e8b5038d21c23d61ab2b53af9a267c8bbd0c4828814b597064063346"
	ProductControlledMCPToolSurfaceV2AggregateSchemaBytes  = 2647
	ProductControlledMCPToolSurfaceV2AggregateSchemaDigest = "699ad79b737f03ba5b954ebaef4a0dd8b1d3b90c4df6928d865dbc2b9c63ae14"
)

// ProductControlledMCPToolSurface is a separately versioned extension. It is
// never accepted by the real-provider gate without its own exact-surface
// qualification.
func ProductControlledMCPToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithControlledMCP())
}

// ProductControlledMCPToolSurfaceV2 adds the fixed Streamable HTTP profile
// wording without mutating the historical stdio-only surface.
func ProductControlledMCPToolSurfaceV2() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithControlledMCPV2())
}

// ValidateOfflineFakeControlledMCPSurface is only a zero-egress harness gate;
// it does not qualify an MCP endpoint, process or real model interaction.
func ValidateOfflineFakeControlledMCPSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductControlledMCPToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductControlledMCPToolSurfaceQualification ||
		profile.Purpose != OfflineControlledMCPToolSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineControlledMCPToolSurfaceSimulationMarker ||
		profile.ProductProviderL2Fingerprint != OfflineControlledMCPToolSurfaceSimulationMarker || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 8 || expected.ManifestDigest != ProductControlledMCPToolSurfaceV1ManifestDigest ||
		expected.AggregateSchemaBytes != ProductControlledMCPToolSurfaceV1AggregateSchemaBytes || expected.AggregateSchemaDigest != ProductControlledMCPToolSurfaceV1AggregateSchemaDigest ||
		surface.ToolCount != 8 || observed.ToolCount != 8 ||
		surface.ManifestDigest != ProductControlledMCPToolSurfaceV1ManifestDigest || observed.ManifestDigest != ProductControlledMCPToolSurfaceV1ManifestDigest ||
		surface.AggregateSchemaBytes != ProductControlledMCPToolSurfaceV1AggregateSchemaBytes || observed.AggregateSchemaBytes != ProductControlledMCPToolSurfaceV1AggregateSchemaBytes ||
		surface.AggregateSchemaDigest != ProductControlledMCPToolSurfaceV1AggregateSchemaDigest || observed.AggregateSchemaDigest != ProductControlledMCPToolSurfaceV1AggregateSchemaDigest {
		return fmt.Errorf("controlled-MCP tool surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

// ValidateOfflineFakeControlledMCPSurfaceV2 accepts only the transport-neutral
// tool description in the explicit zero-egress fake simulation.
func ValidateOfflineFakeControlledMCPSurfaceV2(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductControlledMCPToolSurfaceV2()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductControlledMCPToolSurfaceV2Qualification ||
		profile.Purpose != OfflineControlledMCPToolSurfaceV2Purpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineControlledMCPToolSurfaceV2SimulationMarker ||
		profile.ProductProviderL2Fingerprint != OfflineControlledMCPToolSurfaceV2SimulationMarker || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 8 || expected.ManifestDigest != ProductControlledMCPToolSurfaceV2ManifestDigest ||
		expected.AggregateSchemaBytes != ProductControlledMCPToolSurfaceV2AggregateSchemaBytes || expected.AggregateSchemaDigest != ProductControlledMCPToolSurfaceV2AggregateSchemaDigest ||
		surface.ToolCount != 8 || observed.ToolCount != 8 ||
		surface.ManifestDigest != ProductControlledMCPToolSurfaceV2ManifestDigest || observed.ManifestDigest != ProductControlledMCPToolSurfaceV2ManifestDigest ||
		surface.AggregateSchemaBytes != ProductControlledMCPToolSurfaceV2AggregateSchemaBytes || observed.AggregateSchemaBytes != ProductControlledMCPToolSurfaceV2AggregateSchemaBytes ||
		surface.AggregateSchemaDigest != ProductControlledMCPToolSurfaceV2AggregateSchemaDigest || observed.AggregateSchemaDigest != ProductControlledMCPToolSurfaceV2AggregateSchemaDigest {
		return fmt.Errorf("controlled-MCP v2 tool surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

// ValidateOfflineFakeSkillSurface accepts only the exact v5 schema in the
// deterministic fake runtime. Its marker is explicitly not provider evidence.
func ValidateOfflineFakeSkillSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductSkillToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductSkillToolSurfaceQualification || profile.Purpose != OfflineReadOnlySkillSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineSkillSurfaceSimulationMarker || profile.ProductProviderL2Fingerprint != OfflineSkillSurfaceSimulationMarker ||
		profile.ToolCallLimit <= 0 || expected.ManifestDigest != ProductSkillToolSurfaceV5ManifestDigest || expected.ToolCount != 8 ||
		expected.AggregateSchemaBytes != ProductSkillToolSurfaceV5AggregateSchemaBytes || expected.AggregateSchemaDigest != ProductSkillToolSurfaceV5AggregateSchemaDigest ||
		surface.ManifestDigest != ProductSkillToolSurfaceV5ManifestDigest || surface.ToolCount != 8 ||
		surface.AggregateSchemaBytes != ProductSkillToolSurfaceV5AggregateSchemaBytes || surface.AggregateSchemaDigest != ProductSkillToolSurfaceV5AggregateSchemaDigest ||
		observed.ManifestDigest != ProductSkillToolSurfaceV5ManifestDigest || observed.ToolCount != 8 ||
		observed.AggregateSchemaBytes != ProductSkillToolSurfaceV5AggregateSchemaBytes || observed.AggregateSchemaDigest != ProductSkillToolSurfaceV5AggregateSchemaDigest {
		return fmt.Errorf("read-only Skill surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

func ToolSurfaceFromTools(tools []any) ToolSurface {
	raw, _ := json.Marshal(tools)
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	schemaDigest, schemaBytes, err := AggregateToolSchemaMetadata(tools)
	if err != nil {
		schemaDigest = ""
		schemaBytes = 0
	}
	return ToolSurface{Tools: append([]any(nil), tools...), ToolCount: len(tools), ManifestDigest: digest, AggregateSchemaDigest: schemaDigest, AggregateSchemaBytes: schemaBytes}
}

func AggregateToolSchemaMetadata(tools []any) (string, int, error) {
	schemas := make([]json.RawMessage, 0, len(tools))
	totalBytes := 0
	for index, value := range tools {
		tool, ok := value.(map[string]any)
		if !ok {
			return "", 0, fmt.Errorf("tool %d is not an object", index+1)
		}
		schema, exists := tool["inputSchema"]
		if !exists || schema == nil {
			return "", 0, fmt.Errorf("tool %d is missing inputSchema", index+1)
		}
		raw, err := json.Marshal(schema)
		if err != nil {
			return "", 0, fmt.Errorf("tool %d inputSchema cannot be serialized: %w", index+1, err)
		}
		var object map[string]json.RawMessage
		if err = json.Unmarshal(raw, &object); err != nil || object == nil {
			return "", 0, fmt.Errorf("tool %d inputSchema is not an object", index+1)
		}
		schemas = append(schemas, json.RawMessage(raw))
		totalBytes += len(raw)
	}
	aggregate, err := json.Marshal(schemas)
	if err != nil {
		return "", 0, fmt.Errorf("aggregate inputSchema cannot be serialized: %w", err)
	}
	hash := sha256.Sum256(aggregate)
	return hex.EncodeToString(hash[:]), totalBytes, nil
}

type SessionStartOptions struct {
	SessionID    string
	Model        string
	Effort       string
	Profile      string
	MissionID    string
	TaskID       string
	ToolSurface  ToolSurface
	WorkerCgroup runner.WorkerProcessCgroup
}

type ThreadStartOptions struct {
	Model  string
	Effort string
	Tools  []any
}

type TurnResult struct {
	State                 string
	Outcome               string
	ToolCalls             int
	ProviderEgress        int
	ReconnectAttemptCount int
	ReconnectRecovered    bool
	// RetryVisibility is limited when the provider process can retry internally
	// without exposing those attempts through the Worker protocol.
	RetryVisibility       string
	RetryObservationScope string
	TurnCompleted         bool
	Usage                 codex.TokenUsage
	StartedAt             time.Time
	FinishedAt            time.Time
}

const RetryObservationCodexAppServerWillRetry = "codex_app_server_responseStreamDisconnected_willRetry"

type RuntimeStats struct {
	Reservations       int
	ActiveReservations int
	ClosedReservations int
	ProviderEgress     int
}

type ExecutionProfile struct {
	Model                            string
	Effort                           string
	Profile                          string
	ToolCallLimit                    int
	Purpose                          string
	ExactSurfaceExecutionFingerprint string
	ProductProviderL2Fingerprint     string
	ExecutionEnvelope                string
	ToolSurfaceQualification         string
	TransportPolicy                  codex.TransportPolicy
}

type Reservation struct {
	ID string
}

type Runtime interface {
	Mode() string
	Readiness(context.Context) error
	ToolSurface() ToolSurface
	ExecutionProfile() ExecutionProfile
	Reserve(context.Context, ExecutionAuthorization) (Reservation, error)
	CloseReservation(context.Context, Reservation, string) error
	Start(context.Context, SessionStartOptions) (Session, error)
	Stats() RuntimeStats
}

type LinuxWorkerCgroupRequirement interface {
	RequiresLinuxWorkerCgroup() bool
}

func validateRuntimeBinding(authorization ExecutionAuthorization, mode string, profile ExecutionProfile, surface ToolSurface) error {
	if err := ValidateRuntimeExecutionAuthorization(authorization); err != nil {
		return err
	}
	if authorization.ProviderMode != mode || authorization.Model != profile.Model || authorization.Effort != profile.Effort || authorization.Profile != profile.Profile || authorization.Purpose != profile.Purpose || authorization.ToolCallLimit != profile.ToolCallLimit || authorization.ToolSurfaceDigest != surface.ManifestDigest || authorization.ToolSurfaceQualification != profile.ToolSurfaceQualification || authorization.ToolCount != surface.ToolCount || authorization.AggregateSchemaBytes != surface.AggregateSchemaBytes || authorization.AggregateSchemaDigest != surface.AggregateSchemaDigest || authorization.ExactSurfaceExecutionFingerprint != profile.ExactSurfaceExecutionFingerprint || authorization.ProductProviderL2Fingerprint != profile.ProductProviderL2Fingerprint || authorization.ExecutionEnvelope != profile.ExecutionEnvelope || authorization.TransportPolicyRevision != profile.TransportPolicy.Revision {
		return fmt.Errorf("execution authorization does not match selected provider runtime")
	}
	return nil
}

type Session interface {
	Process() *runner.Process
	Initialize(context.Context, codex.TransportPolicy) error
	StartThread(context.Context, ThreadStartOptions) (string, error)
	Turn(context.Context, string, string, codex.TurnOptions, ToolHandler) (TurnResult, error)
	Stop(context.Context) (runner.StopProof, error)
}

type ProductSurfaceDiagnosticSession interface {
	Session
	StartDiagnosticThread(context.Context, ThreadStartOptions) (string, error)
}
