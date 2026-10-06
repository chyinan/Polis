// pattern: Functional Core
package kernel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"polis/internal/core"
)

type ProductDeliveryManifestCompletionCommand struct {
	ArtifactID               string
	ExpectedManifestRevision int64
	RequestID                string
	Evidence                 ReadyProductDeliveryEvidence
}

type DeliveryManifestEvidence struct {
	Reference string
	Digest    string
	Detail    string
}

type ReadyProductDeliveryEvidence struct {
	SourceInputs     DeliveryManifestEvidence
	EnvironmentBuild DeliveryManifestEvidence
	RunInstructions  DeliveryManifestEvidence
	Limitations      DeliveryManifestEvidence
	LicenseSource    DeliveryManifestEvidence
}

type deliveryManifestEvidence = DeliveryManifestEvidence
type readyProductDeliveryEvidence = ReadyProductDeliveryEvidence

type readyProductDeliveryManifestInput struct {
	CompanyID          string
	MissionID          string
	TaskID             string
	ArtifactID         string
	Revision           int64
	ArtifactBytes      int
	ArtifactSHA256     string
	VerificationDetail string
	CreatedAt          time.Time
	Evidence           readyProductDeliveryEvidence
}

type historicalProductDeliveryManifestInput struct {
	CompanyID      string
	MissionID      string
	TaskID         string
	ArtifactID     string
	ArtifactBytes  int
	ArtifactSHA256 string
	CreatedAt      time.Time
}

func validateProductDeliveryManifestCompletionCommand(command ProductDeliveryManifestCompletionCommand) error {
	if !core.ValidID(command.ArtifactID) || !core.ValidID(command.RequestID) || command.ExpectedManifestRevision <= 0 || command.ExpectedManifestRevision == math.MaxInt64 {
		return fmt.Errorf("invalid delivery manifest completion command")
	}
	return validateReadyProductDeliveryEvidence(command.Evidence)
}

func productDeliveryChangeRoute(missionState string) string {
	switch missionState {
	case "active", "paused":
		return "mission_change_request"
	case "succeeded", "ended_not_met", "cancelled":
		return "company_backlog"
	default:
		return ""
	}
}

func productDeliveryDispositionRoute(missionState, dispositionState string) string {
	if dispositionState != "changes_requested" {
		return ""
	}
	return productDeliveryChangeRoute(missionState)
}

func buildReadyProductDeliveryManifest(input readyProductDeliveryManifestInput) (durableProductDeliveryManifest, []byte, string, error) {
	if !core.ValidID(input.CompanyID) || !core.ValidID(input.MissionID) || !core.ValidID(input.TaskID) || !core.ValidID(input.ArtifactID) ||
		input.Revision <= 1 || input.ArtifactBytes <= 0 || input.ArtifactBytes > 64*1024*1024 || !validSHA256(input.ArtifactSHA256) ||
		input.CreatedAt.IsZero() || !validDeliveryManifestDetail(input.VerificationDetail) {
		return durableProductDeliveryManifest{}, nil, "", fmt.Errorf("invalid ready delivery manifest identity or verification evidence")
	}
	if err := validateReadyProductDeliveryEvidence(input.Evidence); err != nil {
		return durableProductDeliveryManifest{}, nil, "", err
	}

	byteSize := strconv.Itoa(input.ArtifactBytes)
	manifest := durableProductDeliveryManifest{
		SchemaVersion: durableProductDeliveryManifestSchema,
		DeliveryID:    input.ArtifactID,
		Revision:      strconv.FormatInt(input.Revision, 10),
		CompanyID:     input.CompanyID,
		MissionID:     input.MissionID,
		TaskID:        input.TaskID,
		ArtifactID:    input.ArtifactID,
		State:         "ready",
		Artifact: durableProductDeliveryArtifact{
			FileName: "artifact.bin",
			ByteSize: byteSize,
			SHA256:   input.ArtifactSHA256,
		},
		Sections: []durableProductDeliveryManifestSection{
			{Key: "source_inputs", State: "available", Detail: formatDeliveryManifestEvidence(input.Evidence.SourceInputs)},
			{Key: "environment_build", State: "available", Detail: formatDeliveryManifestEvidence(input.Evidence.EnvironmentBuild)},
			{Key: "file_inventory", State: "available", Detail: fmt.Sprintf("artifact.bin; bytes=%s; sha256=%s", byteSize, input.ArtifactSHA256)},
			{Key: "run_instructions", State: "available", Detail: formatDeliveryManifestEvidence(input.Evidence.RunInstructions)},
			{Key: "verification", State: "available", Detail: input.VerificationDetail},
			{Key: "limitations", State: "available", Detail: formatDeliveryManifestEvidence(input.Evidence.Limitations)},
			{Key: "license_source", State: "available", Detail: formatDeliveryManifestEvidence(input.Evidence.LicenseSource)},
			{Key: "feedback", State: "not_requested", Detail: "No user feedback has been requested; download or preview is not acceptance."},
		},
		CreatedAt: input.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	for _, section := range manifest.Sections {
		if !validDeliveryManifestDetail(section.Detail) {
			return durableProductDeliveryManifest{}, nil, "", fmt.Errorf("delivery manifest section %q is invalid", section.Key)
		}
	}
	manifestBytes, err := marshalDurableProductDeliveryManifest(manifest)
	if err != nil {
		return durableProductDeliveryManifest{}, nil, "", err
	}
	digest := sha256.Sum256(manifestBytes)
	return manifest, manifestBytes, hex.EncodeToString(digest[:]), nil
}

func buildHistoricalProductDeliveryManifest(input historicalProductDeliveryManifestInput) (durableProductDeliveryManifest, []byte, string, error) {
	if !core.ValidID(input.CompanyID) || !core.ValidID(input.MissionID) || !core.ValidID(input.TaskID) || !core.ValidID(input.ArtifactID) || input.ArtifactBytes <= 0 || input.ArtifactBytes > 64*1024*1024 || !validSHA256(input.ArtifactSHA256) || input.CreatedAt.IsZero() {
		return durableProductDeliveryManifest{}, nil, "", fmt.Errorf("invalid historical delivery manifest identity")
	}
	byteSize := strconv.Itoa(input.ArtifactBytes)
	manifest := durableProductDeliveryManifest{
		SchemaVersion: durableProductDeliveryManifestSchema, DeliveryID: input.ArtifactID, Revision: "1",
		CompanyID: input.CompanyID, MissionID: input.MissionID, TaskID: input.TaskID, ArtifactID: input.ArtifactID, State: "assembling",
		Artifact: durableProductDeliveryArtifact{FileName: "artifact.bin", ByteSize: byteSize, SHA256: input.ArtifactSHA256},
		Sections: []durableProductDeliveryManifestSection{
			{Key: "source_inputs", State: "unavailable", Detail: "Historical backfill has no captured source-input provenance."},
			{Key: "environment_build", State: "unavailable", Detail: "Historical backfill has no captured environment/build evidence."},
			{Key: "file_inventory", State: "available", Detail: fmt.Sprintf("artifact.bin; bytes=%s; sha256=%s", byteSize, input.ArtifactSHA256)},
			{Key: "run_instructions", State: "unavailable", Detail: "Historical backfill has no captured reproducible run instructions."},
			{Key: "verification", State: "unavailable", Detail: "Historical backfill does not infer independent delivery qualification."},
			{Key: "limitations", State: "unavailable", Detail: "Historical backfill has no reviewed limitation declaration."},
			{Key: "license_source", State: "unavailable", Detail: "Historical backfill has no captured license/source declaration."},
			{Key: "feedback", State: "not_requested", Detail: "No user feedback has been requested; download or preview is not acceptance."},
		},
		CreatedAt: input.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	for _, section := range manifest.Sections {
		if !validDeliveryManifestDetail(section.Detail) {
			return durableProductDeliveryManifest{}, nil, "", fmt.Errorf("historical delivery manifest section %q is invalid", section.Key)
		}
	}
	raw, err := marshalDurableProductDeliveryManifest(manifest)
	if err != nil {
		return durableProductDeliveryManifest{}, nil, "", err
	}
	digest := sha256.Sum256(raw)
	return manifest, raw, hex.EncodeToString(digest[:]), nil
}

func marshalDurableProductDeliveryManifest(manifest durableProductDeliveryManifest) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(manifest); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'}), nil
}

func validateStoredProductDeliveryManifest(rawJSON []byte, storedSHA256, companyID, missionID, taskID, artifactID string, revision int64, state string) (durableProductDeliveryManifest, error) {
	var manifest durableProductDeliveryManifest
	decoder := json.NewDecoder(bytes.NewReader(rawJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return durableProductDeliveryManifest{}, fmt.Errorf("invalid durable delivery manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return durableProductDeliveryManifest{}, fmt.Errorf("durable delivery manifest has trailing data")
	}
	canonical, err := marshalDurableProductDeliveryManifest(manifest)
	if err != nil {
		return durableProductDeliveryManifest{}, err
	}
	digest := sha256.Sum256(canonical)
	if hex.EncodeToString(digest[:]) != storedSHA256 || manifest.SchemaVersion != durableProductDeliveryManifestSchema ||
		manifest.DeliveryID != artifactID || manifest.Revision != strconv.FormatInt(revision, 10) || manifest.CompanyID != companyID ||
		manifest.MissionID != missionID || manifest.TaskID != taskID || manifest.ArtifactID != artifactID || manifest.State != state ||
		manifest.Artifact.FileName != "artifact.bin" || !validSHA256(manifest.Artifact.SHA256) || len(manifest.Sections) != 8 {
		return durableProductDeliveryManifest{}, fmt.Errorf("durable delivery manifest identity or digest mismatch")
	}
	byteSize, err := strconv.Atoi(manifest.Artifact.ByteSize)
	if err != nil || byteSize <= 0 || byteSize > 64*1024*1024 || strconv.Itoa(byteSize) != manifest.Artifact.ByteSize || !validDeliveryManifestSectionShape(manifest.Sections) {
		return durableProductDeliveryManifest{}, fmt.Errorf("durable delivery manifest content is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, manifest.CreatedAt); err != nil {
		return durableProductDeliveryManifest{}, fmt.Errorf("durable delivery manifest timestamp is invalid")
	}
	return manifest, nil
}

func validDeliveryManifestSectionShape(sections []durableProductDeliveryManifestSection) bool {
	required := [...]string{"source_inputs", "environment_build", "file_inventory", "run_instructions", "verification", "limitations", "license_source", "feedback"}
	if len(sections) != len(required) {
		return false
	}
	for index, section := range sections {
		if section.Key != required[index] || !validDeliveryManifestDetail(section.Detail) {
			return false
		}
		switch section.State {
		case "available", "unavailable", "missing", "not_requested":
		default:
			return false
		}
	}
	return true
}

func validateReadyProductDeliveryEvidence(evidence readyProductDeliveryEvidence) error {
	for _, item := range []deliveryManifestEvidence{
		evidence.SourceInputs,
		evidence.EnvironmentBuild,
		evidence.RunInstructions,
		evidence.Limitations,
		evidence.LicenseSource,
	} {
		if !validDeliveryManifestReference(item.Reference) || !validSHA256(item.Digest) || !validDeliveryManifestDetail(item.Detail) {
			return fmt.Errorf("delivery evidence reference, digest, or detail is invalid")
		}
	}
	if !validContentAddressedDeliveryDeclaration(evidence.Limitations, "polis.delivery.limitations@") || !validContentAddressedDeliveryDeclaration(evidence.LicenseSource, "polis.delivery.license-source@") {
		return fmt.Errorf("delivery limitation or license/source evidence is not content-addressed")
	}
	return nil
}

func validContentAddressedDeliveryDeclaration(evidence deliveryManifestEvidence, referencePrefix string) bool {
	if !strings.HasPrefix(evidence.Reference, referencePrefix) {
		return false
	}
	digest := sha256.Sum256([]byte(evidence.Detail))
	return hex.EncodeToString(digest[:]) == evidence.Digest
}

func validDeliveryManifestReference(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00') && strings.TrimSpace(value) == value && len(value) > 0 && len(value) <= 160
}

func formatDeliveryManifestEvidence(evidence deliveryManifestEvidence) string {
	return fmt.Sprintf("reference=%s; digest=%s; %s", evidence.Reference, evidence.Digest, evidence.Detail)
}

func validDeliveryManifestDetail(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00') && len(strings.TrimSpace(value)) > 0 && len(value) <= 512
}
