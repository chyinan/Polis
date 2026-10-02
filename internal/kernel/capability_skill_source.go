// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/capabilitysource"
	"polis/internal/core"
)

const readOnlySkillSourceRef = "local-upload://read-only-skill-bundle"

var readOnlySkillRevisionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)

type ReadOnlySkillPackageInput struct {
	Revision string
	Archive  []byte
}

func (k *Kernel) TXImportReadOnlySkillPackage(ctx context.Context, companyID string, input ReadOnlySkillPackageInput, requestID string) (SkillRevision, error) {
	revision := strings.TrimSpace(input.Revision)
	if !core.ValidID(companyID) || !core.ValidID(requestID) || revision != input.Revision || !validReadOnlySkillRevision(revision) || len(input.Archive) == 0 {
		return SkillRevision{}, core.Malformed
	}
	var companyState string
	if err := k.pool.QueryRow(ctx, "SELECT state FROM companies WHERE id=$1", companyID).Scan(&companyState); errors.Is(err, pgx.ErrNoRows) {
		return SkillRevision{}, core.OutOfScope
	} else if err != nil {
		return SkillRevision{}, err
	}
	if err := validateCapabilityCatalogCompanyState(companyState); err != nil {
		return SkillRevision{}, err
	}
	bundle, err := capabilitysource.PrepareReadOnlySkillBundle(input.Archive)
	if err != nil {
		return SkillRevision{}, core.Malformed
	}
	if !core.ValidID(bundle.Manifest.Name) {
		return SkillRevision{}, core.Malformed
	}
	manifestJSON, err := json.Marshal(bundle.Manifest)
	if err != nil || string(manifestJSON) != string(bundle.ManifestJSON) {
		return SkillRevision{}, core.Integrity
	}
	id := stableCapabilityID("skill", companyID, requestID)
	fingerprint := struct {
		ID            string
		Revision      string
		SourceRef     string
		ContentDigest string
		Manifest      json.RawMessage
	}{id, revision, readOnlySkillSourceRef, bundle.ContentDigest, manifestJSON}
	unlock, err := k.lockCapabilitySourceImport(ctx, companyID, requestID, "company", bundle.Manifest.Name, revision)
	if err != nil {
		return SkillRevision{}, err
	}
	defer unlock()
	if err = k.preflightReadOnlySkillPackageWrite(ctx, companyID, bundle.Manifest.Name, revision, requestID, fingerprint); err != nil {
		return SkillRevision{}, err
	}

	storedFiles := make([]capabilitysource.SkillBundleFile, 0, len(bundle.Files))
	for _, file := range bundle.Files {
		digest, storeErr := k.putBlobWithClaim(ctx, companyID, file.Content)
		if storeErr != nil {
			return SkillRevision{}, storeErr
		}
		if digest != file.ContentSHA256 {
			return SkillRevision{}, core.Integrity
		}
		stored, readErr := readBlob(k.root, companyID, digest)
		if readErr != nil {
			return SkillRevision{}, readErr
		}
		storedFiles = append(storedFiles, capabilitysource.SkillBundleFile{
			RelativePath: file.RelativePath, MediaType: file.MediaType, ByteSize: file.ByteSize,
			ContentSHA256: file.ContentSHA256, Content: stored,
		})
	}
	if err = capabilitysource.VerifyReadOnlySkillBundle(bundle.Manifest, bundle.ContentDigest, storedFiles); err != nil {
		return SkillRevision{}, core.Integrity
	}
	requestLockedContext := withCapabilitySourceRequestLock(ctx, companyID, requestID)
	_, err = k.TXWrite(requestLockedContext, k.LocalScope(companyID), nil, requestID, "capability.skill.import.readonly", fingerprint, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO skill_revisions(company_id,id,publisher_scope,package_id,revision,display_name,source_ref,content_digest,manifest,status)
VALUES($1,$2,'company',$3,$4,$5,$6,$7,$8,'candidate')`, companyID, id, bundle.Manifest.Name, revision, bundle.Manifest.Name, readOnlySkillSourceRef, bundle.ContentDigest, manifestJSON); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: id, Status: "candidate"}, nil
	})
	if err != nil {
		return SkillRevision{}, err
	}
	return k.getSkillRevision(ctx, companyID, id)
}

// lockSkillPackageRevision rejects concurrent changes before they can write CAS blobs.
func (k *Kernel) lockSkillPackageRevision(ctx context.Context, companyID, publisherScope, packageID, revision string) (func(), error) {
	identity := companyID + "\x00" + publisherScope + "\x00" + packageID + "\x00" + revision
	digest := sha256.Sum256([]byte(identity))
	lockKey := int64(binary.BigEndian.Uint64(digest[:8]))
	// Advisory lock sessions must not consume connections from the transaction
	// pool: import callers hold a lock while issuing preflight and write queries.
	connection, err := k.lockPool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	var locked bool
	if err = connection.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockKey).Scan(&locked); err != nil {
		// The server may have acquired the session lock even if cancellation or
		// a transport error hid the result. Never return that session to the pool.
		discardSkillPackageLockConnection(connection)
		return nil, err
	}
	if !locked {
		connection.Release()
		return nil, core.Conflict
	}
	return func() {
		unlockContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var unlocked bool
		if unlockErr := connection.QueryRow(unlockContext, "SELECT pg_advisory_unlock($1)", lockKey).Scan(&unlocked); unlockErr != nil || !unlocked {
			discardSkillPackageLockConnection(connection)
			return
		}
		connection.Release()
	}, nil
}

func discardSkillPackageLockConnection(connection *pgxpool.Conn) {
	conn := connection.Hijack()
	closeContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = conn.Close(closeContext)
}

func (k *Kernel) preflightReadOnlySkillPackageWrite(ctx context.Context, companyID, packageID, revision, requestID string, input any) error {
	want := fingerprint(struct {
		Op    string
		Input any
	}{"capability.skill.import.readonly", input})
	var existingFingerprint string
	err := k.pool.QueryRow(ctx, `SELECT fingerprint FROM receipts WHERE company_id=$1 AND actor='local-owner' AND key=$2`, companyID, requestID).Scan(&existingFingerprint)
	if err == nil {
		if existingFingerprint != want {
			return core.Conflict
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var existing int
	err = k.pool.QueryRow(ctx, `SELECT 1 FROM skill_revisions WHERE company_id=$1 AND publisher_scope='company' AND package_id=$2 AND revision=$3`, companyID, packageID, revision).Scan(&existing)
	if err == nil {
		return core.Conflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return nil
}

func validReadOnlySkillRevision(revision string) bool {
	return readOnlySkillRevisionPattern.MatchString(revision)
}

func (k *Kernel) verifyStoredReadOnlySkillPackage(ctx context.Context, companyID, skillID string) (bool, error) {
	var sourceRef string
	var contentDigest string
	var manifestJSON []byte
	err := k.pool.QueryRow(ctx, `SELECT source_ref,content_digest,manifest FROM skill_revisions WHERE company_id=$1 AND id=$2`, companyID, skillID).Scan(&sourceRef, &contentDigest, &manifestJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var manifest capabilitysource.ReadOnlySkillManifest
	if err = json.Unmarshal(manifestJSON, &manifest); err != nil {
		return false, core.Integrity
	}
	if manifest.SchemaVersion != capabilitysource.ReadOnlySkillBundleSchema {
		return false, nil
	}
	if sourceRef != readOnlySkillSourceRef {
		return false, core.Integrity
	}
	files := make([]capabilitysource.SkillBundleFile, 0, len(manifest.Files))
	for _, entry := range manifest.Files {
		if entry.ContentSHA256 == "" {
			return false, core.Integrity
		}
		content, readErr := readBlob(k.root, companyID, entry.ContentSHA256)
		if readErr != nil {
			return false, core.Integrity
		}
		files = append(files, capabilitysource.SkillBundleFile{
			RelativePath: entry.RelativePath, MediaType: entry.MediaType, ByteSize: entry.ByteSize,
			ContentSHA256: entry.ContentSHA256, Content: content,
		})
	}
	if err = capabilitysource.VerifyReadOnlySkillBundle(manifest, contentDigest, files); err != nil {
		return false, core.Integrity
	}
	return true, nil
}
