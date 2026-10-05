// pattern: Functional Core
package provider

import (
	"encoding/hex"
	"errors"
	"fmt"
	"polis/internal/core"
)

const (
	Live2AuthorizationPurpose                      = "R0.5B_REAL_PROVIDER_PRODUCT_SMOKE_LIVE_2"
	ProductReservationBridgeQualificationPurpose   = "R0.5B16_RESERVATION_TO_INITIALIZE_BRIDGE_QUALIFICATION"
	OfflineReadOnlySkillSurfacePurpose             = "offline-read-only-skill-surface"
	OfflineSkillSurfaceSimulationMarker            = "offline-skill-surface-unqualified"
	OfflineSkillDirectorySurfacePurpose            = "offline-skill-directory-surface"
	OfflineSkillDirectorySurfaceSimulationMarker   = "offline-skill-directory-surface-unqualified"
	ProductExactSurfaceExecutionFingerprint        = "e736d8298f0920c6dd81796006b2486881647f0fdbc66bb111bbd2387a1e99ff"
	ProductProviderL2Fingerprint                   = "59a4ac003fb0f49360382a707f7057edc03e0574ba328d0cf3a59eeab2726781"
	ProductProviderRuntimeEnvelopeFingerprintV2    = "676052c13ae0380250c4fcb9dbdb3cd8794167e214e6b3766e6e38b4d68bdc51"
	ProductProviderRuntimeVersionV2                = "0.154.0-alpha.6.2"
	ProductProviderBinarySHA256V2                  = "081e4de4be8e38fac6ed4d95e3b1a0b9f6d31c090ddc36e1696b349fe406f575"
	ProductProviderHelperSHA256V2                  = "fdf360c3a02adce2a29272357d61681bdddc2ab9828a5fa615c4528e4384a23e"
	ProductToolSurfaceV2ManifestDigest             = productSurfaceV2ManifestDigest
	ProductToolSurfaceV2AggregateSchemaBytes       = 1470
	ProductToolSurfaceV2AggregateSchemaDigest      = productSurfaceV2SchemaDigest
	ProductToolSurfaceV3ManifestDigest             = productSurfaceV3ManifestDigest
	ProductToolSurfaceV3AggregateSchemaBytes       = 2206
	ProductToolSurfaceV3AggregateSchemaDigest      = productSurfaceV3SchemaDigest
	ProductToolSurfaceV4ManifestDigest             = productSurfaceV4ManifestDigest
	ProductToolSurfaceV4AggregateSchemaBytes       = 2206
	ProductToolSurfaceV4AggregateSchemaDigest      = productSurfaceV4SchemaDigest
	ProductSkillToolSurfaceV5ManifestDigest        = "f1fe445f46675e7d1883c3b9360f0ad6bef168ce2f5a2ebc8ea97e9a1f28b2a8"
	ProductSkillToolSurfaceV5AggregateSchemaBytes  = 2459
	ProductSkillToolSurfaceV5AggregateSchemaDigest = "a43fb0039ccc3e2b654fdf2549ac22432f690f95eb16157ee8373e1e6d059ed0"
	ProductSkillDirectoryManifestDigest            = "7ad11ab318f15e124554709b8b5bfed2e87f60d4fcad07428def7e41fd1390e1"
	ProductSkillDirectorySchemaBytes               = 2724
	ProductSkillDirectorySchemaDigest              = "8ffefe6b5f3399dfb480ab9d77e063b4e407074d45c95a0cf4e85a0603ad788e"
)

var (
	errBusinessReservationRequired    = errors.New("business provider Start requires a prior one-use reservation")
	errBusinessReservationAlreadyUsed = errors.New("business provider reservation or Start attempt is already consumed")
)

type ExecutionAuthorization struct {
	CompanyID                        string
	MissionID                        string
	TaskID                           string
	TaskKind                         core.TaskKind
	TaskOwnerID                      string
	EmployeeID                       string
	EmployeeRole                     string
	SessionID                        string
	Epoch                            int64
	Incarnation                      string
	Model                            string
	Profile                          string
	Effort                           string
	ProviderMode                     string
	Purpose                          string
	ToolSurfaceDigest                string
	ToolSurfaceQualification         string
	ToolCount                        int
	AggregateSchemaBytes             int
	AggregateSchemaDigest            string
	ExactSurfaceExecutionFingerprint string
	ProductProviderL2Fingerprint     string
	TaskValidationBindingDigest      string
	WorkspaceDigest                  string
	WorkspaceRevision                int64
	ExecutionEnvelope                string
	TransportPolicyRevision          string
	ToolCallLimit                    int
}

func ValidateExecutionAuthorization(authorization ExecutionAuthorization) error {
	if err := validateExecutionAuthorizationShape(authorization); err != nil {
		return err
	}
	if authorization.ToolSurfaceQualification != ProductToolSurfaceQualification || authorization.ToolSurfaceDigest != ProductToolSurfaceV4ManifestDigest || authorization.ToolCount != 7 || authorization.AggregateSchemaBytes != ProductToolSurfaceV4AggregateSchemaBytes || authorization.AggregateSchemaDigest != ProductToolSurfaceV4AggregateSchemaDigest || authorization.ExactSurfaceExecutionFingerprint != ProductExactSurfaceExecutionFingerprint || authorization.ProductProviderL2Fingerprint != ProductProviderL2Fingerprint {
		return fmt.Errorf("execution authorization does not match the currently qualified product surface")
	}
	return nil
}

// ValidateRuntimeExecutionAuthorization keeps the real-provider gate pinned to
// v4. The v5 Skill surface is accepted only by the zero-egress fake runtime,
// using an explicit marker that cannot be mistaken for provider qualification.
func ValidateRuntimeExecutionAuthorization(authorization ExecutionAuthorization) error {
	if authorization.ProviderMode == "fake" && authorization.ToolSurfaceQualification == ProductCSVInputRangeToolSurfaceQualification {
		if err := validateExecutionAuthorizationShape(authorization); err != nil {
			return err
		}
		profile := ExecutionProfile{
			ToolCallLimit: authorization.ToolCallLimit, Purpose: authorization.Purpose,
			ExecutionEnvelope:                authorization.ExecutionEnvelope,
			ToolSurfaceQualification:         authorization.ToolSurfaceQualification,
			ExactSurfaceExecutionFingerprint: authorization.ExactSurfaceExecutionFingerprint,
			ProductProviderL2Fingerprint:     authorization.ProductProviderL2Fingerprint,
		}
		expected := ProductCSVInputRangeToolSurface()
		return ValidateOfflineFakeCSVInputRangeSurface(authorization.ProviderMode, profile, ToolSurface{
			ToolCount: authorization.ToolCount, ManifestDigest: authorization.ToolSurfaceDigest,
			AggregateSchemaBytes: authorization.AggregateSchemaBytes, AggregateSchemaDigest: authorization.AggregateSchemaDigest,
			Tools: expected.Tools,
		})
	}
	if authorization.ProviderMode == "fake" && authorization.ToolSurfaceQualification == ProductMissionChangeAssessmentToolSurfaceQualification {
		if err := validateExecutionAuthorizationShape(authorization); err != nil {
			return err
		}
		profile := ExecutionProfile{
			ToolCallLimit: authorization.ToolCallLimit, Purpose: authorization.Purpose,
			ExecutionEnvelope:                authorization.ExecutionEnvelope,
			ToolSurfaceQualification:         authorization.ToolSurfaceQualification,
			ExactSurfaceExecutionFingerprint: authorization.ExactSurfaceExecutionFingerprint,
			ProductProviderL2Fingerprint:     authorization.ProductProviderL2Fingerprint,
		}
		return ValidateOfflineFakeMissionChangeAssessmentSurface(authorization.ProviderMode, profile, ToolSurface{
			ToolCount: authorization.ToolCount, ManifestDigest: authorization.ToolSurfaceDigest,
			AggregateSchemaBytes: authorization.AggregateSchemaBytes, AggregateSchemaDigest: authorization.AggregateSchemaDigest,
			Tools: ProductMissionChangeAssessmentToolSurface().Tools,
		})
	}
	if authorization.ProviderMode == "fake" && authorization.ToolSurfaceQualification == ProductWorkspaceSnapshotRevocationToolSurfaceQualification {
		if err := validateExecutionAuthorizationShape(authorization); err != nil {
			return err
		}
		profile := ExecutionProfile{
			ToolCallLimit: authorization.ToolCallLimit, Purpose: authorization.Purpose,
			ExecutionEnvelope:                authorization.ExecutionEnvelope,
			ToolSurfaceQualification:         authorization.ToolSurfaceQualification,
			ExactSurfaceExecutionFingerprint: authorization.ExactSurfaceExecutionFingerprint,
			ProductProviderL2Fingerprint:     authorization.ProductProviderL2Fingerprint,
		}
		return ValidateOfflineFakeWorkspaceSnapshotRevocationSurface(authorization.ProviderMode, profile, ToolSurface{
			ToolCount: authorization.ToolCount, ManifestDigest: authorization.ToolSurfaceDigest,
			AggregateSchemaBytes: authorization.AggregateSchemaBytes, AggregateSchemaDigest: authorization.AggregateSchemaDigest,
			Tools: ProductWorkspaceSnapshotRevocationToolSurface().Tools,
		})
	}
	if authorization.ProviderMode == "fake" && authorization.ToolSurfaceQualification == ProductWorkspaceTreeToolSurfaceQualification {
		if err := validateExecutionAuthorizationShape(authorization); err != nil {
			return err
		}
		profile := ExecutionProfile{
			ToolCallLimit: authorization.ToolCallLimit, Purpose: authorization.Purpose,
			ExecutionEnvelope:                authorization.ExecutionEnvelope,
			ToolSurfaceQualification:         authorization.ToolSurfaceQualification,
			ExactSurfaceExecutionFingerprint: authorization.ExactSurfaceExecutionFingerprint,
			ProductProviderL2Fingerprint:     authorization.ProductProviderL2Fingerprint,
		}
		return ValidateOfflineFakeWorkspaceTreeSurface(authorization.ProviderMode, profile, ToolSurface{
			ToolCount: authorization.ToolCount, ManifestDigest: authorization.ToolSurfaceDigest,
			AggregateSchemaBytes: authorization.AggregateSchemaBytes, AggregateSchemaDigest: authorization.AggregateSchemaDigest,
			Tools: ProductWorkspaceTreeToolSurface().Tools,
		})
	}
	if authorization.ProviderMode == "fake" && authorization.ToolSurfaceQualification == ProductSkillDirectoryToolSurfaceQualification {
		if err := validateExecutionAuthorizationShape(authorization); err != nil {
			return err
		}
		profile := ExecutionProfile{
			ToolCallLimit: authorization.ToolCallLimit, Purpose: authorization.Purpose,
			ExecutionEnvelope:                authorization.ExecutionEnvelope,
			ToolSurfaceQualification:         authorization.ToolSurfaceQualification,
			ExactSurfaceExecutionFingerprint: authorization.ExactSurfaceExecutionFingerprint,
			ProductProviderL2Fingerprint:     authorization.ProductProviderL2Fingerprint,
		}
		return ValidateOfflineFakeSkillDirectorySurface(authorization.ProviderMode, profile, ToolSurface{
			ToolCount: authorization.ToolCount, ManifestDigest: authorization.ToolSurfaceDigest,
			AggregateSchemaBytes: authorization.AggregateSchemaBytes, AggregateSchemaDigest: authorization.AggregateSchemaDigest,
			Tools: ProductSkillDirectoryToolSurface().Tools,
		})
	}
	if authorization.ProviderMode == "fake" && authorization.ToolSurfaceQualification == ProductSharedMissionArtifactToolSurfaceQualification {
		if err := validateExecutionAuthorizationShape(authorization); err != nil {
			return err
		}
		profile := ExecutionProfile{
			ToolCallLimit: authorization.ToolCallLimit, Purpose: authorization.Purpose,
			ExecutionEnvelope:                authorization.ExecutionEnvelope,
			ToolSurfaceQualification:         authorization.ToolSurfaceQualification,
			ExactSurfaceExecutionFingerprint: authorization.ExactSurfaceExecutionFingerprint,
			ProductProviderL2Fingerprint:     authorization.ProductProviderL2Fingerprint,
		}
		return ValidateOfflineFakeSharedMissionArtifactSurface(authorization.ProviderMode, profile, ToolSurface{
			ToolCount: authorization.ToolCount, ManifestDigest: authorization.ToolSurfaceDigest,
			AggregateSchemaBytes: authorization.AggregateSchemaBytes, AggregateSchemaDigest: authorization.AggregateSchemaDigest,
			Tools: ProductSharedMissionArtifactToolSurface().Tools,
		})
	}
	if authorization.ProviderMode == "fake" && authorization.ToolSurfaceQualification == ProductDirectMessagingToolSurfaceQualification {
		if err := validateExecutionAuthorizationShape(authorization); err != nil {
			return err
		}
		profile := ExecutionProfile{
			ToolCallLimit: authorization.ToolCallLimit, Purpose: authorization.Purpose,
			ExecutionEnvelope:                authorization.ExecutionEnvelope,
			ToolSurfaceQualification:         authorization.ToolSurfaceQualification,
			ExactSurfaceExecutionFingerprint: authorization.ExactSurfaceExecutionFingerprint,
			ProductProviderL2Fingerprint:     authorization.ProductProviderL2Fingerprint,
		}
		return ValidateOfflineFakeProductDirectMessagingSurface(authorization.ProviderMode, profile, ToolSurface{
			ToolCount: authorization.ToolCount, ManifestDigest: authorization.ToolSurfaceDigest,
			AggregateSchemaBytes: authorization.AggregateSchemaBytes, AggregateSchemaDigest: authorization.AggregateSchemaDigest,
			Tools: ProductDirectMessagingToolSurface().Tools,
		})
	}
	if authorization.ProviderMode == "fake" && authorization.ToolSurfaceQualification == ProductControlledMCPToolSurfaceV2Qualification {
		if err := validateExecutionAuthorizationShape(authorization); err != nil {
			return err
		}
		profile := ExecutionProfile{
			ToolCallLimit: authorization.ToolCallLimit, Purpose: authorization.Purpose,
			ExecutionEnvelope:                authorization.ExecutionEnvelope,
			ToolSurfaceQualification:         authorization.ToolSurfaceQualification,
			ExactSurfaceExecutionFingerprint: authorization.ExactSurfaceExecutionFingerprint,
			ProductProviderL2Fingerprint:     authorization.ProductProviderL2Fingerprint,
		}
		return ValidateOfflineFakeControlledMCPSurfaceV2(authorization.ProviderMode, profile, ToolSurface{
			ToolCount: authorization.ToolCount, ManifestDigest: authorization.ToolSurfaceDigest,
			AggregateSchemaBytes: authorization.AggregateSchemaBytes, AggregateSchemaDigest: authorization.AggregateSchemaDigest,
			Tools: ProductControlledMCPToolSurfaceV2().Tools,
		})
	}
	if authorization.ProviderMode == "fake" && authorization.ToolSurfaceQualification == ProductControlledMCPToolSurfaceQualification {
		if err := validateExecutionAuthorizationShape(authorization); err != nil {
			return err
		}
		profile := ExecutionProfile{
			ToolCallLimit: authorization.ToolCallLimit, Purpose: authorization.Purpose,
			ExecutionEnvelope:                authorization.ExecutionEnvelope,
			ToolSurfaceQualification:         authorization.ToolSurfaceQualification,
			ExactSurfaceExecutionFingerprint: authorization.ExactSurfaceExecutionFingerprint,
			ProductProviderL2Fingerprint:     authorization.ProductProviderL2Fingerprint,
		}
		return ValidateOfflineFakeControlledMCPSurface(authorization.ProviderMode, profile, ToolSurface{
			ToolCount: authorization.ToolCount, ManifestDigest: authorization.ToolSurfaceDigest,
			AggregateSchemaBytes: authorization.AggregateSchemaBytes, AggregateSchemaDigest: authorization.AggregateSchemaDigest,
			Tools: ProductControlledMCPToolSurface().Tools,
		})
	}
	if authorization.ProviderMode != "fake" || authorization.ToolSurfaceQualification != ProductSkillToolSurfaceQualification {
		return ValidateExecutionAuthorization(authorization)
	}
	if err := validateExecutionAuthorizationShape(authorization); err != nil {
		return err
	}
	surface := ProductSkillToolSurface()
	if surface.ManifestDigest != ProductSkillToolSurfaceV5ManifestDigest || surface.ToolCount != 8 ||
		surface.AggregateSchemaBytes != ProductSkillToolSurfaceV5AggregateSchemaBytes || surface.AggregateSchemaDigest != ProductSkillToolSurfaceV5AggregateSchemaDigest ||
		authorization.Purpose != OfflineReadOnlySkillSurfacePurpose || authorization.ExecutionEnvelope != OfflineExecutionEnvelope ||
		authorization.ToolSurfaceDigest != ProductSkillToolSurfaceV5ManifestDigest || authorization.ToolCount != 8 ||
		authorization.AggregateSchemaBytes != ProductSkillToolSurfaceV5AggregateSchemaBytes || authorization.AggregateSchemaDigest != ProductSkillToolSurfaceV5AggregateSchemaDigest ||
		authorization.ExactSurfaceExecutionFingerprint != OfflineSkillSurfaceSimulationMarker || authorization.ProductProviderL2Fingerprint != OfflineSkillSurfaceSimulationMarker {
		return fmt.Errorf("offline Skill surface authorization is not bound to its explicit zero-egress simulation")
	}
	return nil
}

func validateExecutionAuthorizationShape(authorization ExecutionAuthorization) error {
	for name, value := range map[string]string{
		"company":                         authorization.CompanyID,
		"mission":                         authorization.MissionID,
		"task":                            authorization.TaskID,
		"task kind":                       string(authorization.TaskKind),
		"task owner":                      authorization.TaskOwnerID,
		"employee":                        authorization.EmployeeID,
		"employee role":                   authorization.EmployeeRole,
		"session":                         authorization.SessionID,
		"incarnation":                     authorization.Incarnation,
		"model":                           authorization.Model,
		"profile":                         authorization.Profile,
		"effort":                          authorization.Effort,
		"provider mode":                   authorization.ProviderMode,
		"purpose":                         authorization.Purpose,
		"tool surface":                    authorization.ToolSurfaceDigest,
		"tool surface qualification":      authorization.ToolSurfaceQualification,
		"aggregate schema digest":         authorization.AggregateSchemaDigest,
		"exact surface fingerprint":       authorization.ExactSurfaceExecutionFingerprint,
		"product provider L2 fingerprint": authorization.ProductProviderL2Fingerprint,
		"TaskValidationBinding digest":    authorization.TaskValidationBindingDigest,
		"workspace digest":                authorization.WorkspaceDigest,
		"execution envelope":              authorization.ExecutionEnvelope,
		"transport policy":                authorization.TransportPolicyRevision,
	} {
		if value == "" {
			return fmt.Errorf("execution authorization missing %s", name)
		}
	}
	if authorization.Epoch <= 0 || authorization.ToolCallLimit <= 0 || authorization.WorkspaceRevision <= 0 {
		return fmt.Errorf("execution authorization has invalid epoch, workspace revision, or tool budget")
	}
	if authorization.Purpose == Live2AuthorizationPurpose && authorization.ExecutionEnvelope != ProductProviderRuntimeEnvelopeFingerprintV2 {
		return fmt.Errorf("LIVE_2 execution authorization does not match the qualified provider runtime envelope")
	}
	if !validAuthorizationDigest(authorization.TaskValidationBindingDigest) || !validAuthorizationDigest(authorization.WorkspaceDigest) {
		return fmt.Errorf("execution authorization has an invalid TaskValidationBinding or workspace digest")
	}
	if !core.IsProductProviderExecutableTask(authorization.TaskKind, authorization.TaskOwnerID) || authorization.EmployeeID != authorization.TaskOwnerID || authorization.EmployeeRole != "backend" {
		return fmt.Errorf("execution authorization is not bound to a provider-executable product Task")
	}
	if authorization.Effort != "medium" {
		return fmt.Errorf("execution authorization effort is not the approved medium profile")
	}
	return nil
}

func validAuthorizationDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func ValidateExecutionAuthorizationMatch(bound, expected ExecutionAuthorization) error {
	if err := ValidateExecutionAuthorization(bound); err != nil {
		return err
	}
	if err := ValidateExecutionAuthorization(expected); err != nil {
		return err
	}
	if bound != expected {
		return fmt.Errorf("execution authorization does not match authoritative product context")
	}
	return nil
}
