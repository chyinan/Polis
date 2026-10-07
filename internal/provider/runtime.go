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

// ProductSkillDirectoryToolSurface adds bounded exact-Skill path pages while
// keeping the v5 Skill-load registry immutable.
const ProductSkillDirectoryToolSurfaceQualification = "polis-product-tool-surface@9"

func ProductSkillDirectoryToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithBoundedSkillDirectory())
}

func ValidateOfflineFakeSkillDirectorySurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductSkillDirectoryToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductSkillDirectoryToolSurfaceQualification ||
		profile.Purpose != OfflineSkillDirectorySurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineSkillDirectorySurfaceSimulationMarker ||
		profile.ProductProviderL2Fingerprint != OfflineSkillDirectorySurfaceSimulationMarker || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 9 || expected.ManifestDigest != ProductSkillDirectoryManifestDigest ||
		expected.AggregateSchemaBytes != ProductSkillDirectorySchemaBytes || expected.AggregateSchemaDigest != ProductSkillDirectorySchemaDigest ||
		surface.ToolCount != 9 || observed.ToolCount != 9 ||
		surface.ManifestDigest != ProductSkillDirectoryManifestDigest || observed.ManifestDigest != ProductSkillDirectoryManifestDigest ||
		surface.AggregateSchemaBytes != ProductSkillDirectorySchemaBytes || observed.AggregateSchemaBytes != ProductSkillDirectorySchemaBytes ||
		surface.AggregateSchemaDigest != ProductSkillDirectorySchemaDigest || observed.AggregateSchemaDigest != ProductSkillDirectorySchemaDigest {
		return fmt.Errorf("bounded Skill directory surface is available only in its exact zero-egress fake simulation")
	}
	return nil
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

// ProductSharedMissionArtifactToolSurface adds only bounded reads of
// same-Mission published Task Artifacts to the fake-only @7 profile.
const ProductSharedMissionArtifactToolSurfaceQualification = "polis-product-tool-surface@8"

const (
	OfflineSharedMissionArtifactToolSurfacePurpose        = "offline-shared-mission-artifact-tool-surface"
	OfflineSharedMissionArtifactToolSurfaceSimulationMark = "offline-shared-mission-artifact-tool-surface-unqualified"
	ProductSharedMissionArtifactManifestDigest            = "4e1e75bd7f6753d5dc55b443e60a9959e7aebd953a3ee29c16a6559e821e8cc3"
	ProductSharedMissionArtifactSchemaDigest              = "daf4523db81f860aeffdc5e7137b86fd6f4108f12592dceaa9ba74791b3b66bf"
	ProductSharedMissionArtifactSchemaBytes               = 3813
)

func ProductSharedMissionArtifactToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithSharedMissionArtifacts())
}

// ProductWorkspaceTreeToolSurface adds the bounded logical private Task tree
// and immutable draft snapshots on a distinct fake-only registry. The real
// provider authorization path remains pinned to @4.
const ProductWorkspaceTreeToolSurfaceQualification = "polis-product-tool-surface@10"

const (
	OfflineWorkspaceTreeSurfacePurpose        = "offline-workspace-tree-tool-surface"
	OfflineWorkspaceTreeSurfaceSimulationMark = "offline-workspace-tree-tool-surface-unqualified"
	ProductWorkspaceTreeManifestDigest        = "8ada99789818b7198861c32f39cce9a7a9be48fd24e7a3b5a139dd2e762f1aba"
	ProductWorkspaceTreeSchemaDigest          = "cde1dc84c2e16ae7c85f6c590c704f03a6123d0a46775f709701192228b289a3"
	ProductWorkspaceTreeSchemaBytes           = 5386
)

func ProductWorkspaceTreeToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithWorkspaceTree())
}

const ProductWorkspaceSnapshotRevocationToolSurfaceQualification = "polis-product-tool-surface@11"

const (
	OfflineWorkspaceSnapshotRevocationSurfacePurpose        = "offline-workspace-snapshot-revocation-tool-surface"
	OfflineWorkspaceSnapshotRevocationSurfaceSimulationMark = "offline-workspace-snapshot-revocation-tool-surface-unqualified"
	ProductWorkspaceSnapshotRevocationManifestDigest        = "25152782a2709b80aefac62aef4361a5de4eceefd13133a940a0790bde35dd30"
	ProductWorkspaceSnapshotRevocationSchemaDigest          = "627364e0509ea58afb0ed584f44dbf78fcb99ec47bc4605af60908facb8ea2a1"
	ProductWorkspaceSnapshotRevocationSchemaBytes           = 5601
)

func ProductWorkspaceSnapshotRevocationToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithWorkspaceSnapshotRevocation())
}

const ProductMissionChangeAssessmentToolSurfaceQualification = "polis-product-tool-surface@12"
const ProductCSVInputRangeToolSurfaceQualification = "polis-product-tool-surface@13"
const ProductEnvironmentStatusToolSurfaceQualification = "polis-product-tool-surface@14"
const ProductReadOnlyJobsToolSurfaceQualification = "polis-product-tool-surface@15"
const ProductEnvironmentEnsureToolSurfaceQualification = "polis-product-tool-surface@16"
const ProductBorrowerLeaseToolSurfaceQualification = "polis-product-tool-surface@17"
const ProductBrowserRunToolSurfaceQualification = "polis-product-tool-surface@18"
const ProductResearchOperationsToolSurfaceQualification = "polis-product-tool-surface@19"

const (
	ProductEnvironmentStatusManifestDigest  = "b59ff6acd757764e66c73e1b5e638c3ff1a64a06f44491c919e017f91c48e0af"
	ProductEnvironmentStatusSchemaDigest    = "513160783bc7effd340097ee55519f6a993a3eb57e3ebccd70bf7884afca5270"
	ProductEnvironmentStatusSchemaBytes     = 2282
	ProductReadOnlyJobsManifestDigest       = "68e0ac39e1eb14a19b7243f5bcfab88b8d16630c7e4f6661089922ac32239944"
	ProductReadOnlyJobsSchemaDigest         = "0bd1fe983df7452ec0ce601384c5f03263670483c7312df00ec178e23f561f17"
	ProductReadOnlyJobsSchemaBytes          = 2484
	ProductEnvironmentEnsureManifestDigest  = "79698775c34e1c6ba4f5f664d3729a55a033807606fa6262099bf57d811a4fc7"
	ProductEnvironmentEnsureSchemaDigest    = "d533c6f3d06e91063c59ba3826bbaede8f292894fa16f4493bb348445979065a"
	ProductEnvironmentEnsureSchemaBytes     = 2431
	ProductBorrowerLeaseManifestDigest      = "267d40f1fe9266b6974fc1b41bdb8567ec7e695c3ec37f41e3c070030169f1c8"
	ProductBorrowerLeaseSchemaDigest        = "b59fdc07f682a504a3bc541a6e837c3a142e7e5b99ea33fc2eb0b1029f8e8738"
	ProductBorrowerLeaseSchemaBytes         = 2966
	ProductBrowserRunManifestDigest         = "1a954674caea420d4b9445bcf237fb0be4bf04d07634e16c595af0db62913456"
	ProductBrowserRunSchemaDigest           = "a691aeffad6545d1e618b6eb5c0a2c14152561620fef351e47230f764752a38e"
	ProductBrowserRunSchemaBytes            = 4071
	ProductResearchOperationsManifestDigest = "6149e0fd55fcdfe19575347c7b141536190e48d8cb4713b84ac1399f2e2c9554"
	ProductResearchOperationsSchemaDigest   = "5a853c10ef4f97d9c39304acef65cd42dae2aaabab0d12dc1d3ee0fd42f89a0c"
	ProductResearchOperationsSchemaBytes    = 4501
)

const (
	OfflineMissionChangeAssessmentSurfacePurpose        = "offline-mission-change-assessment-tool-surface"
	OfflineMissionChangeAssessmentSurfaceSimulationMark = "offline-mission-change-assessment-tool-surface-unqualified"
	OfflineCSVInputRangeSurfacePurpose                  = "offline-csv-input-range-tool-surface"
	OfflineCSVInputRangeSurfaceSimulationMarker         = "offline-csv-input-range-tool-surface-unqualified"
	OfflineEnvironmentStatusSurfacePurpose              = "offline-environment-status-tool-surface"
	OfflineEnvironmentStatusSurfaceSimulationMarker     = "offline-environment-status-tool-surface-unqualified"
	OfflineReadOnlyJobsSurfacePurpose                   = "offline-read-only-jobs-tool-surface"
	OfflineReadOnlyJobsSurfaceSimulationMarker          = "offline-read-only-jobs-tool-surface-unqualified"
	OfflineEnvironmentEnsureSurfacePurpose              = "offline-environment-ensure-tool-surface"
	OfflineEnvironmentEnsureSurfaceSimulationMarker     = "offline-environment-ensure-tool-surface-unqualified"
	OfflineBorrowerLeaseSurfacePurpose                  = "offline-borrower-lease-tool-surface"
	OfflineBorrowerLeaseSurfaceSimulationMarker         = "offline-borrower-lease-tool-surface-unqualified"
	OfflineBrowserRunSurfacePurpose                     = "offline-browser-run-tool-surface"
	OfflineBrowserRunSurfaceSimulationMarker            = "offline-browser-run-tool-surface-unqualified"
	OfflineResearchOperationsSurfacePurpose             = "offline-research-operations-tool-surface"
	OfflineResearchOperationsSurfaceSimulationMarker    = "offline-research-operations-tool-surface-unqualified"
	ProductMissionChangeAssessmentManifestDigest        = "cd9f4f852cd5674b1e0660868cb5af628a0186a0f137e8e1778e8fd6809c326d"
	ProductMissionChangeAssessmentSchemaDigest          = "0be7f09a90c96ed73596617207ed1bac7575c6b8fd56cfe086a5880352c6b28a"
	ProductMissionChangeAssessmentSchemaBytes           = 6740
)

func ProductMissionChangeAssessmentToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithMissionChangeAssessment())
}

// ProductCSVInputRangeToolSurface is an unqualified fake-only tool revision.
// The real-provider authorization remains pinned to the existing qualified
// product surface until its own exact-surface and provider evidence exists.
func ProductCSVInputRangeToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithCSVRangeRead())
}

// ProductEnvironmentStatusToolSurface is a fake-only read surface. The
// qualified real-provider contract remains pinned to @4.
func ProductEnvironmentStatusToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithEnvironmentStatus())
}

// ProductReadOnlyJobsToolSurface is a fake-only read surface. The qualified
// real-provider contract remains pinned to @4.
func ProductReadOnlyJobsToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithReadOnlyJobs())
}

// ProductBorrowerLeaseToolSurface is a fake-only extension. The control-plane
// lease records are testable offline; starting or keeping a service alive
// remains a separately qualified executor capability.
func ProductBorrowerLeaseToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithBorrowerLeases())
}

// ProductBrowserRunToolSurface is a fake-only control-plane surface. It can
// persist a default-denied request and read its result; it never opens a URL.
func ProductBrowserRunToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithBrowserRun())
}

// ProductResearchOperationsToolSurface is a fake-only unavailable backend
// surface. It records explicit intents without performing internet egress.
func ProductResearchOperationsToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithResearchOperations())
}

func ValidateOfflineFakeResearchOperationsSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductResearchOperationsToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductResearchOperationsToolSurfaceQualification ||
		profile.Purpose != OfflineResearchOperationsSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineResearchOperationsSurfaceSimulationMarker ||
		profile.ProductProviderL2Fingerprint != OfflineResearchOperationsSurfaceSimulationMarker || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 16 || expected.ManifestDigest != ProductResearchOperationsManifestDigest || expected.AggregateSchemaBytes != ProductResearchOperationsSchemaBytes || expected.AggregateSchemaDigest != ProductResearchOperationsSchemaDigest ||
		surface.ToolCount != 16 || observed.ToolCount != 16 || surface.ManifestDigest != ProductResearchOperationsManifestDigest || observed.ManifestDigest != ProductResearchOperationsManifestDigest ||
		surface.AggregateSchemaBytes != ProductResearchOperationsSchemaBytes || observed.AggregateSchemaBytes != ProductResearchOperationsSchemaBytes || surface.AggregateSchemaDigest != ProductResearchOperationsSchemaDigest || observed.AggregateSchemaDigest != ProductResearchOperationsSchemaDigest {
		return fmt.Errorf("research operations surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

func ValidateOfflineFakeBrowserRunSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductBrowserRunToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductBrowserRunToolSurfaceQualification ||
		profile.Purpose != OfflineBrowserRunSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineBrowserRunSurfaceSimulationMarker ||
		profile.ProductProviderL2Fingerprint != OfflineBrowserRunSurfaceSimulationMarker || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 14 || expected.ManifestDigest != ProductBrowserRunManifestDigest || expected.AggregateSchemaBytes != ProductBrowserRunSchemaBytes || expected.AggregateSchemaDigest != ProductBrowserRunSchemaDigest ||
		surface.ToolCount != 14 || observed.ToolCount != 14 || surface.ManifestDigest != ProductBrowserRunManifestDigest || observed.ManifestDigest != ProductBrowserRunManifestDigest ||
		surface.AggregateSchemaBytes != ProductBrowserRunSchemaBytes || observed.AggregateSchemaBytes != ProductBrowserRunSchemaBytes || surface.AggregateSchemaDigest != ProductBrowserRunSchemaDigest || observed.AggregateSchemaDigest != ProductBrowserRunSchemaDigest {
		return fmt.Errorf("BrowserRun surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

func ValidateOfflineFakeBorrowerLeaseSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductBorrowerLeaseToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductBorrowerLeaseToolSurfaceQualification ||
		profile.Purpose != OfflineBorrowerLeaseSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineBorrowerLeaseSurfaceSimulationMarker ||
		profile.ProductProviderL2Fingerprint != OfflineBorrowerLeaseSurfaceSimulationMarker || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 12 || expected.ManifestDigest != ProductBorrowerLeaseManifestDigest || expected.AggregateSchemaBytes != ProductBorrowerLeaseSchemaBytes || expected.AggregateSchemaDigest != ProductBorrowerLeaseSchemaDigest ||
		surface.ToolCount != 12 || observed.ToolCount != 12 || surface.ManifestDigest != ProductBorrowerLeaseManifestDigest || observed.ManifestDigest != ProductBorrowerLeaseManifestDigest ||
		surface.AggregateSchemaBytes != ProductBorrowerLeaseSchemaBytes || observed.AggregateSchemaBytes != ProductBorrowerLeaseSchemaBytes || surface.AggregateSchemaDigest != ProductBorrowerLeaseSchemaDigest || observed.AggregateSchemaDigest != ProductBorrowerLeaseSchemaDigest {
		return fmt.Errorf("borrower lease surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

func ValidateOfflineFakeReadOnlyJobsSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductReadOnlyJobsToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductReadOnlyJobsToolSurfaceQualification ||
		profile.Purpose != OfflineReadOnlyJobsSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineReadOnlyJobsSurfaceSimulationMarker ||
		profile.ProductProviderL2Fingerprint != OfflineReadOnlyJobsSurfaceSimulationMarker || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 9 || expected.ManifestDigest != ProductReadOnlyJobsManifestDigest || expected.AggregateSchemaBytes != ProductReadOnlyJobsSchemaBytes || expected.AggregateSchemaDigest != ProductReadOnlyJobsSchemaDigest ||
		surface.ToolCount != 9 || observed.ToolCount != 9 || surface.ManifestDigest != ProductReadOnlyJobsManifestDigest || observed.ManifestDigest != ProductReadOnlyJobsManifestDigest ||
		surface.AggregateSchemaBytes != ProductReadOnlyJobsSchemaBytes || observed.AggregateSchemaBytes != ProductReadOnlyJobsSchemaBytes || surface.AggregateSchemaDigest != ProductReadOnlyJobsSchemaDigest || observed.AggregateSchemaDigest != ProductReadOnlyJobsSchemaDigest {
		return fmt.Errorf("read-only jobs surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

// ProductEnvironmentEnsureToolSurface is a fake-only request surface. The
// qualified real-provider contract remains pinned to @4.
func ProductEnvironmentEnsureToolSurface() ToolSurface {
	return ToolSurfaceFromTools(codex.ProductEmployeeToolsWithEnvironmentEnsure())
}

func ValidateOfflineFakeEnvironmentEnsureSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductEnvironmentEnsureToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductEnvironmentEnsureToolSurfaceQualification ||
		profile.Purpose != OfflineEnvironmentEnsureSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineEnvironmentEnsureSurfaceSimulationMarker ||
		profile.ProductProviderL2Fingerprint != OfflineEnvironmentEnsureSurfaceSimulationMarker || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 9 || expected.ManifestDigest != ProductEnvironmentEnsureManifestDigest || expected.AggregateSchemaBytes != ProductEnvironmentEnsureSchemaBytes || expected.AggregateSchemaDigest != ProductEnvironmentEnsureSchemaDigest ||
		surface.ToolCount != 9 || observed.ToolCount != 9 || surface.ManifestDigest != ProductEnvironmentEnsureManifestDigest || observed.ManifestDigest != ProductEnvironmentEnsureManifestDigest ||
		surface.AggregateSchemaBytes != ProductEnvironmentEnsureSchemaBytes || observed.AggregateSchemaBytes != ProductEnvironmentEnsureSchemaBytes || surface.AggregateSchemaDigest != ProductEnvironmentEnsureSchemaDigest || observed.AggregateSchemaDigest != ProductEnvironmentEnsureSchemaDigest {
		return fmt.Errorf("environment ensure surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

func ValidateOfflineFakeEnvironmentStatusSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductEnvironmentStatusToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductEnvironmentStatusToolSurfaceQualification ||
		profile.Purpose != OfflineEnvironmentStatusSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineEnvironmentStatusSurfaceSimulationMarker ||
		profile.ProductProviderL2Fingerprint != OfflineEnvironmentStatusSurfaceSimulationMarker || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 8 || expected.ManifestDigest != ProductEnvironmentStatusManifestDigest ||
		expected.AggregateSchemaBytes != ProductEnvironmentStatusSchemaBytes || expected.AggregateSchemaDigest != ProductEnvironmentStatusSchemaDigest ||
		surface.ToolCount != 8 || observed.ToolCount != 8 || surface.ManifestDigest != ProductEnvironmentStatusManifestDigest ||
		observed.ManifestDigest != ProductEnvironmentStatusManifestDigest || surface.AggregateSchemaBytes != ProductEnvironmentStatusSchemaBytes ||
		observed.AggregateSchemaBytes != ProductEnvironmentStatusSchemaBytes || surface.AggregateSchemaDigest != ProductEnvironmentStatusSchemaDigest ||
		observed.AggregateSchemaDigest != ProductEnvironmentStatusSchemaDigest {
		return fmt.Errorf("environment status surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

func ValidateOfflineFakeCSVInputRangeSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductCSVInputRangeToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductCSVInputRangeToolSurfaceQualification ||
		profile.Purpose != OfflineCSVInputRangeSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineCSVInputRangeSurfaceSimulationMarker ||
		profile.ProductProviderL2Fingerprint != OfflineCSVInputRangeSurfaceSimulationMarker || profile.ToolCallLimit <= 0 ||
		surface.ToolCount != expected.ToolCount || observed.ToolCount != expected.ToolCount ||
		surface.ManifestDigest != expected.ManifestDigest || observed.ManifestDigest != expected.ManifestDigest ||
		surface.AggregateSchemaBytes != expected.AggregateSchemaBytes || observed.AggregateSchemaBytes != expected.AggregateSchemaBytes ||
		surface.AggregateSchemaDigest != expected.AggregateSchemaDigest || observed.AggregateSchemaDigest != expected.AggregateSchemaDigest {
		return fmt.Errorf("CSV range-read surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

func ValidateOfflineFakeMissionChangeAssessmentSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductMissionChangeAssessmentToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductMissionChangeAssessmentToolSurfaceQualification ||
		profile.Purpose != OfflineMissionChangeAssessmentSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineMissionChangeAssessmentSurfaceSimulationMark ||
		profile.ProductProviderL2Fingerprint != OfflineMissionChangeAssessmentSurfaceSimulationMark || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 25 || expected.ManifestDigest != ProductMissionChangeAssessmentManifestDigest ||
		expected.AggregateSchemaBytes != ProductMissionChangeAssessmentSchemaBytes || expected.AggregateSchemaDigest != ProductMissionChangeAssessmentSchemaDigest ||
		surface.ToolCount != 25 || observed.ToolCount != 25 || surface.ManifestDigest != ProductMissionChangeAssessmentManifestDigest ||
		observed.ManifestDigest != ProductMissionChangeAssessmentManifestDigest || surface.AggregateSchemaBytes != ProductMissionChangeAssessmentSchemaBytes ||
		observed.AggregateSchemaBytes != ProductMissionChangeAssessmentSchemaBytes || surface.AggregateSchemaDigest != ProductMissionChangeAssessmentSchemaDigest ||
		observed.AggregateSchemaDigest != ProductMissionChangeAssessmentSchemaDigest {
		return fmt.Errorf("mission-change assessment surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

func ValidateOfflineFakeWorkspaceSnapshotRevocationSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductWorkspaceSnapshotRevocationToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductWorkspaceSnapshotRevocationToolSurfaceQualification ||
		profile.Purpose != OfflineWorkspaceSnapshotRevocationSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineWorkspaceSnapshotRevocationSurfaceSimulationMark ||
		profile.ProductProviderL2Fingerprint != OfflineWorkspaceSnapshotRevocationSurfaceSimulationMark || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 23 || expected.ManifestDigest != ProductWorkspaceSnapshotRevocationManifestDigest ||
		expected.AggregateSchemaBytes != ProductWorkspaceSnapshotRevocationSchemaBytes || expected.AggregateSchemaDigest != ProductWorkspaceSnapshotRevocationSchemaDigest ||
		surface.ToolCount != 23 || observed.ToolCount != 23 || surface.ManifestDigest != ProductWorkspaceSnapshotRevocationManifestDigest ||
		observed.ManifestDigest != ProductWorkspaceSnapshotRevocationManifestDigest || surface.AggregateSchemaBytes != ProductWorkspaceSnapshotRevocationSchemaBytes ||
		observed.AggregateSchemaBytes != ProductWorkspaceSnapshotRevocationSchemaBytes || surface.AggregateSchemaDigest != ProductWorkspaceSnapshotRevocationSchemaDigest ||
		observed.AggregateSchemaDigest != ProductWorkspaceSnapshotRevocationSchemaDigest {
		return fmt.Errorf("workspace-snapshot revocation surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

func ValidateOfflineFakeWorkspaceTreeSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductWorkspaceTreeToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductWorkspaceTreeToolSurfaceQualification ||
		profile.Purpose != OfflineWorkspaceTreeSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineWorkspaceTreeSurfaceSimulationMark ||
		profile.ProductProviderL2Fingerprint != OfflineWorkspaceTreeSurfaceSimulationMark || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 22 || expected.ManifestDigest != ProductWorkspaceTreeManifestDigest ||
		expected.AggregateSchemaBytes != ProductWorkspaceTreeSchemaBytes || expected.AggregateSchemaDigest != ProductWorkspaceTreeSchemaDigest ||
		surface.ToolCount != 22 || observed.ToolCount != 22 || surface.ManifestDigest != ProductWorkspaceTreeManifestDigest ||
		observed.ManifestDigest != ProductWorkspaceTreeManifestDigest || surface.AggregateSchemaBytes != ProductWorkspaceTreeSchemaBytes ||
		observed.AggregateSchemaBytes != ProductWorkspaceTreeSchemaBytes || surface.AggregateSchemaDigest != ProductWorkspaceTreeSchemaDigest ||
		observed.AggregateSchemaDigest != ProductWorkspaceTreeSchemaDigest {
		return fmt.Errorf("workspace-tree surface is available only in its exact zero-egress fake simulation")
	}
	return nil
}

// ValidateOfflineFakeSharedMissionArtifactSurface accepts only the exact
// zero-egress @8 surface. It does not qualify a real provider or filesystem.
func ValidateOfflineFakeSharedMissionArtifactSurface(mode string, profile ExecutionProfile, surface ToolSurface) error {
	expected := ProductSharedMissionArtifactToolSurface()
	observed := ToolSurfaceFromTools(surface.Tools)
	if mode != "fake" || profile.ToolSurfaceQualification != ProductSharedMissionArtifactToolSurfaceQualification ||
		profile.Purpose != OfflineSharedMissionArtifactToolSurfacePurpose || profile.ExecutionEnvelope != OfflineExecutionEnvelope ||
		profile.ExactSurfaceExecutionFingerprint != OfflineSharedMissionArtifactToolSurfaceSimulationMark ||
		profile.ProductProviderL2Fingerprint != OfflineSharedMissionArtifactToolSurfaceSimulationMark || profile.ToolCallLimit <= 0 ||
		expected.ToolCount != 14 || expected.ManifestDigest != ProductSharedMissionArtifactManifestDigest ||
		expected.AggregateSchemaBytes != ProductSharedMissionArtifactSchemaBytes || expected.AggregateSchemaDigest != ProductSharedMissionArtifactSchemaDigest ||
		surface.ToolCount != 14 || observed.ToolCount != 14 ||
		surface.ManifestDigest != ProductSharedMissionArtifactManifestDigest || observed.ManifestDigest != ProductSharedMissionArtifactManifestDigest ||
		surface.AggregateSchemaBytes != ProductSharedMissionArtifactSchemaBytes || observed.AggregateSchemaBytes != ProductSharedMissionArtifactSchemaBytes ||
		surface.AggregateSchemaDigest != ProductSharedMissionArtifactSchemaDigest || observed.AggregateSchemaDigest != ProductSharedMissionArtifactSchemaDigest {
		return fmt.Errorf("shared-Mission-Artifact surface is available only in its exact zero-egress fake simulation")
	}
	return nil
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
	UsageUpdates          int
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
