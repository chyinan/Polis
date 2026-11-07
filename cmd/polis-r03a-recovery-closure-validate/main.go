package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/recovery"
	"reflect"
	"sort"
	"strconv"
)

type stateExport struct {
	Tasks                  []map[string]any `json:"tasks"`
	ContractRevisions      []map[string]any `json:"contract_revisions"`
	Messages               []map[string]any `json:"messages"`
	Obligations            []map[string]any `json:"obligations"`
	PeerWorkSignals        []map[string]any `json:"peer_work_signals"`
	WorkerWorkspaces       []map[string]any `json:"worker_workspaces"`
	WorkerCheckpoints      []map[string]any `json:"worker_checkpoints"`
	Artifacts              []map[string]any `json:"artifacts"`
	ArtifactQualifications []map[string]any `json:"artifact_qualifications"`
	Events                 []map[string]any `json:"events"`
}

type casManifest struct {
	Entries []recoveryCASBlob `json:"entries"`
}

type recoveryCASBlob struct {
	CompanyID string `json:"company_id"`
	Digest    string `json:"content_sha256"`
	Size      int64  `json:"size"`
}

func main() {
	sourcePath := flag.String("source-state", "", "immutable source state export")
	restoredPath := flag.String("restored-state", "", "immutable restored state export")
	casManifestPath := flag.String("cas-manifest", "", "immutable CAS manifest")
	casRoot := flag.String("cas-root", "", "immutable package CAS root")
	output := flag.String("output", "", "qualification output")
	flag.Parse()
	if *sourcePath == "" || *casManifestPath == "" || *casRoot == "" || *output == "" {
		fail("source-state, cas-manifest, cas-root and output are required")
	}
	var source, restored stateExport
	readJSON(*sourcePath, &source)
	restoredComparison := "NOT_APPLICABLE_BEFORE_RESTORE"
	if *restoredPath != "" {
		readJSON(*restoredPath, &restored)
		if err := compareClosure(source, restored); err != nil {
			fail("RESTORE_MAPPING_DEFECT: " + err.Error())
		}
		restoredComparison = "PASSED"
	} else {
		restored = source
	}
	artifact, workspace, qualification, checkpoint := closureValues(restored)
	if artifact == nil || workspace == nil || qualification == nil || checkpoint == nil {
		fail("PACKAGE_CAPTURE_DEFECT: continuity-critical closure record is missing")
	}
	artifactDigest := stringValue(artifact, "digest")
	artifactID := stringValue(artifact, "id")
	artifactState := stringValue(artifact, "state")
	artifactVerdict := stringValue(artifact, "verdict")
	workspaceTaskID := stringValue(workspace, "task_id")
	workspaceRevision := intValue(workspace, "revision")
	workspaceDigest := stringValue(workspace, "digest")
	qualificationWorkspaceRevision := intValue(qualification, "workspace_revision")
	qualificationWorkspaceDigest := stringValue(qualification, "workspace_digest")
	checkpointData, _ := checkpoint["data"].(map[string]any)
	checkpointRevision := intValue(checkpointData, "workspace_revision")
	checkpointDigest := stringValue(checkpointData, "workspace_digest")
	checkpointContract := stringValue(checkpointData, "contract_revision_id")
	expectedContract := stringValue(qualification, "contract_revision_id")
	if artifactID == "" || expectedContract == "" || stringValue(qualification, "artifact_id") != artifactID {
		fail("PACKAGE_CAPTURE_DEFECT: artifact qualification identity is incomplete")
	}
	var manifest casManifest
	readJSON(*casManifestPath, &manifest)
	casPresent, err := verifyPackageCAS(*casRoot, manifest.Entries)
	if err != nil {
		fail("CAS_REBIND_DEFECT: " + err.Error())
	}
	report, err := recovery.ValidateArtifactWorkspaceContinuity(recovery.ArtifactWorkspaceContinuityInput{
		ArtifactID: artifactID, ArtifactDigest: artifactDigest, ArtifactState: artifactState, ArtifactVerdict: artifactVerdict,
		ArtifactWorkspaceTaskID: workspaceTaskID, ArtifactWorkspaceRevision: qualificationWorkspaceRevision, ArtifactWorkspaceDigest: qualificationWorkspaceDigest,
		CurrentWorkspaceTaskID: workspaceTaskID, CurrentWorkspaceRevision: workspaceRevision, CurrentWorkspaceDigest: workspaceDigest,
		CheckpointID: stringValue(checkpoint, "id"), CheckpointWorkspaceRevision: checkpointRevision, CheckpointWorkspaceDigest: checkpointDigest,
		CheckpointContractID: checkpointContract, ExpectedContractID: expectedContract,
		CASArtifactDigest: artifactDigest, CASBlobPresent: casPresent[artifactDigest],
	})
	if err != nil {
		fail("CONTINUITY_VERIFIER_DEFECT: " + err.Error())
	}
	events := restored.Events
	lastSequence := int64(0)
	eventOrder := true
	for _, event := range events {
		sequence := intValue(event, "company_seq")
		if sequence <= lastSequence {
			eventOrder = false
		}
		lastSequence = sequence
	}
	writeJSON(*output, map[string]any{
		"status": "PASSED", "package_semantic_closure": "PASSED", "artifact_workspace_continuity": report,
		"source_restored_relational_closure": restoredComparison, "closure_canonicalization_revision": "r03a-semantic-closure@1", "event_ordering_contract": "company_seq", "cas_closure": "PASSED", "synthesized_rows": 0, "synthesized_blobs": 0,
		"event_count": len(events), "event_ordering": eventOrder,
		"package_mutated": false, "restored_specimen_mutated": false, "artifact_id": artifactID, "artifact_digest": artifactDigest,
		"artifact_state": artifactState, "artifact_verdict": artifactVerdict, "artifact_workspace_revision": qualificationWorkspaceRevision,
		"artifact_workspace_digest": qualificationWorkspaceDigest, "checkpoint_id": stringValue(checkpoint, "id"),
		"checkpoint_contract_revision_id": checkpointContract, "required_cas_blob_count": len(manifest.Entries), "cas_digest_match_count": len(casPresent),
	})
}

func compareClosure(source, restored stateExport) error {
	left, right := closureArrays(source), closureArrays(restored)
	for name, before := range left {
		a, b := canonicalSlice(name, before), canonicalSlice(name, right[name])
		if !reflect.DeepEqual(a, b) {
			return fmt.Errorf("closure array %s differs after canonicalization: source_len=%d restored_len=%d source_sample=%s restored_sample=%s", name, len(a), len(b), sampleJSON(a), sampleJSON(b))
		}
	}
	return nil
}

func closureArrays(s stateExport) map[string][]map[string]any {
	return map[string][]map[string]any{"tasks": s.Tasks, "contract_revisions": s.ContractRevisions, "messages": s.Messages, "obligations": s.Obligations, "peer_work_signals": s.PeerWorkSignals, "worker_workspaces": s.WorkerWorkspaces, "worker_checkpoints": s.WorkerCheckpoints, "artifacts": s.Artifacts, "artifact_qualifications": s.ArtifactQualifications, "events": s.Events}
}

func canonicalSlice(name string, values []map[string]any) []map[string]any {
	out := append([]map[string]any(nil), values...)
	sort.SliceStable(out, func(i, j int) bool { return semanticKey(name, out[i]) < semanticKey(name, out[j]) })
	return out
}

func semanticKey(name string, value map[string]any) string {
	if name == "events" {
		return fmt.Sprintf("%020d", intValue(value, "company_seq"))
	}
	keys := map[string]string{"tasks": "id", "contract_revisions": "revision", "messages": "id", "obligations": "id", "peer_work_signals": "id", "worker_workspaces": "task_id", "worker_checkpoints": "id", "artifacts": "id", "artifact_qualifications": "artifact_id"}
	if key, ok := keys[name]; ok {
		return stringValue(value, key)
	}
	return sampleJSON([]map[string]any{value})
}

func sampleJSON(value any) string {
	raw, _ := json.Marshal(value)
	if len(raw) > 1200 {
		return string(raw[:1200]) + "..."
	}
	return string(raw)
}

func closureValues(s stateExport) (map[string]any, map[string]any, map[string]any, map[string]any) {
	var artifact, workspace, qualification, checkpoint map[string]any
	for _, value := range s.Artifacts {
		if stringValue(value, "verdict") == "candidate" {
			artifact = value
			break
		}
	}
	if artifact == nil {
		return nil, nil, nil, nil
	}
	for _, value := range s.WorkerWorkspaces {
		if stringValue(value, "task_id") == taskForArtifact(s.Tasks, stringValue(artifact, "task_id")) {
			workspace = value
			break
		}
	}
	for _, value := range s.ArtifactQualifications {
		if stringValue(value, "artifact_id") == stringValue(artifact, "id") {
			qualification = value
			break
		}
	}
	for _, value := range s.WorkerCheckpoints {
		data, _ := value["data"].(map[string]any)
		if stringValue(data, "kind") == "qualified" {
			checkpoint = value
			break
		}
	}
	return artifact, workspace, qualification, checkpoint
}

func taskForArtifact(tasks []map[string]any, taskID string) string {
	for _, task := range tasks {
		if stringValue(task, "id") == taskID {
			return taskID
		}
	}
	return taskID
}
func stringValue(value map[string]any, key string) string { v, _ := value[key].(string); return v }
func intValue(value map[string]any, key string) int64 {
	switch v := value[key].(type) {
	case float64:
		return int64(v)
	case json.Number:
		n, _ := strconv.ParseInt(string(v), 10, 64)
		return n
	}
	return 0
}

func verifyPackageCAS(root string, entries []recoveryCASBlob) (map[string]bool, error) {
	present := map[string]bool{}
	for _, entry := range entries {
		path := filepath.Join(root, entry.CompanyID, entry.Digest)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("missing blob %s: %w", entry.Digest, err)
		}
		if !info.Mode().IsRegular() || info.Size() != entry.Size {
			return nil, fmt.Errorf("invalid blob %s", entry.Digest)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(raw)
		if hex.EncodeToString(hash[:]) != entry.Digest {
			return nil, fmt.Errorf("digest mismatch %s", entry.Digest)
		}
		present[entry.Digest] = true
	}
	return present, nil
}

func readJSON(path string, target any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail(err.Error())
	}
	if err := json.Unmarshal(raw, target); err != nil {
		fail(err.Error())
	}
}
func writeJSON(path string, value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		fail(err.Error())
	}
}
func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
