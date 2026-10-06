// pattern: Functional Core
package kernel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"polis/internal/core"
)

const OperationEvidenceManifestSchema = "polis-operation-evidence@1"

type OperationEvidenceManifest struct {
	SchemaVersion string                         `json:"schemaVersion"`
	CompanyID     string                         `json:"companyId"`
	MissionID     string                         `json:"missionId"`
	TaskID        string                         `json:"taskId"`
	OperationID   string                         `json:"operationId"`
	Artifacts     []OperationEvidenceArtifactRef `json:"artifacts"`
}

type OperationEvidenceArtifactRef struct {
	ArtifactID string `json:"artifactId"`
	Digest     string `json:"digest"`
	Kind       string `json:"kind"`
}

type ResearchOperationEvidenceEnvelope struct {
	EvidenceManifestSHA256 string                    `json:"evidenceManifestSha256"`
	EvidenceManifest       OperationEvidenceManifest `json:"evidenceManifest"`
	Payload                json.RawMessage           `json:"payload,omitempty"`
}

func ValidateResearchOperationEvidenceEnvelope(raw []byte, companyID, missionID, taskID, operationID string) (ResearchOperationEvidenceEnvelope, error) {
	var envelope ResearchOperationEvidenceEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil || !validEnvironmentSHA256(envelope.EvidenceManifestSHA256) {
		return ResearchOperationEvidenceEnvelope{}, errors.New("research operation evidence envelope is malformed")
	}
	canonicalManifest, err := CanonicalOperationEvidenceJSON(mustMarshalOperationEvidenceManifest(envelope.EvidenceManifest))
	if err != nil || digestCapabilityBytes(canonicalManifest) != envelope.EvidenceManifestSHA256 || ValidateOperationEvidenceManifest(envelope.EvidenceManifest, companyID, missionID, taskID, operationID) != nil {
		return ResearchOperationEvidenceEnvelope{}, errors.New("research operation evidence envelope is invalid")
	}
	return envelope, nil
}

func mustMarshalOperationEvidenceManifest(manifest OperationEvidenceManifest) []byte {
	encoded, _ := json.Marshal(manifest)
	return encoded
}

// CanonicalOperationEvidenceJSON defines the bytes hashed into
// evidence_manifest_sha256. Database JSONB and callers must use this same
// parse-and-marshal form rather than hashing presentation whitespace or key
// order.
func CanonicalOperationEvidenceJSON(raw []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return nil, errors.New("operation evidence contains trailing JSON")
		}
		return nil, err
	}
	return json.Marshal(value)
}

func ValidateOperationEvidenceManifest(manifest OperationEvidenceManifest, companyID, missionID, taskID, operationID string) error {
	if manifest.SchemaVersion != OperationEvidenceManifestSchema || manifest.CompanyID != companyID || manifest.MissionID != missionID || manifest.TaskID != taskID || manifest.OperationID != operationID || !core.ValidID(companyID) || !core.ValidID(missionID) || !core.ValidID(taskID) || !core.ValidID(operationID) {
		return errors.New("operation evidence manifest scope is invalid")
	}
	if len(manifest.Artifacts) == 0 || len(manifest.Artifacts) > 16 {
		return errors.New("operation evidence manifest artifact count is invalid")
	}
	seen := make(map[string]struct{}, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		if !core.ValidID(artifact.ArtifactID) || !validEnvironmentSHA256(artifact.Digest) || !validOperationEvidenceKind(artifact.Kind) {
			return fmt.Errorf("operation evidence artifact reference is invalid")
		}
		if _, exists := seen[artifact.ArtifactID]; exists {
			return errors.New("operation evidence manifest contains a duplicate artifact")
		}
		seen[artifact.ArtifactID] = struct{}{}
	}
	return nil
}

func validOperationEvidenceKind(value string) bool {
	switch value {
	case "screenshot", "dom", "trace", "console_log", "network_log", "search_result", "page_snapshot":
		return true
	default:
		return false
	}
}
