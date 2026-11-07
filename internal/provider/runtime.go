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
	SessionID   string
	Model       string
	Effort      string
	Profile     string
	MissionID   string
	TaskID      string
	ToolSurface ToolSurface
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
	TurnCompleted         bool
	Usage                 codex.TokenUsage
	StartedAt             time.Time
	FinishedAt            time.Time
}

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

func validateRuntimeBinding(authorization ExecutionAuthorization, mode string, profile ExecutionProfile, surface ToolSurface) error {
	if err := ValidateExecutionAuthorization(authorization); err != nil {
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
