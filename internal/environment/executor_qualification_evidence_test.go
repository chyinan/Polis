// pattern: Functional Core
package environment

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEnvironmentExecutorQualificationEvidenceBindsEveryRequiredCheck(t *testing.T) {
	fingerprint := EnvironmentExecutorFingerprint{
		ExecutorSHA256:        strings.Repeat("a", 64),
		HostSHA256:            strings.Repeat("b", 64),
		IsolationPolicySHA256: strings.Repeat("c", 64),
	}
	report := EnvironmentExecutorQualificationEvidence{
		SchemaVersion: "polis-node-executor-qualification@1", ProfileID: WindowsNodeNPMProfile,
		ExecutorFingerprintSHA256: fingerprint.ExecutorSHA256, HostFingerprintSHA256: fingerprint.HostSHA256,
		IsolationPolicySHA256: fingerprint.IsolationPolicySHA256, ToolchainSHA256: strings.Repeat("d", 64),
	}
	for _, checkID := range RequiredEnvironmentExecutorQualificationChecks(WindowsNodeNPMProfile) {
		report.Checks = append(report.Checks, EnvironmentExecutorQualificationCheck{ID: checkID, Status: "passed", EvidenceInputID: "qualification-proof", EvidenceInputRevision: 1, EvidenceSHA256: strings.Repeat("e", 64)})
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateEnvironmentExecutorQualificationEvidence(encoded, WindowsNodeNPMProfile, report.ToolchainSHA256, fingerprint, "qualified"); err != nil {
		t.Fatalf("valid Windows qualification evidence: %v", err)
	}

	stale := report
	stale.HostFingerprintSHA256 = strings.Repeat("f", 64)
	staleBytes, _ := json.Marshal(stale)
	if _, err = ValidateEnvironmentExecutorQualificationEvidence(staleBytes, WindowsNodeNPMProfile, report.ToolchainSHA256, fingerprint, "qualified"); err == nil {
		t.Fatal("accepted a qualification report from another host")
	}

	missing := report
	missing.Checks = append([]EnvironmentExecutorQualificationCheck(nil), report.Checks[:len(report.Checks)-1]...)
	missingBytes, _ := json.Marshal(missing)
	if _, err = ValidateEnvironmentExecutorQualificationEvidence(missingBytes, WindowsNodeNPMProfile, report.ToolchainSHA256, fingerprint, "qualified"); err == nil {
		t.Fatal("accepted a report missing a required Windows qualification check")
	}
	legacyCombined := report
	legacyCombined.Checks = make([]EnvironmentExecutorQualificationCheck, 0, len(report.Checks)-1)
	for _, check := range report.Checks {
		if check.ID == "windows_cpu_memory_process_bounds" || check.ID == "windows_workspace_storage_ceiling" {
			continue
		}
		legacyCombined.Checks = append(legacyCombined.Checks, check)
	}
	legacyCombined.Checks = append(legacyCombined.Checks, EnvironmentExecutorQualificationCheck{
		ID: "windows_resource_bounds", Status: "passed", EvidenceInputID: "qualification-proof", EvidenceInputRevision: 1, EvidenceSHA256: strings.Repeat("e", 64),
	})
	legacyBytes, _ := json.Marshal(legacyCombined)
	if _, err = ValidateEnvironmentExecutorQualificationEvidence(legacyBytes, WindowsNodeNPMProfile, report.ToolchainSHA256, fingerprint, "qualified"); err == nil {
		t.Fatal("accepted a report that collapsed disk storage and CPU/memory into one qualification check")
	}

	failed := report
	failed.Checks = append([]EnvironmentExecutorQualificationCheck(nil), report.Checks...)
	failed.Checks[0].Status = "failed"
	failedBytes, _ := json.Marshal(failed)
	if _, err = ValidateEnvironmentExecutorQualificationEvidence(failedBytes, WindowsNodeNPMProfile, report.ToolchainSHA256, fingerprint, "qualified"); err == nil {
		t.Fatal("qualified an executor with a failed required check")
	}
}

func TestWindowsExecutorQualificationSeparatesDiskQuotaFromCPUAndMemoryBounds(t *testing.T) {
	required := RequiredEnvironmentExecutorQualificationChecks(WindowsNodeNPMProfile)
	var hasCPUAndMemory, hasWorkspaceDisk, hasServiceListener, hasCombinedResource bool
	for _, checkID := range required {
		switch checkID {
		case "windows_cpu_memory_process_bounds":
			hasCPUAndMemory = true
		case "windows_workspace_storage_ceiling":
			hasWorkspaceDisk = true
		case "windows_service_listener_network_boundary":
			hasServiceListener = true
		case "windows_resource_bounds":
			hasCombinedResource = true
		}
	}
	if !hasCPUAndMemory || !hasWorkspaceDisk || !hasServiceListener || hasCombinedResource {
		t.Fatalf("Windows qualification checks do not require separate CPU/memory, disk and service-listener proof: %v", required)
	}
}

func TestEnvironmentExecutorQualificationEvidenceRejectsUnknownAndUnboundedData(t *testing.T) {
	fingerprint := EnvironmentExecutorFingerprint{ExecutorSHA256: strings.Repeat("a", 64), HostSHA256: strings.Repeat("b", 64), IsolationPolicySHA256: strings.Repeat("c", 64)}
	if _, err := ValidateEnvironmentExecutorQualificationEvidence([]byte(`{"schemaVersion":"polis-node-executor-qualification@1","profileId":"windows-node-npm@1","unexpected":true}`), WindowsNodeNPMProfile, strings.Repeat("d", 64), fingerprint, "qualified"); err == nil {
		t.Fatal("accepted an unknown qualification report field")
	}
	if _, err := ValidateEnvironmentExecutorQualificationEvidence(make([]byte, 64*1024+1), WindowsNodeNPMProfile, strings.Repeat("d", 64), fingerprint, "qualified"); err == nil {
		t.Fatal("accepted an oversized qualification report")
	}
}
