// pattern: Functional Core
package intake

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
)

const ModelInputManifestSchema = "polis-model-input-manifest@1"

type MissionInputReference struct {
	InputID       string
	Revision      int64
	RequestID     string
	SourceKind    string
	DisplayName   string
	MediaType     string
	ByteSize      int64
	ContentDigest string
	State         State
}

type ModelInputManifestEntry struct {
	InputID       string `json:"inputId"`
	Revision      int64  `json:"revision"`
	RequestID     string `json:"requestId"`
	SourceKind    string `json:"sourceKind"`
	DisplayName   string `json:"displayName"`
	MediaType     string `json:"mediaType"`
	ByteSize      int64  `json:"byteSize"`
	ContentDigest string `json:"contentDigest"`
	State         State  `json:"state"`
}

type ModelInputManifest struct {
	SchemaVersion   string                    `json:"schemaVersion"`
	CompanyID       string                    `json:"companyId"`
	MissionID       string                    `json:"missionId"`
	TaskID          string                    `json:"taskId"`
	CandidateInputs []ModelInputManifestEntry `json:"candidateInputs"`
	ExcludedInputs  []ModelInputManifestEntry `json:"excludedInputs"`
	DeliveryStatus  string                    `json:"deliveryStatus"`
}

func PrepareModelInputManifest(companyID, missionID, taskID string, revisions []MissionInputReference) (ModelInputManifest, string, error) {
	if companyID == "" || missionID == "" || taskID == "" || len(revisions) > 1000 {
		return ModelInputManifest{}, "", errors.New("invalid model input manifest scope or size")
	}
	ordered := append([]MissionInputReference(nil), revisions...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].InputID != ordered[j].InputID {
			return ordered[i].InputID < ordered[j].InputID
		}
		return ordered[i].Revision < ordered[j].Revision
	})
	latest := make([]MissionInputReference, 0, len(ordered))
	for index, revision := range ordered {
		if err := validateMissionInputReference(revision); err != nil {
			return ModelInputManifest{}, "", err
		}
		if index > 0 && revision.InputID == ordered[index-1].InputID && revision.Revision == ordered[index-1].Revision {
			return ModelInputManifest{}, "", errors.New("duplicate mission input revision")
		}
		if len(latest) == 0 || latest[len(latest)-1].InputID != revision.InputID {
			latest = append(latest, revision)
			continue
		}
		if revision.Revision > latest[len(latest)-1].Revision {
			latest[len(latest)-1] = revision
		}
	}
	manifest := ModelInputManifest{
		SchemaVersion: ModelInputManifestSchema,
		CompanyID:     companyID, MissionID: missionID, TaskID: taskID,
		CandidateInputs: make([]ModelInputManifestEntry, 0, len(latest)),
		ExcludedInputs:  make([]ModelInputManifestEntry, 0),
		DeliveryStatus:  "not_delivered",
	}
	for _, revision := range latest {
		entry := ModelInputManifestEntry{
			InputID: revision.InputID, Revision: revision.Revision, RequestID: revision.RequestID,
			SourceKind: revision.SourceKind, DisplayName: revision.DisplayName, MediaType: revision.MediaType,
			ByteSize: revision.ByteSize, ContentDigest: revision.ContentDigest, State: revision.State,
		}
		if revision.State == StateUsable || (isInputArchiveSource(revision.SourceKind) && revision.State == StatePartial) || (revision.State == StatePartial && isProviderImageSource(revision.SourceKind, revision.MediaType)) {
			manifest.CandidateInputs = append(manifest.CandidateInputs, entry)
		} else {
			manifest.ExcludedInputs = append(manifest.ExcludedInputs, entry)
		}
	}
	digest, err := modelInputManifestDigest(manifest)
	if err != nil {
		return ModelInputManifest{}, "", err
	}
	return manifest, digest, nil
}

func VerifyModelInputManifest(manifest ModelInputManifest, expectedDigest string) error {
	if manifest.SchemaVersion != ModelInputManifestSchema || manifest.CompanyID == "" || manifest.MissionID == "" || manifest.TaskID == "" || manifest.DeliveryStatus != "not_delivered" || len(manifest.CandidateInputs)+len(manifest.ExcludedInputs) > 1000 {
		return errors.New("invalid model input manifest envelope")
	}
	seen := make(map[string]struct{}, len(manifest.CandidateInputs)+len(manifest.ExcludedInputs))
	for _, entry := range manifest.CandidateInputs {
		if entry.State != StateUsable && !(isInputArchiveSource(entry.SourceKind) && entry.State == StatePartial) && !(entry.State == StatePartial && isProviderImageSource(entry.SourceKind, entry.MediaType)) {
			return errors.New("candidate input is not usable")
		}
		if err := validateManifestEntry(entry); err != nil {
			return err
		}
		if _, exists := seen[entry.InputID]; exists {
			return errors.New("duplicate model input manifest entry")
		}
		seen[entry.InputID] = struct{}{}
	}
	for _, entry := range manifest.ExcludedInputs {
		if entry.State == StateUsable {
			return errors.New("excluded input is usable")
		}
		if err := validateManifestEntry(entry); err != nil {
			return err
		}
		if _, exists := seen[entry.InputID]; exists {
			return errors.New("duplicate model input manifest entry")
		}
		seen[entry.InputID] = struct{}{}
	}
	digest, err := modelInputManifestDigest(manifest)
	if err != nil {
		return err
	}
	if digest != expectedDigest {
		return errors.New("model input manifest digest mismatch")
	}
	return nil
}

func validateMissionInputReference(reference MissionInputReference) error {
	if reference.InputID == "" || reference.Revision <= 0 || reference.RequestID == "" || reference.DisplayName == "" || reference.MediaType == "" || reference.ByteSize <= 0 || !validSHA256Digest(reference.ContentDigest) {
		return errors.New("invalid mission input reference")
	}
	if reference.SourceKind != "upload" && reference.SourceKind != "directory_snapshot" && reference.SourceKind != "zip_snapshot" && reference.SourceKind != "pdf_snapshot" && reference.SourceKind != "git_snapshot" {
		return errors.New("unsupported mission input source kind")
	}
	switch reference.State {
	case StateUsable, StatePartial, StateUnsupported, StateUploading, StateStored, StateRejected:
		return nil
	default:
		return errors.New("unsupported mission input state")
	}
}

func validateManifestEntry(entry ModelInputManifestEntry) error {
	return validateMissionInputReference(MissionInputReference{
		InputID: entry.InputID, Revision: entry.Revision, RequestID: entry.RequestID, SourceKind: entry.SourceKind,
		DisplayName: entry.DisplayName, MediaType: entry.MediaType, ByteSize: entry.ByteSize,
		ContentDigest: entry.ContentDigest, State: entry.State,
	})
}

func modelInputManifestDigest(manifest ModelInputManifest) (string, error) {
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validSHA256Digest(value string) bool {
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
