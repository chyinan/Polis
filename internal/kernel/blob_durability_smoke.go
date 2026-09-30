// pattern: Imperative Shell

package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
)

type BlobDurabilitySmokeReport struct {
	Status                      string `json:"status"`
	Root                        string `json:"root"`
	ContentDigest               string `json:"content_digest"`
	ReadBackDigest              string `json:"read_back_digest"`
	FinalBlobPresent            bool   `json:"final_blob_present"`
	ContentHashCorrect          bool   `json:"content_hash_correct"`
	StageFiles                  int    `json:"stage_files"`
	AtomicFinalization          string `json:"atomic_finalization"`
	ParentDirectorySyncContract string `json:"parent_directory_sync_contract"`
	ParentDirectorySyncOutcome  string `json:"parent_directory_sync_outcome"`
	Cleanup                     string `json:"cleanup"`
	Error                       string `json:"error,omitempty"`
}

func RunBlobDurabilitySmoke(root string) (report BlobDurabilitySmokeReport, err error) {
	report = BlobDurabilitySmokeReport{Status: "failed", Root: root, ParentDirectorySyncContract: parentDirectorySyncContract()}
	if root == "" {
		return report, errors.New("blob durability smoke root is required")
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return report, err
	}
	company := "blob-durability-smoke-" + newID()
	content := []byte("polis-r0.3a-blob-durability-smoke-v1\n")
	report.ContentDigest = blobContentDigest(content)
	actualDigest, err := putBlob(root, company, content)
	if err != nil {
		return report, err
	}
	if actualDigest != report.ContentDigest {
		return report, errors.New("blob durability smoke digest changed during publish")
	}
	handle, err := blobDir(root, company)
	if err != nil {
		return report, err
	}
	parentSyncErr := syncParentDirectory(handle)
	switch {
	case parentSyncErr == nil:
		report.ParentDirectorySyncOutcome = "strong"
	case errors.Is(parentSyncErr, ErrParentDirectorySyncUnsupported):
		report.ParentDirectorySyncOutcome = "explicitly_unsupported"
	default:
		handle.Close()
		return report, parentSyncErr
	}
	directory, err := handle.Open(".")
	if err != nil {
		handle.Close()
		return report, err
	}
	entries, err := directory.ReadDir(-1)
	closeDirectoryErr := directory.Close()
	if err != nil {
		handle.Close()
		return report, err
	}
	if closeDirectoryErr != nil {
		handle.Close()
		return report, closeDirectoryErr
	}
	for _, entry := range entries {
		if entry.Name() == actualDigest {
			report.FinalBlobPresent = true
		}
		if len(entry.Name()) >= len(".stage-") && entry.Name()[:len(".stage-")] == ".stage-" {
			report.StageFiles++
		}
	}
	if err = handle.Close(); err != nil {
		return report, err
	}
	readBack, err := readBlob(root, company, actualDigest)
	if err != nil {
		return report, err
	}
	report.ReadBackDigest = blobContentDigest(readBack)
	report.ContentHashCorrect = report.ReadBackDigest == report.ContentDigest && string(readBack) == string(content)
	report.AtomicFinalization = "final_digest_present_no_stage_file"
	if err = removeBlobFile(root, company, actualDigest); err != nil {
		return report, err
	}
	cleanupErr := removeBlobCompany(root, company)
	if cleanupErr != nil {
		return report, cleanupErr
	}
	report.Cleanup = "owned_company_directory_removed"
	if !report.FinalBlobPresent || report.StageFiles != 0 || !report.ContentHashCorrect {
		return report, errors.New("blob durability smoke finalization verification failed")
	}
	report.Status = "passed"
	return report, nil
}

func removeBlobCompany(root, company string) error {
	parent, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Remove(company)
}

func removeBlobFile(root, company, name string) error {
	handle, err := blobDir(root, company)
	if err != nil {
		return err
	}
	defer handle.Close()
	return handle.Remove(name)
}

func blobContentDigest(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}
