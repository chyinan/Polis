// pattern: Functional Core
package workbench

import (
	"errors"
	"time"

	"polis/internal/intake"
)

type TaskInputManifestView struct {
	CompanyID          string                         `json:"companyId"`
	MissionID          string                         `json:"missionId"`
	TaskID             string                         `json:"taskId"`
	ManifestDigest     string                         `json:"manifestDigest"`
	DeliveryStatus     string                         `json:"deliveryStatus"`
	PayloadDigest      *string                        `json:"payloadDigest"`
	IncludedInputIDs   []string                       `json:"includedInputIds"`
	IncludedInputPaths []intake.ModelInputDeliveryRef `json:"includedInputPaths"`
	InputExclusions    []intake.ModelInputExclusion   `json:"inputExclusions"`
	Manifest           intake.ModelInputManifest      `json:"manifest"`
	CreatedAt          string                         `json:"createdAt"`
}

func newTaskInputManifestView(companyID, missionID, taskID, digest, deliveryStatus string, payloadDigest *string, manifest intake.ModelInputManifest, includedRefs []intake.ModelInputDeliveryRef, inputExclusions []intake.ModelInputExclusion, directoryFilesByInputID map[string][]intake.DirectoryInputFile, hasAttempt bool, createdAt time.Time) (TaskInputManifestView, error) {
	if companyID == "" || missionID == "" || taskID == "" || manifest.CompanyID != companyID || manifest.MissionID != missionID || manifest.TaskID != taskID || intake.VerifyModelInputManifest(manifest, digest) != nil || !validTaskInputDeliveryStatus(deliveryStatus) {
		return TaskInputManifestView{}, errors.New("task input manifest scope or digest is invalid")
	}
	if hasAttempt != (payloadDigest != nil) || payloadDigest != nil && !validDeliveryDigest(*payloadDigest) {
		return TaskInputManifestView{}, errors.New("task input payload digest does not match the delivery attempt state")
	}
	includedIDs := make([]string, 0, len(includedRefs))
	seenInputIDs := make(map[string]struct{}, len(includedRefs))
	for _, included := range includedRefs {
		candidate, ok := candidateInputByID(manifest, included.InputID)
		if !ok {
			return TaskInputManifestView{}, errors.New("delivered input reference is outside the manifest candidates")
		}
		if candidate.SourceKind == "upload" && (!(intake.ProviderTextInputEligible(candidate) || intake.ProviderImageInputEligible(candidate)) || included.RelativePath != "" || included.MediaType != candidate.MediaType || included.ByteSize != candidate.ByteSize || included.ContentDigest != candidate.ContentDigest) {
			return TaskInputManifestView{}, errors.New("delivered upload reference differs from its frozen manifest candidate")
		}
		if intake.IsInputArchiveSource(candidate.SourceKind) && included.RelativePath == "" {
			return TaskInputManifestView{}, errors.New("delivered archive file has no relative path")
		}
		if _, exists := seenInputIDs[included.InputID]; !exists {
			seenInputIDs[included.InputID] = struct{}{}
			includedIDs = append(includedIDs, included.InputID)
		}
	}
	for _, excluded := range inputExclusions {
		if _, ok := candidateInputByID(manifest, excluded.InputID); !ok || !validInputExclusionReason(excluded.Reason) {
			return TaskInputManifestView{}, errors.New("task input exclusion is not valid for a manifest candidate")
		}
	}
	if hasAttempt {
		if err := intake.VerifyModelInputDeliverySelection(manifest, includedRefs, inputExclusions, directoryFilesByInputID); err != nil {
			return TaskInputManifestView{}, err
		}
	}
	if !hasAttempt && (len(includedRefs) != 0 || len(inputExclusions) != 0) {
		return TaskInputManifestView{}, errors.New("task input projection contains refs without a delivery attempt")
	}
	if deliveryStatus == "not_required" && (len(includedRefs) != 0 || len(inputExclusions) != 0) {
		return TaskInputManifestView{}, errors.New("task input delivery marked not required despite a selected or excluded input")
	}
	return TaskInputManifestView{
		CompanyID: companyID, MissionID: missionID, TaskID: taskID, ManifestDigest: digest,
		DeliveryStatus: deliveryStatus, PayloadDigest: payloadDigest, IncludedInputIDs: includedIDs, IncludedInputPaths: includedRefs, InputExclusions: inputExclusions,
		Manifest: manifest, CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func candidateInputByID(manifest intake.ModelInputManifest, inputID string) (intake.ModelInputManifestEntry, bool) {
	for _, candidate := range manifest.CandidateInputs {
		if candidate.InputID == inputID {
			return candidate, true
		}
	}
	return intake.ModelInputManifestEntry{}, false
}

func validInputExclusionReason(reason string) bool {
	switch reason {
	case "representation_not_supported", "context_limit":
		return true
	default:
		return false
	}
}

func validTaskInputDeliveryStatus(status string) bool {
	switch status {
	case "not_delivered", "sending", "provider_delivered", "local_context_loaded", "outcome_unknown", "not_sent", "not_required":
		return true
	default:
		return false
	}
}

func validDeliveryDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}
