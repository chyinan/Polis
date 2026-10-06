// pattern: Functional Core
package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// FixedTeamTaskRevision is the semantic binding between an owner-confirmed
// RoleRevision and one concrete task assignment. It is deliberately a
// decision record, not an execution grant: the current fixed-team revision is
// unverified, so every result remains human-gated.
type FixedTeamTaskRevision struct {
	RevisionSHA256     string `json:"revision_sha256"`
	RoleRevisionSHA256 string `json:"role_revision_sha256"`
	TaskType           string `json:"task_type"`
	TaskKind           string `json:"task_kind"`
	Owner              string `json:"owner"`
	Qualification      string `json:"qualification"`
	RequiresHuman      bool   `json:"requires_human"`
	ReasonCode         string `json:"reason_code"`
}

type fixedTeamTaskRevisionDigestPayload struct {
	RoleRevisionSHA256 string `json:"role_revision_sha256"`
	TaskType           string `json:"task_type"`
	TaskKind           string `json:"task_kind"`
	Owner              string `json:"owner"`
	Qualification      string `json:"qualification"`
	RequiresHuman      bool   `json:"requires_human"`
	ReasonCode         string `json:"reason_code"`
}

// TaskRevisionFor resolves a semantic task_type/owner/TaskKind tuple against
// the exact fixed-team RoleRevision. It reports mismatches explicitly and
// never turns the owner confirmation or descriptive mapping into execution
// permission.
func (revision FixedTeamCoverageRoleRevision) TaskRevisionFor(taskType, owner, taskKind string) FixedTeamTaskRevision {
	qualification := revision.Qualification
	if qualification == "" {
		qualification = "unverified"
	}
	result := FixedTeamTaskRevision{
		RoleRevisionSHA256: revision.RevisionSHA256,
		TaskType:           taskType,
		TaskKind:           taskKind,
		Owner:              owner,
		Qualification:      qualification,
		RequiresHuman:      true,
		ReasonCode:         "role_revision_unavailable",
	}
	if !revision.compiled || revision.RevisionSHA256 != FixedTeamCoverageSHA256() || revision.OwnerDecision != FixedTeamCoverageOwnerDecision {
		return withFixedTeamTaskRevisionDigest(result)
	}
	var assignment *FixedTeamCoverageAssignment
	for index := range revision.Contract.Coverage {
		if revision.Contract.Coverage[index].TaskType == taskType {
			assignment = &revision.Contract.Coverage[index]
			break
		}
	}
	if assignment == nil {
		result.ReasonCode = "task_type_uncovered"
		return withFixedTeamTaskRevisionDigest(result)
	}
	if owner != assignment.Owner {
		result.ReasonCode = "task_owner_mismatch"
		return withFixedTeamTaskRevisionDigest(result)
	}
	expectedTaskKind := FixedTeamCoverageTaskKind(taskType)
	if expectedTaskKind == "" || taskKind != expectedTaskKind {
		result.ReasonCode = "task_kind_mismatch"
		return withFixedTeamTaskRevisionDigest(result)
	}
	result.ReasonCode = "task_revision_unqualified"
	return withFixedTeamTaskRevisionDigest(result)
}

func withFixedTeamTaskRevisionDigest(result FixedTeamTaskRevision) FixedTeamTaskRevision {
	payload := fixedTeamTaskRevisionDigestPayload{
		RoleRevisionSHA256: result.RoleRevisionSHA256,
		TaskType:           result.TaskType,
		TaskKind:           result.TaskKind,
		Owner:              result.Owner,
		Qualification:      result.Qualification,
		RequiresHuman:      result.RequiresHuman,
		ReasonCode:         result.ReasonCode,
	}
	encoded, _ := json.Marshal(payload)
	digest := sha256.Sum256(encoded)
	result.RevisionSHA256 = hex.EncodeToString(digest[:])
	return result
}
