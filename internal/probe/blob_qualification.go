// pattern: Imperative Shell

package probe

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const r03aT2BlobQualificationLoaderSchema = "r0.3a-blob-durability-consumer-v2"

var ErrR03AT2BlobQualificationStale = errors.New("blob durability qualification is stale")

type R03AT2BlobDurabilityQualification struct {
	LoaderSchemaVersion              string `json:"loader_schema_version"`
	SourceQualificationPath          string `json:"source_qualification_path"`
	SourceQualificationID            string `json:"source_qualification_id"`
	SourceQualificationContentDigest string `json:"source_qualification_content_digest"`
	NormalizedContractDigest         string `json:"normalized_contract_digest"`
	Qualification                    string `json:"qualification"`
	Status                           string `json:"status"`
	MediumConsumed                   int    `json:"medium_consumed"`
	HighConsumed                     int    `json:"high_consumed"`
	ProviderEgress                   int    `json:"provider_egress"`
	EligibleForBackendContinuation3  bool   `json:"eligible_for_backend_continuation_3"`
	FileContentSync                  string `json:"file_content_sync"`
	AtomicFinalize                   string `json:"atomic_finalize"`
	LinuxParentDirectorySync         string `json:"linux_parent_directory_sync"`
	WindowsParentDirectorySync       string `json:"windows_parent_directory_sync"`
	WindowsSmokeStatus               string `json:"windows_smoke_status"`
	WindowsContentHashCorrect        bool   `json:"windows_content_hash_correct"`
	WindowsFinalBlobPresent          bool   `json:"windows_final_blob_present"`
	WindowsAtomicFinalization        string `json:"windows_atomic_finalization"`
	WindowsParentSyncOutcome         string `json:"windows_parent_sync_outcome"`
	WindowsStageFiles                int    `json:"windows_stage_files"`
	WindowsCleanup                   string `json:"windows_cleanup"`
	OSRootTraversal                  string `json:"os_root_traversal"`
	DesignPackModified               bool   `json:"design_pack_modified"`
	TransportForensicsContinued      bool   `json:"transport_forensics_continued"`
}

type r03aBlobQualificationDocument struct {
	Qualification                   string `json:"qualification"`
	Status                          string `json:"status"`
	MediumConsumed                  int    `json:"medium_consumed"`
	HighConsumed                    int    `json:"high_consumed"`
	ProviderEgress                  int    `json:"provider_egress"`
	EligibleForBackendContinuation3 bool   `json:"eligible_for_backend_continuation_3"`
	DurabilityContract              struct {
		FileContentSync            string `json:"file_content_sync"`
		AtomicFinalize             string `json:"atomic_finalize"`
		LinuxParentDirectorySync   string `json:"linux_parent_directory_sync"`
		WindowsParentDirectorySync string `json:"windows_parent_directory_sync"`
	} `json:"durability_contract"`
	Regressions struct {
		OSRootTraversal string `json:"os_root_traversal"`
	} `json:"regressions"`
	WindowsSmoke struct {
		Status                     string `json:"status"`
		ContentHashCorrect         bool   `json:"content_hash_correct"`
		FinalBlobPresent           bool   `json:"final_blob_present"`
		AtomicFinalization         string `json:"atomic_finalization"`
		ParentDirectorySyncOutcome string `json:"parent_directory_sync_outcome"`
		StageFiles                 int    `json:"stage_files"`
		Cleanup                    string `json:"cleanup"`
	} `json:"windows_native_filesystem_smoke"`
	DesignPackModified          bool `json:"design_pack_modified"`
	TransportForensicsContinued bool `json:"transport_forensics_continued"`
}

func LoadR03AT2BlobDurabilityQualification(path string) (R03AT2BlobDurabilityQualification, error) {
	var empty R03AT2BlobDurabilityQualification
	if path == "" {
		return empty, errors.New("blob durability qualification is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return empty, fmt.Errorf("blob durability qualification unavailable: %w", err)
	}
	var document r03aBlobQualificationDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return empty, fmt.Errorf("blob durability qualification is invalid: %w", err)
	}
	contract := document.DurabilityContract
	smoke := document.WindowsSmoke
	if document.Qualification != "R0.3A Blob Durability Windows Qualification" || document.Status != "PASSED" || document.MediumConsumed != 0 || document.HighConsumed != 0 || document.ProviderEgress != 0 || !document.EligibleForBackendContinuation3 || contract.FileContentSync != "forced_on_windows_and_linux_failure_is_fatal" || contract.AtomicFinalize != "rename_failure_is_fatal" || contract.LinuxParentDirectorySync != "strong_file_sync_atomic_rename_parent_directory_fsync" || contract.WindowsParentDirectorySync != "explicitly_classified_unsupported_after_file_sync_and_atomic_rename; weaker_than_unix_and_not_equivalent" || smoke.Status != "PASSED" || !smoke.ContentHashCorrect || !smoke.FinalBlobPresent || smoke.AtomicFinalization != "final_digest_present_no_stage_file" || smoke.ParentDirectorySyncOutcome != "explicitly_unsupported" || smoke.StageFiles != 0 || smoke.Cleanup != "owned_company_directory_removed" || document.Regressions.OSRootTraversal != "PASSED" || document.DesignPackModified || document.TransportForensicsContinued {
		return empty, errors.New("blob durability qualification is stale, incomplete, or outside the continuation boundary")
	}
	normalized := struct {
		FileContentSync            string `json:"file_content_sync"`
		AtomicFinalize             string `json:"atomic_finalize"`
		LinuxParentDirectorySync   string `json:"linux_parent_directory_sync"`
		WindowsParentDirectorySync string `json:"windows_parent_directory_sync"`
	}{contract.FileContentSync, contract.AtomicFinalize, contract.LinuxParentDirectorySync, contract.WindowsParentDirectorySync}
	normalizedRaw, err := json.Marshal(normalized)
	if err != nil {
		return empty, err
	}
	return R03AT2BlobDurabilityQualification{
		LoaderSchemaVersion:              r03aT2BlobQualificationLoaderSchema,
		SourceQualificationPath:          filepath.Clean(path),
		SourceQualificationID:            document.Qualification,
		SourceQualificationContentDigest: digest(raw),
		NormalizedContractDigest:         digest(normalizedRaw),
		Qualification:                    document.Qualification,
		Status:                           document.Status,
		MediumConsumed:                   document.MediumConsumed,
		HighConsumed:                     document.HighConsumed,
		ProviderEgress:                   document.ProviderEgress,
		EligibleForBackendContinuation3:  document.EligibleForBackendContinuation3,
		FileContentSync:                  contract.FileContentSync,
		AtomicFinalize:                   contract.AtomicFinalize,
		LinuxParentDirectorySync:         contract.LinuxParentDirectorySync,
		WindowsParentDirectorySync:       contract.WindowsParentDirectorySync,
		WindowsSmokeStatus:               smoke.Status,
		WindowsContentHashCorrect:        smoke.ContentHashCorrect,
		WindowsFinalBlobPresent:          smoke.FinalBlobPresent,
		WindowsAtomicFinalization:        smoke.AtomicFinalization,
		WindowsParentSyncOutcome:         smoke.ParentDirectorySyncOutcome,
		WindowsStageFiles:                smoke.StageFiles,
		WindowsCleanup:                   smoke.Cleanup,
		OSRootTraversal:                  document.Regressions.OSRootTraversal,
		DesignPackModified:               document.DesignPackModified,
		TransportForensicsContinued:      document.TransportForensicsContinued,
	}, nil
}

func RequireR03AT2BlobDurabilityQualification(path string) (R03AT2BlobDurabilityQualification, error) {
	return LoadR03AT2BlobDurabilityQualification(path)
}

func CompareR03AT2BlobDurabilityQualification(expected, current R03AT2BlobDurabilityQualification) error {
	if expected.LoaderSchemaVersion != current.LoaderSchemaVersion || expected.SourceQualificationPath != current.SourceQualificationPath || expected.SourceQualificationID != current.SourceQualificationID || expected.SourceQualificationContentDigest != current.SourceQualificationContentDigest || expected.NormalizedContractDigest != current.NormalizedContractDigest {
		return ErrR03AT2BlobQualificationStale
	}
	return nil
}

func VerifyR03AT2BlobDurabilityFreshness(path string, current R03AT2BlobDurabilityQualification) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("blob durability freshness unavailable: %w", err)
	}
	var record struct {
		BlobDurabilityQualification R03AT2BlobDurabilityQualification `json:"blob_durability_qualification"`
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		return fmt.Errorf("blob durability freshness is invalid: %w", err)
	}
	if err := CompareR03AT2BlobDurabilityQualification(record.BlobDurabilityQualification, current); err != nil {
		return err
	}
	return nil
}
