// pattern: Functional Core
package environment

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const EnvironmentExecutorQualificationEvidenceSchema = "polis-node-executor-qualification@1"

type EnvironmentExecutorQualificationCheck struct {
	ID                    string `json:"id"`
	Status                string `json:"status"`
	EvidenceInputID       string `json:"evidenceInputId"`
	EvidenceInputRevision int64  `json:"evidenceInputRevision"`
	EvidenceSHA256        string `json:"evidenceSha256"`
}

type EnvironmentExecutorQualificationEvidence struct {
	SchemaVersion             string                                  `json:"schemaVersion"`
	ProfileID                 string                                  `json:"profileId"`
	ExecutorFingerprintSHA256 string                                  `json:"executorFingerprintSha256"`
	HostFingerprintSHA256     string                                  `json:"hostFingerprintSha256"`
	IsolationPolicySHA256     string                                  `json:"isolationPolicySha256"`
	ToolchainSHA256           string                                  `json:"toolchainSha256"`
	Checks                    []EnvironmentExecutorQualificationCheck `json:"checks"`
}

func RequiredEnvironmentExecutorQualificationChecks(profileID string) []string {
	switch profileID {
	case WindowsNodeNPMProfile:
		return []string{
			"windows_input_boundary",
			"windows_dependency_policy",
			"windows_appcontainer_network_boundary",
			"windows_service_listener_network_boundary",
			"windows_job_object_cleanup",
			"windows_service_browser_ingress",
			"windows_cpu_memory_process_bounds",
			"windows_workspace_storage_ceiling",
			"windows_clean_vm_install_recovery",
		}
	case LinuxNodeNPMProfile:
		return []string{
			"linux_input_boundary",
			"linux_offline_dependency_policy",
			"linux_bwrap_network_boundary",
			"linux_cgroup_ownership_cleanup",
			"linux_service_browser_ingress",
			"linux_resource_bounds",
			"linux_clean_host_recovery",
		}
	default:
		return nil
	}
}

func ValidateEnvironmentExecutorQualificationEvidence(content []byte, profileID, toolchainSHA256 string, fingerprint EnvironmentExecutorFingerprint, decision string) (EnvironmentExecutorQualificationEvidence, error) {
	if len(content) == 0 || len(content) > 64<<10 || !validEnvironmentFingerprintDigest(toolchainSHA256) ||
		!validEnvironmentFingerprintDigest(fingerprint.ExecutorSHA256) || !validEnvironmentFingerprintDigest(fingerprint.HostSHA256) ||
		!validEnvironmentFingerprintDigest(fingerprint.IsolationPolicySHA256) || (decision != "qualified" && decision != "revoked") {
		return EnvironmentExecutorQualificationEvidence{}, errors.New("executor qualification evidence request is invalid")
	}
	required := RequiredEnvironmentExecutorQualificationChecks(profileID)
	if len(required) == 0 {
		return EnvironmentExecutorQualificationEvidence{}, errors.New("executor qualification profile is unsupported")
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var evidence EnvironmentExecutorQualificationEvidence
	if err := decoder.Decode(&evidence); err != nil {
		return EnvironmentExecutorQualificationEvidence{}, errors.New("executor qualification evidence is malformed")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return EnvironmentExecutorQualificationEvidence{}, errors.New("executor qualification evidence contains trailing data")
	}
	if evidence.SchemaVersion != EnvironmentExecutorQualificationEvidenceSchema || evidence.ProfileID != profileID ||
		evidence.ExecutorFingerprintSHA256 != fingerprint.ExecutorSHA256 || evidence.HostFingerprintSHA256 != fingerprint.HostSHA256 ||
		evidence.IsolationPolicySHA256 != fingerprint.IsolationPolicySHA256 || evidence.ToolchainSHA256 != toolchainSHA256 || len(evidence.Checks) != len(required) {
		return EnvironmentExecutorQualificationEvidence{}, errors.New("executor qualification evidence does not match the current host, profile or toolchain")
	}
	checks := make(map[string]EnvironmentExecutorQualificationCheck, len(evidence.Checks))
	for _, check := range evidence.Checks {
		if check.ID == "" || !validEnvironmentEvidenceReferenceID(check.EvidenceInputID) || check.EvidenceInputRevision <= 0 || !validEnvironmentFingerprintDigest(check.EvidenceSHA256) || (check.Status != "passed" && check.Status != "failed") {
			return EnvironmentExecutorQualificationEvidence{}, errors.New("executor qualification evidence check is invalid")
		}
		if _, exists := checks[check.ID]; exists {
			return EnvironmentExecutorQualificationEvidence{}, errors.New("executor qualification evidence repeats a check")
		}
		checks[check.ID] = check
	}
	for _, checkID := range required {
		check, exists := checks[checkID]
		if !exists || (decision == "qualified" && check.Status != "passed") {
			return EnvironmentExecutorQualificationEvidence{}, errors.New("executor qualification evidence omits or fails a required check")
		}
	}
	return evidence, nil
}

func validEnvironmentEvidenceReferenceID(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.') {
			return false
		}
	}
	return true
}
