// pattern: Functional Core
package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"polis/internal/codex"
	"polis/internal/runner"
)

const (
	ProductSurfaceExecutionIdentitySchema         = "polis-product-exact-surface-execution@1"
	ProductSurfaceDiagnosticAuthorizationSchema   = "polis-product-surface-diagnostic-authorization@1"
	ProductSurfaceDiagnosticReservationSchema     = "polis-product-surface-diagnostic-reservation@1"
	ProductProviderL2EvidenceSchema               = "polis-product-provider-l2-evidence@1"
	ProductSurfaceDiagnosticPurpose               = "PRODUCT_SURFACE_V4_LIVE_QUALIFICATION"
	ProductSurfaceDiagnosticAttempt2Purpose       = "PRODUCT_SURFACE_V4_LIVE_QUALIFICATION_ATTEMPT_2"
	ProductSurfaceDiagnosticAttempt3Purpose       = "PRODUCT_SURFACE_V4_LIVE_QUALIFICATION_ATTEMPT_3"
	ProductLocalProcessLaunchQualificationPurpose = "R0.5B13_LOCAL_PROCESS_LAUNCH_QUALIFICATION"
	ProductSurfaceDiagnosticInstructionRevision   = "product-surface-v4-diagnostic-instructions@1"
	ProductSurfaceDiagnosticCanaryOutput          = "POLIS_PRODUCT_SURFACE_V4_CANARY_OK"
	ProductSurfaceDiagnosticPrompt                = "Respond with exactly POLIS_PRODUCT_SURFACE_V4_CANARY_OK."
	ProductProviderRuntimeImplementation          = "codex-app-server"
	ProductProviderProtocolCompatibility          = "codex-app-server-stdio/initialize-thread-start@1"
)

const (
	productSurfaceV2ManifestDigest = "2b403fc0f3c9a1513473828e66203becb7ac5202f298f917034fd95929a9f3b9"
	productSurfaceV2SchemaDigest   = "8f2e1ee8ba8c8d456af9ce9839f62a854bfe7c9f5847613657ccd7f77a9da04f"
	productSurfaceV3ManifestDigest = "95d4f2e2b1551096f05c7683786ec09a2d75189f8411fc66ffc759514de32257"
	productSurfaceV3SchemaDigest   = "5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc"
	productSurfaceV4ManifestDigest = "60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4"
	productSurfaceV4SchemaDigest   = productSurfaceV3SchemaDigest
)

type ProductProviderRuntimeIdentity struct {
	RuntimeImplementation string `json:"runtime_implementation"`
	NativeVersion         string `json:"native_version"`
	BinarySHA256          string `json:"binary_sha256"`
	HelperSHA256          string `json:"helper_sha256"`
	ProtocolCompatibility string `json:"protocol_compatibility"`
}

type ProductSurfaceRuntimePolicy struct {
	NativeConfigSHA256     string `json:"native_config_sha256"`
	NativeTransportPolicy  string `json:"native_transport_policy"`
	ApprovalPolicy         string `json:"approval_policy"`
	SandboxPolicy          string `json:"sandbox_policy"`
	ThreadWorkingDirectory string `json:"thread_working_directory"`
	ThreadEphemeral        bool   `json:"thread_ephemeral"`
	ModelFallbackAllowed   bool   `json:"model_fallback_allowed"`
}

type ProductSurfaceExecutionIdentity struct {
	SchemaVersion                 string                         `json:"schema_version"`
	SurfaceID                     string                         `json:"surface_id"`
	ToolCount                     int                            `json:"tool_count"`
	ManifestDigest                string                         `json:"manifest_digest"`
	AggregateSchemaBytes          int                            `json:"aggregate_schema_bytes"`
	AggregateSchemaDigest         string                         `json:"aggregate_schema_digest"`
	Model                         string                         `json:"model"`
	Effort                        string                         `json:"effort"`
	Profile                       string                         `json:"profile"`
	ProviderMode                  string                         `json:"provider_mode"`
	ProviderRuntime               ProductProviderRuntimeIdentity `json:"provider_runtime"`
	LaunchEnvelope                runner.NativeLaunchEnvelope    `json:"launch_runtime_envelope"`
	RuntimePolicy                 ProductSurfaceRuntimePolicy    `json:"runtime_policy"`
	TransportPolicy               codex.TransportPolicySnapshot  `json:"transport_policy"`
	DiagnosticInstructionRevision string                         `json:"diagnostic_instruction_revision"`
}

type ProductSurfaceDiagnosticAuthorization struct {
	SchemaVersion              string                          `json:"schema_version"`
	AuthorizationID            string                          `json:"authorization_id"`
	Purpose                    string                          `json:"purpose"`
	ProviderSessionID          string                          `json:"provider_session_id"`
	ExecutionIdentity          ProductSurfaceExecutionIdentity `json:"execution_identity"`
	ExecutionFingerprint       string                          `json:"product_exact_surface_execution_fingerprint"`
	OfflineQualificationStatus string                          `json:"offline_qualification_status"`
	OfflineQualificationDigest string                          `json:"offline_qualification_digest"`
	ReservationLimit           int                             `json:"diagnostic_reservation_limit"`
	ProviderAttemptLimit       int                             `json:"provider_attempt_limit"`
	ProviderEgressLimit        int                             `json:"provider_egress_limit"`
	MediumLimit                int                             `json:"medium_limit"`
	BusinessAllowance          int                             `json:"business_allowance"`
	HighLimit                  int                             `json:"high_limit"`
	RetryLimit                 int                             `json:"retry_limit"`
	ExpectedToolCalls          int                             `json:"expected_tool_calls"`
	MaximumToolCallEvents      int                             `json:"maximum_tool_call_events"`
	BusinessObjectsAllowed     bool                            `json:"business_objects_allowed"`
	IssuedAt                   time.Time                       `json:"issued_at"`
}

type ProductSurfaceDiagnosticReservation struct {
	SchemaVersion        string    `json:"schema_version"`
	AuthorizationID      string    `json:"authorization_id"`
	AuthorizationDigest  string    `json:"authorization_digest"`
	ReservationNumber    int       `json:"reservation_number"`
	ProviderAttemptLimit int       `json:"provider_attempt_limit"`
	RetryLimit           int       `json:"retry_limit"`
	ReservedAt           time.Time `json:"reserved_at"`
}

type ProductProviderL2Evidence struct {
	SchemaVersion                string                          `json:"schema_version"`
	ExecutionIdentity            ProductSurfaceExecutionIdentity `json:"execution_identity"`
	ExecutionFingerprint         string                          `json:"product_exact_surface_execution_fingerprint"`
	DiagnosticAuthorizationID    string                          `json:"diagnostic_authorization_id"`
	OfflineQualificationDigest   string                          `json:"offline_qualification_digest"`
	DiagnosticReservationCount   int                             `json:"diagnostic_reservation_count"`
	ProviderEgress               int                             `json:"provider_egress"`
	RegisteredToolCount          int                             `json:"registered_tool_count"`
	FirstOutput                  string                          `json:"first_output"`
	ToolCalls                    int                             `json:"tool_calls"`
	ReconnectCount               int                             `json:"reconnect_count"`
	TurnTerminal                 string                          `json:"turn_terminal"`
	CleanStop                    bool                            `json:"clean_stop"`
	CompanyCount                 int                             `json:"company_count"`
	MissionCount                 int                             `json:"mission_count"`
	TaskCount                    int                             `json:"task_count"`
	WorkerSessionCount           int                             `json:"worker_session_count"`
	SuccessorCount               int                             `json:"successor_count"`
	BusinessMutationCount        int                             `json:"business_mutation_count"`
	AggregateManifestDigestAfter string                          `json:"aggregate_manifest_digest_after"`
	AggregateSchemaDigestAfter   string                          `json:"aggregate_schema_digest_after"`
	FirstOutputLatencyMS         int64                           `json:"first_output_latency_ms"`
	ElapsedMS                    int64                           `json:"elapsed_ms"`
	Usage                        codex.TokenUsage                `json:"usage"`
	SecondReservationDenied      bool                            `json:"second_reservation_denied"`
}

func ComputeProductSurfaceExecutionFingerprint(identity ProductSurfaceExecutionIdentity) (string, error) {
	if err := ValidateProductSurfaceExecutionIdentity(identity); err != nil {
		return "", err
	}
	canonical := identity
	canonical.LaunchEnvelope.WorkingDirectory = ""
	canonical.LaunchEnvelope.Home = ""
	canonical.LaunchEnvelope.CodexHome = ""
	canonical.LaunchEnvelope.RuntimeRoot = ""
	canonical.LaunchEnvelope.RuntimeHome = ""
	canonical.LaunchEnvelope.TempDirectory = ""
	canonical.LaunchEnvelope.EvidenceDirectory = ""
	raw, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("marshal product execution identity: %w", err)
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func ValidateProductSurfaceExecutionIdentity(identity ProductSurfaceExecutionIdentity) error {
	if identity.SchemaVersion != ProductSurfaceExecutionIdentitySchema || identity.SurfaceID != ProductToolSurfaceQualification || identity.ToolCount != 7 || identity.ManifestDigest != productSurfaceV4ManifestDigest || identity.AggregateSchemaBytes != 2206 || identity.AggregateSchemaDigest != productSurfaceV4SchemaDigest {
		return errors.New("product execution identity does not bind the current exact tool surface")
	}
	if identity.Model != "gpt-5.6-luna" || identity.Effort != "medium" || identity.Profile != identity.Model+"/"+identity.Effort || identity.ProviderMode != "real" {
		return errors.New("product execution identity does not bind the approved diagnostic profile")
	}
	if identity.ProviderRuntime.RuntimeImplementation != ProductProviderRuntimeImplementation || identity.ProviderRuntime.NativeVersion == "" || !isDiagnosticSHA256(identity.ProviderRuntime.BinarySHA256) || !isDiagnosticSHA256(identity.ProviderRuntime.HelperSHA256) || identity.ProviderRuntime.ProtocolCompatibility != ProductProviderProtocolCompatibility {
		return errors.New("product execution identity has an invalid provider runtime identity")
	}
	launch := identity.LaunchEnvelope
	if launch.SchemaVersion != runner.NativeLaunchEnvelopeSchema || launch.LaunchMode == "" || launch.RuntimeOS == "" || launch.ProcessExecutable == "" || launch.BinaryPath == "" || launch.HelperPath == "" || launch.BinarySHA256 != identity.ProviderRuntime.BinarySHA256 || launch.HelperSHA256 != identity.ProviderRuntime.HelperSHA256 || len(launch.Argv) == 0 || launch.Stdio == "" || launch.Boundary == "" || launch.EnvironmentPolicy != runner.ControlledEnvironmentPolicy || launch.InheritedEnvironmentPolicy != runner.InheritedEnvironmentPolicy || len(launch.Environment) == 0 || launch.StdinMode == "" || launch.StdoutMode == "" || launch.StderrMode == "" || launch.PipeSetupMode == "" || len(launch.ProcessCreationFlags) == 0 || launch.ProcessGroupPolicy == "" || launch.JobObjectPolicy == "" || launch.RuntimeRootRule == "" || launch.RuntimeHomeRule == "" || launch.TempDirectoryRule == "" || launch.EvidenceRootRule == "" || !isDiagnosticSHA256(launch.Fingerprint) {
		return errors.New("product execution identity has an incomplete native launch envelope")
	}
	policy := identity.TransportPolicy
	if policy.Revision == "" || policy.InitializeTimeoutMS <= 0 || policy.StartAcknowledgementMS <= 0 || policy.FirstOutputDeadlineMS <= 0 || policy.ReconnectGraceMS <= 0 || policy.StreamingIdleMS <= 0 || policy.TotalTurnDeadlineMS <= 0 || policy.ReconciliationMS <= 0 {
		return errors.New("product execution identity has an incomplete transport policy")
	}
	launchPolicy := launch.TransportPolicy
	if launchPolicy.Revision != policy.Revision || launchPolicy.InitializeTimeoutMS != policy.InitializeTimeoutMS || launchPolicy.StartAcknowledgementMS != policy.StartAcknowledgementMS || launchPolicy.FirstOutputDeadlineMS != policy.FirstOutputDeadlineMS || launchPolicy.ReconnectGraceMS != policy.ReconnectGraceMS || launchPolicy.StreamingIdleMS != policy.StreamingIdleMS || launchPolicy.TotalTurnDeadlineMS != policy.TotalTurnDeadlineMS || launchPolicy.ReconciliationMS != policy.ReconciliationMS {
		return errors.New("launch envelope transport policy does not match product execution identity")
	}
	runtimePolicy := identity.RuntimePolicy
	if runtimePolicy.NativeConfigSHA256 != digestHex([]byte(runner.NativeDefaultConfig)) || runtimePolicy.NativeTransportPolicy != string(runner.NativeTransportPolicyNativeDefault) || runtimePolicy.ApprovalPolicy != "never" || runtimePolicy.SandboxPolicy != "read-only" || runtimePolicy.ThreadWorkingDirectory != "/work" || !runtimePolicy.ThreadEphemeral || runtimePolicy.ModelFallbackAllowed {
		return errors.New("product execution identity runtime policy is not the approved read-only diagnostic envelope")
	}
	if identity.DiagnosticInstructionRevision != ProductSurfaceDiagnosticInstructionRevision {
		return errors.New("product execution identity diagnostic instructions are not current")
	}
	return nil
}

func NewProductSurfaceDiagnosticAuthorization(authorizationID, providerSessionID string, identity ProductSurfaceExecutionIdentity, offlineQualificationDigest string, issuedAt time.Time) (ProductSurfaceDiagnosticAuthorization, error) {
	return NewProductSurfaceDiagnosticAuthorizationForPurpose(authorizationID, providerSessionID, identity, offlineQualificationDigest, issuedAt, ProductSurfaceDiagnosticPurpose)
}

func NewProductSurfaceDiagnosticAuthorizationForPurpose(authorizationID, providerSessionID string, identity ProductSurfaceExecutionIdentity, offlineQualificationDigest string, issuedAt time.Time, purpose string) (ProductSurfaceDiagnosticAuthorization, error) {
	fingerprint, err := ComputeProductSurfaceExecutionFingerprint(identity)
	if err != nil {
		return ProductSurfaceDiagnosticAuthorization{}, err
	}
	authorization := ProductSurfaceDiagnosticAuthorization{
		SchemaVersion:   ProductSurfaceDiagnosticAuthorizationSchema,
		AuthorizationID: authorizationID, Purpose: purpose, ProviderSessionID: providerSessionID,
		ExecutionIdentity: identity, ExecutionFingerprint: fingerprint,
		OfflineQualificationStatus: "PASSED", OfflineQualificationDigest: offlineQualificationDigest,
		ReservationLimit: 1, ProviderAttemptLimit: 1, ProviderEgressLimit: 1,
		MediumLimit: 1, BusinessAllowance: 0, HighLimit: 0, RetryLimit: 0,
		ExpectedToolCalls: 0, MaximumToolCallEvents: 1, BusinessObjectsAllowed: false,
		IssuedAt: issuedAt.UTC(),
	}
	if err = ValidateProductSurfaceDiagnosticAuthorization(authorization, authorization); err != nil {
		return ProductSurfaceDiagnosticAuthorization{}, err
	}
	return authorization, nil
}

func ValidateProductSurfaceDiagnosticAuthorization(bound, expected ProductSurfaceDiagnosticAuthorization) error {
	if bound.SchemaVersion != ProductSurfaceDiagnosticAuthorizationSchema || !isProductSurfaceDiagnosticPurpose(bound.Purpose) || bound.AuthorizationID == "" || bound.ProviderSessionID == "" || bound.OfflineQualificationStatus != "PASSED" || !isDiagnosticSHA256(bound.OfflineQualificationDigest) || bound.IssuedAt.IsZero() {
		return errors.New("invalid or incomplete product surface diagnostic authorization")
	}
	if bound.ReservationLimit != 1 || bound.ProviderAttemptLimit != 1 || bound.ProviderEgressLimit != 1 || bound.MediumLimit != 1 || bound.BusinessAllowance != 0 || bound.HighLimit != 0 || bound.RetryLimit != 0 || bound.ExpectedToolCalls != 0 || bound.MaximumToolCallEvents != 1 || bound.BusinessObjectsAllowed {
		return errors.New("product surface diagnostic authorization exceeds its one-turn zero-business scope")
	}
	if err := ValidateProductSurfaceExecutionIdentity(bound.ExecutionIdentity); err != nil {
		return err
	}
	fingerprint, err := ComputeProductSurfaceExecutionFingerprint(bound.ExecutionIdentity)
	if err != nil {
		return err
	}
	if bound.ExecutionFingerprint != fingerprint {
		return errors.New("product surface diagnostic execution fingerprint mismatch")
	}
	if !reflect.DeepEqual(bound, expected) {
		return errors.New("product surface diagnostic authorization does not match the current exact binding")
	}
	return nil
}

func isProductSurfaceDiagnosticPurpose(purpose string) bool {
	return purpose == ProductSurfaceDiagnosticPurpose || purpose == ProductSurfaceDiagnosticAttempt2Purpose || purpose == ProductSurfaceDiagnosticAttempt3Purpose
}

func ComputeProductProviderL2Fingerprint(evidence ProductProviderL2Evidence) (string, error) {
	if evidence.SchemaVersion != ProductProviderL2EvidenceSchema || evidence.DiagnosticAuthorizationID == "" || !isDiagnosticSHA256(evidence.OfflineQualificationDigest) {
		return "", errors.New("product provider L2 evidence is incomplete")
	}
	if err := ValidateProductSurfaceExecutionIdentity(evidence.ExecutionIdentity); err != nil {
		return "", err
	}
	expectedExecution, err := ComputeProductSurfaceExecutionFingerprint(evidence.ExecutionIdentity)
	if err != nil {
		return "", err
	}
	if evidence.ExecutionFingerprint != expectedExecution {
		return "", errors.New("product provider L2 evidence execution fingerprint mismatch")
	}
	if evidence.DiagnosticReservationCount != 1 || evidence.ProviderEgress != 1 || evidence.RegisteredToolCount != 7 || evidence.FirstOutput != ProductSurfaceDiagnosticCanaryOutput || evidence.ToolCalls != 0 || evidence.ReconnectCount != 0 || evidence.TurnTerminal != "completed" || !evidence.CleanStop {
		return "", errors.New("product provider L2 evidence does not meet the live canary pass boundary")
	}
	if !evidence.SecondReservationDenied {
		return "", errors.New("product provider L2 evidence is missing the duplicate-reservation denial")
	}
	if evidence.CompanyCount != 0 || evidence.MissionCount != 0 || evidence.TaskCount != 0 || evidence.WorkerSessionCount != 0 || evidence.SuccessorCount != 0 || evidence.BusinessMutationCount != 0 {
		return "", errors.New("product provider L2 evidence contains business side effects")
	}
	if evidence.AggregateManifestDigestAfter != evidence.ExecutionIdentity.ManifestDigest || evidence.AggregateSchemaDigestAfter != evidence.ExecutionIdentity.AggregateSchemaDigest || evidence.FirstOutputLatencyMS < 0 || evidence.ElapsedMS <= 0 {
		return "", errors.New("product provider L2 evidence is stale or missing timing evidence")
	}
	raw, err := json.Marshal(evidence)
	if err != nil {
		return "", fmt.Errorf("marshal product provider L2 evidence: %w", err)
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func digestHex(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func isDiagnosticSHA256(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}
