// pattern: Functional Core
package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"polis/internal/core"
)

const (
	RecoveryBackupPackageSchema = "polis-recovery-backup@1"
	RecoveryBackupManifestName  = "manifest.json"
	RecoveryBackupCompleteName  = "manifest.sha256"
	RecoveryBackupDatabaseName  = "database.dump"
	RecoveryBackupCASName       = "cas-manifest.json"
)

type RecoveryBackupDatabase struct {
	FileName        string                    `json:"fileName"`
	ByteSize        int64                     `json:"byteSize"`
	SHA256          string                    `json:"sha256"`
	DatabaseName    string                    `json:"databaseName"`
	DatabaseOwner   string                    `json:"databaseOwner"`
	RuntimeRoleName string                    `json:"runtimeRoleName"`
	PostgresMajor   int                       `json:"postgresMajor"`
	SchemaVersion   int64                     `json:"schemaVersion"`
	Extensions      []RecoveryBackupExtension `json:"extensions"`
}

type RecoveryBackupExtension struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Schema  string `json:"schema"`
}

type RecoveryBackupCAS struct {
	ManifestFileName string `json:"manifestFileName"`
	ManifestSHA256   string `json:"manifestSha256"`
	BlobCount        int    `json:"blobCount"`
	TotalBytes       int64  `json:"totalBytes"`
}

type RecoveryBackupCASEntry struct {
	CompanyID string `json:"companyId"`
	SHA256    string `json:"sha256"`
	ByteSize  int64  `json:"byteSize"`
}

type RecoveryBackupManifest struct {
	SchemaVersion       string                 `json:"schemaVersion"`
	Status              string                 `json:"status"`
	GenerationID        string                 `json:"generationId"`
	CreatedAt           string                 `json:"createdAt"`
	CredentialsIncluded bool                   `json:"credentialsIncluded"`
	Database            RecoveryBackupDatabase `json:"database"`
	CAS                 RecoveryBackupCAS      `json:"cas"`
}

type RecoveryBackupVerificationReport struct {
	Status              string                    `json:"status"`
	GenerationID        string                    `json:"generationId"`
	ManifestSHA256      string                    `json:"manifestSha256"`
	DatabaseSHA256      string                    `json:"databaseSha256"`
	DatabaseByteSize    int64                     `json:"databaseByteSize"`
	DatabaseName        string                    `json:"databaseName"`
	DatabaseOwner       string                    `json:"databaseOwner"`
	RuntimeRoleName     string                    `json:"runtimeRoleName"`
	PostgresMajor       int                       `json:"postgresMajor"`
	SchemaVersion       int64                     `json:"schemaVersion"`
	Extensions          []RecoveryBackupExtension `json:"extensions"`
	RequiredBlobCount   int                       `json:"requiredBlobCount"`
	VerifiedBlobCount   int                       `json:"verifiedBlobCount"`
	VerifiedBlobBytes   int64                     `json:"verifiedBlobBytes"`
	CredentialsIncluded bool                      `json:"credentialsIncluded"`
}

var ErrRecoveryBackupInvalid = errors.New("recovery backup package invalid")
var ErrRecoveryBackupMaintenanceBusy = errors.New("recovery backup maintenance window is unavailable")

func ValidateRecoveryBackupManifest(manifest RecoveryBackupManifest) error {
	if manifest.SchemaVersion != RecoveryBackupPackageSchema || manifest.Status != "COMPLETE" || !validRecoveryGenerationID(manifest.GenerationID) || manifest.CreatedAt == "" || manifest.CredentialsIncluded {
		return fmt.Errorf("%w: manifest identity, status, timestamp, or credential declaration is invalid", ErrRecoveryBackupInvalid)
	}
	if _, err := time.Parse(time.RFC3339Nano, manifest.CreatedAt); err != nil {
		return fmt.Errorf("%w: backup timestamp is invalid", ErrRecoveryBackupInvalid)
	}
	if manifest.Database.FileName != RecoveryBackupDatabaseName || manifest.Database.ByteSize <= 0 || !validRecoveryDigest(manifest.Database.SHA256) || manifest.Database.DatabaseName == "" || !core.ValidID(manifest.Database.DatabaseOwner) || !core.ValidID(manifest.Database.RuntimeRoleName) || manifest.Database.PostgresMajor < 12 || manifest.Database.SchemaVersion <= 0 || manifest.Database.Extensions == nil {
		return fmt.Errorf("%w: database dump declaration is invalid", ErrRecoveryBackupInvalid)
	}
	previousExtension := ""
	for _, extension := range manifest.Database.Extensions {
		if !core.ValidID(extension.Name) || extension.Version == "" || !core.ValidID(extension.Schema) || extension.Name <= previousExtension {
			return fmt.Errorf("%w: database extension inventory is invalid or non-canonical", ErrRecoveryBackupInvalid)
		}
		previousExtension = extension.Name
	}
	if manifest.CAS.ManifestFileName != RecoveryBackupCASName || !validRecoveryDigest(manifest.CAS.ManifestSHA256) || manifest.CAS.BlobCount < 0 || manifest.CAS.TotalBytes < 0 {
		return fmt.Errorf("%w: CAS manifest declaration is invalid", ErrRecoveryBackupInvalid)
	}
	return nil
}

func VerifyRecoveryBackupPackage(packageRoot string) (RecoveryBackupVerificationReport, error) {
	var report RecoveryBackupVerificationReport
	if !filepath.IsAbs(packageRoot) {
		return report, fmt.Errorf("%w: package path must be absolute", ErrRecoveryBackupInvalid)
	}
	rootInfo, err := os.Lstat(packageRoot)
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return report, fmt.Errorf("%w: package root must be an existing non-symlink directory", ErrRecoveryBackupInvalid)
	}
	manifestPath := filepath.Join(packageRoot, RecoveryBackupManifestName)
	manifestRaw, err := readBoundedRegularFile(manifestPath, 4<<20)
	if err != nil {
		return report, fmt.Errorf("%w: manifest unavailable: %v", ErrRecoveryBackupInvalid, err)
	}
	manifestHash := sha256.Sum256(manifestRaw)
	manifestSHA := hex.EncodeToString(manifestHash[:])
	completeRaw, err := readBoundedRegularFile(filepath.Join(packageRoot, RecoveryBackupCompleteName), 256)
	if err != nil || strings.TrimSpace(string(completeRaw)) != manifestSHA {
		return report, fmt.Errorf("%w: manifest completion digest is missing or mismatched", ErrRecoveryBackupInvalid)
	}
	var manifest RecoveryBackupManifest
	if err = json.Unmarshal(manifestRaw, &manifest); err != nil {
		return report, fmt.Errorf("%w: manifest JSON is invalid", ErrRecoveryBackupInvalid)
	}
	if err = ValidateRecoveryBackupManifest(manifest); err != nil {
		return report, err
	}
	databasePath := filepath.Join(packageRoot, manifest.Database.FileName)
	databaseInfo, err := os.Lstat(databasePath)
	if err != nil || databaseInfo.Mode()&os.ModeSymlink != 0 || !databaseInfo.Mode().IsRegular() || databaseInfo.Size() != manifest.Database.ByteSize {
		return report, fmt.Errorf("%w: database dump is missing, unsafe, or has a different size", ErrRecoveryBackupInvalid)
	}
	databaseSHA, _, err := hashRegularFile(databasePath)
	if err != nil || databaseSHA != manifest.Database.SHA256 {
		return report, fmt.Errorf("%w: database dump digest mismatch", ErrRecoveryBackupInvalid)
	}
	casManifestPath := filepath.Join(packageRoot, manifest.CAS.ManifestFileName)
	casRaw, err := readBoundedRegularFile(casManifestPath, 64<<20)
	if err != nil {
		return report, fmt.Errorf("%w: CAS manifest unavailable: %v", ErrRecoveryBackupInvalid, err)
	}
	casHash := sha256.Sum256(casRaw)
	if hex.EncodeToString(casHash[:]) != manifest.CAS.ManifestSHA256 {
		return report, fmt.Errorf("%w: CAS manifest digest mismatch", ErrRecoveryBackupInvalid)
	}
	var casManifest struct {
		SchemaVersion string                   `json:"schemaVersion"`
		Entries       []RecoveryBackupCASEntry `json:"entries"`
	}
	if err = json.Unmarshal(casRaw, &casManifest); err != nil || casManifest.SchemaVersion != RecoveryBackupPackageSchema {
		return report, fmt.Errorf("%w: CAS manifest schema is invalid", ErrRecoveryBackupInvalid)
	}
	if err = validateRecoveryCASEntries(casManifest.Entries, manifest.CAS.BlobCount, manifest.CAS.TotalBytes); err != nil {
		return report, err
	}
	verifiedBytes, err := verifyRecoveryBackupCAS(filepath.Join(packageRoot, "cas"), casManifest.Entries)
	if err != nil {
		return report, err
	}
	if err = verifyRecoveryBackupLayout(packageRoot); err != nil {
		return report, err
	}
	return RecoveryBackupVerificationReport{
		Status: "PASSED", GenerationID: manifest.GenerationID, ManifestSHA256: manifestSHA,
		DatabaseSHA256: databaseSHA, DatabaseByteSize: manifest.Database.ByteSize, DatabaseName: manifest.Database.DatabaseName,
		DatabaseOwner: manifest.Database.DatabaseOwner, RuntimeRoleName: manifest.Database.RuntimeRoleName,
		PostgresMajor: manifest.Database.PostgresMajor, SchemaVersion: manifest.Database.SchemaVersion, Extensions: manifest.Database.Extensions,
		RequiredBlobCount: manifest.CAS.BlobCount, VerifiedBlobCount: manifest.CAS.BlobCount,
		VerifiedBlobBytes: verifiedBytes, CredentialsIncluded: manifest.CredentialsIncluded,
	}, nil
}

func verifyRecoveryBackupCAS(root string, entries []RecoveryBackupCASEntry) (int64, error) {
	info, err := os.Lstat(root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return 0, fmt.Errorf("%w: CAS package root is unavailable or unsafe", ErrRecoveryBackupInvalid)
	}
	var total int64
	for _, entry := range entries {
		path := filepath.Join(root, entry.CompanyID, entry.SHA256)
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return total, fmt.Errorf("%w: CAS entry escapes package root", ErrRecoveryBackupInvalid)
		}
		fileInfo, statErr := os.Lstat(path)
		if statErr != nil || fileInfo.Mode()&os.ModeSymlink != 0 || !fileInfo.Mode().IsRegular() || fileInfo.Size() != entry.ByteSize {
			return total, fmt.Errorf("%w: CAS blob is missing, unsafe, or has a different size", ErrRecoveryBackupInvalid)
		}
		digest, size, hashErr := hashRegularFile(path)
		if hashErr != nil || size != entry.ByteSize || digest != entry.SHA256 {
			return total, fmt.Errorf("%w: CAS blob digest mismatch", ErrRecoveryBackupInvalid)
		}
		total += size
	}
	remaining := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		remaining[entry.CompanyID+"/"+entry.SHA256] = struct{}{}
	}
	err = filepath.WalkDir(root, func(path string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		info, statErr := os.Lstat(path)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return ErrRecoveryBackupInvalid
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return ErrRecoveryBackupInvalid
		}
		segments := strings.Split(relative, string(filepath.Separator))
		if info.IsDir() {
			if len(segments) != 1 || !core.ValidID(segments[0]) {
				return ErrRecoveryBackupInvalid
			}
			return nil
		}
		if len(segments) != 2 || !validRecoveryDigest(segments[1]) {
			return ErrRecoveryBackupInvalid
		}
		key := segments[0] + "/" + segments[1]
		if _, expected := remaining[key]; !expected {
			return ErrRecoveryBackupInvalid
		}
		delete(remaining, key)
		return nil
	})
	if err != nil || len(remaining) != 0 {
		return total, fmt.Errorf("%w: CAS directory contains unmanifested or missing entries", ErrRecoveryBackupInvalid)
	}
	return total, nil
}

func validateRecoveryCASEntries(entries []RecoveryBackupCASEntry, expectedCount int, expectedBytes int64) error {
	if len(entries) != expectedCount {
		return fmt.Errorf("%w: CAS entry count differs from package declaration", ErrRecoveryBackupInvalid)
	}
	var totalBytes int64
	previousKey := ""
	for _, entry := range entries {
		if !core.ValidID(entry.CompanyID) || !validRecoveryDigest(entry.SHA256) || entry.ByteSize <= 0 {
			return fmt.Errorf("%w: CAS entry is invalid", ErrRecoveryBackupInvalid)
		}
		key := entry.CompanyID + "/" + entry.SHA256
		if key <= previousKey {
			return fmt.Errorf("%w: CAS entries are duplicated or not canonically ordered", ErrRecoveryBackupInvalid)
		}
		previousKey = key
		if totalBytes > int64(^uint64(0)>>1)-entry.ByteSize {
			return fmt.Errorf("%w: CAS byte total overflows", ErrRecoveryBackupInvalid)
		}
		totalBytes += entry.ByteSize
	}
	if totalBytes != expectedBytes {
		return fmt.Errorf("%w: CAS total byte count differs from entries", ErrRecoveryBackupInvalid)
	}
	return nil
}

func verifyRecoveryBackupLayout(root string) error {
	allowed := map[string]struct{}{
		RecoveryBackupManifestName: {}, RecoveryBackupCompleteName: {}, RecoveryBackupDatabaseName: {}, RecoveryBackupCASName: {}, "cas": {},
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("%w: package directory cannot be read", ErrRecoveryBackupInvalid)
	}
	for _, entry := range entries {
		if _, ok := allowed[entry.Name()]; !ok {
			return fmt.Errorf("%w: package contains an unmanifested top-level entry", ErrRecoveryBackupInvalid)
		}
		info, infoErr := os.Lstat(filepath.Join(root, entry.Name()))
		if infoErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: package contains an unsafe top-level entry", ErrRecoveryBackupInvalid)
		}
	}
	return nil
}

func readBoundedRegularFile(path string, maxBytes int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxBytes {
		return nil, ErrRecoveryBackupInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, maxBytes+1))
}

func hashRegularFile(path string) (string, int64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", 0, ErrRecoveryBackupInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", size, err
	}
	if size != info.Size() {
		return "", size, ErrRecoveryBackupInvalid
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func validRecoveryDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func validRecoveryGenerationID(value string) bool {
	if len(value) != 32 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 16 && hex.EncodeToString(decoded) == value
}

func canonicalRecoveryCASEntries(entries []RecoveryBackupCASEntry) {
	sort.Slice(entries, func(left, right int) bool {
		if entries[left].CompanyID != entries[right].CompanyID {
			return entries[left].CompanyID < entries[right].CompanyID
		}
		return entries[left].SHA256 < entries[right].SHA256
	})
}
