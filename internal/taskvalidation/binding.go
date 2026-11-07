// Package taskvalidation defines the public acceptance contract and its
// immutable, task-scoped validator binding.
package taskvalidation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

const (
	AcceptanceContractRevision    = "text-acceptance@1"
	TextContainsAllRunnerKind     = "text_contains_all"
	TextContainsAllRunnerRevision = "text-contains-all@1"
	maxRequiredTextItems          = 8
	maxRequiredTextBytes          = 512
)

var identityPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

// AcceptanceContract is user-visible Mission input. RequiredText items are
// literal public criteria; only the two documented identity placeholders are
// substituted when a Mission is frozen into a Task binding.
type AcceptanceContract struct {
	Revision     string   `json:"revision"`
	RequiredText []string `json:"required_text"`
}

// Binding freezes the Mission's public contract and the trusted validator
// selection for one Task. The configuration digest covers the identities,
// runner selection and fully resolved contract.
type Binding struct {
	TaskID              string             `json:"task_id"`
	MissionID           string             `json:"mission_id"`
	AcceptanceRevision  string             `json:"acceptance_revision"`
	RunnerKind          string             `json:"runner_kind"`
	RunnerRevision      string             `json:"runner_revision"`
	ConfigurationDigest string             `json:"configuration_digest"`
	Contract            AcceptanceContract `json:"contract"`
}

var ErrInvalidContract = errors.New("invalid task acceptance contract")

// ValidateContract validates an optional public Mission contract. A nil
// contract is valid and represents an exploratory Task without qualification.
func ValidateContract(contract *AcceptanceContract) error {
	if contract == nil {
		return nil
	}
	if contract.Revision != AcceptanceContractRevision || len(contract.RequiredText) == 0 || len(contract.RequiredText) > maxRequiredTextItems {
		return ErrInvalidContract
	}
	seen := make(map[string]struct{}, len(contract.RequiredText))
	total := 0
	for _, criterion := range contract.RequiredText {
		criterion = strings.TrimSpace(criterion)
		if criterion == "" || len(criterion) > maxRequiredTextBytes || !validPlaceholders(criterion) {
			return ErrInvalidContract
		}
		if _, exists := seen[criterion]; exists {
			return ErrInvalidContract
		}
		seen[criterion] = struct{}{}
		total += len(criterion)
	}
	if total > 2048 {
		return ErrInvalidContract
	}
	return nil
}

// Bind returns a detached, task-specific snapshot of the public Mission
// contract. A nil contract intentionally produces no binding.
func Bind(taskID, missionID string, contract *AcceptanceContract) (*Binding, error) {
	if contract == nil {
		return nil, nil
	}
	if !identityPattern.MatchString(taskID) || !identityPattern.MatchString(missionID) || ValidateContract(contract) != nil {
		return nil, ErrInvalidContract
	}
	bound := AcceptanceContract{Revision: contract.Revision, RequiredText: make([]string, len(contract.RequiredText))}
	for i, criterion := range contract.RequiredText {
		criterion = strings.TrimSpace(criterion)
		criterion = strings.ReplaceAll(criterion, "{{mission_id}}", missionID)
		criterion = strings.ReplaceAll(criterion, "{{task_id}}", taskID)
		bound.RequiredText[i] = criterion
	}
	binding := &Binding{
		TaskID:             taskID,
		MissionID:          missionID,
		AcceptanceRevision: contract.Revision,
		RunnerKind:         TextContainsAllRunnerKind,
		RunnerRevision:     TextContainsAllRunnerRevision,
		Contract:           bound,
	}
	binding.ConfigurationDigest = ConfigurationDigest(*binding)
	return binding, nil
}

// ConfigurationDigest returns the stable digest of a binding's authority and
// immutable validator configuration, excluding the digest field itself.
func ConfigurationDigest(binding Binding) string {
	canonical := struct {
		TaskID             string             `json:"task_id"`
		MissionID          string             `json:"mission_id"`
		AcceptanceRevision string             `json:"acceptance_revision"`
		RunnerKind         string             `json:"runner_kind"`
		RunnerRevision     string             `json:"runner_revision"`
		Contract           AcceptanceContract `json:"contract"`
	}{binding.TaskID, binding.MissionID, binding.AcceptanceRevision, binding.RunnerKind, binding.RunnerRevision, binding.Contract}
	raw, err := json.Marshal(canonical)
	if err != nil {
		panic("taskvalidation: canonical binding is not serializable: " + err.Error())
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func validPlaceholders(value string) bool {
	for {
		start := strings.Index(value, "{{")
		if start < 0 {
			return !strings.Contains(value, "}}")
		}
		if strings.Contains(value[:start], "}}") {
			return false
		}
		rest := value[start+2:]
		end := strings.Index(rest, "}}")
		if end < 0 {
			return false
		}
		token := rest[:end]
		if token != "mission_id" && token != "task_id" {
			return false
		}
		value = rest[end+2:]
	}
}
