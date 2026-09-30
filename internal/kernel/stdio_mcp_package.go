// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/capabilitysource"
	"polis/internal/core"
)

type StdioMCPPackageRevision struct {
	CompanyID      string          `json:"companyId"`
	ID             string          `json:"id"`
	ServerID       string          `json:"serverId"`
	Revision       string          `json:"revision"`
	ManifestDigest string          `json:"manifestDigest"`
	Manifest       json.RawMessage `json:"manifest"`
	CreatedAt      string          `json:"createdAt"`
}

type StdioMCPPackageInput struct {
	ServerID  string
	Revision  string
	Archive   []byte
	RequestID string
}

func (k *Kernel) TXImportStdioMCPPackage(ctx context.Context, companyID string, input StdioMCPPackageInput) (StdioMCPPackageRevision, error) {
	revision := strings.TrimSpace(input.Revision)
	serverID := strings.TrimSpace(input.ServerID)
	if !core.ValidID(companyID) || !core.ValidID(input.RequestID) || (serverID != "" && !core.ValidID(serverID)) || revision == "" || revision != input.Revision || !validReadOnlySkillRevision(revision) || len(input.Archive) == 0 {
		return StdioMCPPackageRevision{}, core.Malformed
	}
	var companyState string
	if err := k.pool.QueryRow(ctx, "SELECT state FROM companies WHERE id=$1", companyID).Scan(&companyState); errors.Is(err, pgx.ErrNoRows) {
		return StdioMCPPackageRevision{}, core.OutOfScope
	} else if err != nil {
		return StdioMCPPackageRevision{}, err
	}
	if err := validateCapabilityCatalogCompanyState(companyState); err != nil {
		return StdioMCPPackageRevision{}, err
	}
	bundle, err := capabilitysource.PrepareStdioMCPBundle(input.Archive)
	if err != nil {
		return StdioMCPPackageRevision{}, core.Malformed
	}
	manifestJSON, err := json.Marshal(bundle.Manifest)
	if err != nil || string(manifestJSON) != string(bundle.ManifestJSON) || capabilitysource.ValidateStdioMCPBundleManifest(bundle.Manifest, bundle.ManifestDigest) != nil {
		return StdioMCPPackageRevision{}, core.Integrity
	}
	argsJSON, err := json.Marshal(bundle.Manifest.Args)
	if err != nil {
		return StdioMCPPackageRevision{}, core.Integrity
	}
	descriptorDigest, err := canonicalMCPDescriptorDigest(bundle.Manifest.Name, "stdio", bundle.Manifest.Command, argsJSON)
	if err != nil {
		return StdioMCPPackageRevision{}, err
	}
	if serverID == "" {
		serverID = stableCapabilityID("mcp", companyID, input.RequestID)
	} else if err = k.verifyStdioMCPDefinitionForBundle(ctx, companyID, serverID, bundle.Manifest); err != nil {
		return StdioMCPPackageRevision{}, err
	}
	packageID := stableCapabilityID("mcp-package", companyID, input.RequestID)
	inputFingerprint := struct {
		PackageID      string
		ServerID       string
		Revision       string
		ManifestDigest string
		Manifest       json.RawMessage
	}{packageID, serverID, revision, bundle.ManifestDigest, manifestJSON}
	wantFingerprint := fingerprint(struct {
		Op    string
		Input any
	}{"capability.mcp.package.import.stdio", inputFingerprint})
	unlock, err := k.lockCapabilityMCPPackageImport(ctx, companyID, input.RequestID, serverID, revision)
	if err != nil {
		return StdioMCPPackageRevision{}, err
	}
	defer unlock()
	if err = k.preflightStdioMCPPackageWrite(ctx, companyID, serverID, revision, input.RequestID, wantFingerprint); err != nil {
		return StdioMCPPackageRevision{}, err
	}
	verifiedFiles := make([]capabilitysource.StdioMCPBundleFile, 0, len(bundle.Files))
	for _, file := range bundle.Files {
		digest, storeErr := putBlob(k.root, companyID, file.Content)
		if storeErr != nil {
			return StdioMCPPackageRevision{}, storeErr
		}
		if digest != file.ContentSHA256 {
			return StdioMCPPackageRevision{}, core.Integrity
		}
		stored, readErr := readBlob(k.root, companyID, digest)
		if readErr != nil {
			return StdioMCPPackageRevision{}, readErr
		}
		verifiedFiles = append(verifiedFiles, capabilitysource.StdioMCPBundleFile{
			RelativePath: file.RelativePath, MediaType: file.MediaType, ByteSize: file.ByteSize,
			ContentSHA256: file.ContentSHA256, Content: stored,
		})
	}
	if err = capabilitysource.VerifyStdioMCPBundle(bundle.Manifest, bundle.ManifestDigest, verifiedFiles); err != nil {
		return StdioMCPPackageRevision{}, core.Integrity
	}
	requestLockedContext := withCapabilityMCPServerLock(withCapabilitySourceRequestLock(ctx, companyID, input.RequestID), companyID, serverID)
	_, err = k.TXWrite(requestLockedContext, k.LocalScope(companyID), nil, input.RequestID, "capability.mcp.package.import.stdio", inputFingerprint, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if strings.TrimSpace(input.ServerID) == "" {
			if _, err := tx.Exec(ctx, `INSERT INTO mcp_server_definitions(company_id,id,name,transport,endpoint,command,args,descriptor_digest,status)
VALUES($1,$2,$3,'stdio',NULL,$4,$5,$6,'unverified')`, companyID, serverID, bundle.Manifest.Name, bundle.Manifest.Command, argsJSON, descriptorDigest); err != nil {
				if isUniqueViolation(err) {
					return Receipt{}, core.Conflict
				}
				return Receipt{}, err
			}
		} else if err := verifyStdioMCPDefinitionForBundleTx(ctx, tx, companyID, serverID, bundle.Manifest); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO mcp_server_package_revisions(company_id,package_revision_id,server_id,revision,manifest_digest,manifest,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7)`, companyID, packageID, serverID, revision, bundle.ManifestDigest, manifestJSON, input.RequestID); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		if err := revokeStdioMCPRuntimeQualifications(ctx, tx, companyID, serverID, descriptorDigest, input.RequestID, "MCP package revision changed"); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: packageID, Status: "candidate"}, nil
	})
	if err != nil {
		return StdioMCPPackageRevision{}, err
	}
	return k.getStdioMCPPackageRevision(ctx, companyID, packageID)
}

func (k *Kernel) ReadStdioMCPPackageFiles(ctx context.Context, companyID, serverID, packageRevisionID string) ([]capabilitysource.StdioMCPBundleFile, error) {
	if !core.ValidID(companyID) || !core.ValidID(serverID) || !core.ValidID(packageRevisionID) {
		return nil, core.Malformed
	}
	var manifestJSON []byte
	var manifestDigest string
	err := k.pool.QueryRow(ctx, `SELECT manifest_digest,manifest FROM mcp_server_package_revisions
WHERE company_id=$1 AND server_id=$2 AND package_revision_id=$3`, companyID, serverID, packageRevisionID).Scan(&manifestDigest, &manifestJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, core.OutOfScope
	}
	if err != nil {
		return nil, err
	}
	var manifest capabilitysource.StdioMCPBundleManifestData
	if json.Unmarshal(manifestJSON, &manifest) != nil || capabilitysource.ValidateStdioMCPBundleManifest(manifest, manifestDigest) != nil {
		return nil, core.Integrity
	}
	files := make([]capabilitysource.StdioMCPBundleFile, 0, len(manifest.Files))
	for _, entry := range manifest.Files {
		content, readErr := readBlob(k.root, companyID, entry.ContentSHA256)
		if readErr != nil {
			return nil, core.Integrity
		}
		files = append(files, capabilitysource.StdioMCPBundleFile{
			RelativePath: entry.RelativePath, MediaType: entry.MediaType, ByteSize: entry.ByteSize,
			ContentSHA256: entry.ContentSHA256, Content: content,
		})
	}
	if err = capabilitysource.VerifyStdioMCPBundle(manifest, manifestDigest, files); err != nil {
		return nil, core.Integrity
	}
	return files, nil
}

func (k *Kernel) listStdioMCPPackageRevisions(ctx context.Context, tx pgx.Tx, companyID string) ([]StdioMCPPackageRevision, error) {
	rows, err := tx.Query(ctx, `SELECT company_id,package_revision_id,server_id,revision,manifest_digest,manifest,created_at::text
FROM mcp_server_package_revisions WHERE company_id=$1 ORDER BY created_at DESC,package_revision_id`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	packages := make([]StdioMCPPackageRevision, 0)
	for rows.Next() {
		var item StdioMCPPackageRevision
		if err = rows.Scan(&item.CompanyID, &item.ID, &item.ServerID, &item.Revision, &item.ManifestDigest, &item.Manifest, &item.CreatedAt); err != nil {
			return nil, err
		}
		var manifest capabilitysource.StdioMCPBundleManifestData
		if json.Unmarshal(item.Manifest, &manifest) != nil || capabilitysource.ValidateStdioMCPBundleManifest(manifest, item.ManifestDigest) != nil {
			return nil, core.Integrity
		}
		packages = append(packages, item)
	}
	return packages, rows.Err()
}

func (k *Kernel) getStdioMCPPackageRevision(ctx context.Context, companyID, packageRevisionID string) (StdioMCPPackageRevision, error) {
	var item StdioMCPPackageRevision
	err := k.pool.QueryRow(ctx, `SELECT company_id,package_revision_id,server_id,revision,manifest_digest,manifest,created_at::text
FROM mcp_server_package_revisions WHERE company_id=$1 AND package_revision_id=$2`, companyID, packageRevisionID).Scan(
		&item.CompanyID, &item.ID, &item.ServerID, &item.Revision, &item.ManifestDigest, &item.Manifest, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return StdioMCPPackageRevision{}, core.Integrity
	}
	if err != nil {
		return StdioMCPPackageRevision{}, err
	}
	var manifest capabilitysource.StdioMCPBundleManifestData
	if json.Unmarshal(item.Manifest, &manifest) != nil || capabilitysource.ValidateStdioMCPBundleManifest(manifest, item.ManifestDigest) != nil {
		return StdioMCPPackageRevision{}, core.Integrity
	}
	item.CompanyID = companyID
	return item, nil
}

func (k *Kernel) verifyStdioMCPDefinitionForBundle(ctx context.Context, companyID, serverID string, manifest capabilitysource.StdioMCPBundleManifestData) error {
	var name, transport, descriptorDigest string
	var command *string
	var argsJSON []byte
	err := k.pool.QueryRow(ctx, `SELECT name,transport,command,args,descriptor_digest FROM mcp_server_definitions
WHERE company_id=$1 AND id=$2`, companyID, serverID).Scan(&name, &transport, &command, &argsJSON, &descriptorDigest)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	}
	if err != nil {
		return err
	}
	return verifyStdioMCPDefinitionValues(name, transport, command, argsJSON, descriptorDigest, manifest)
}

func verifyStdioMCPDefinitionForBundleTx(ctx context.Context, tx pgx.Tx, companyID, serverID string, manifest capabilitysource.StdioMCPBundleManifestData) error {
	var name, transport, descriptorDigest string
	var command *string
	var argsJSON []byte
	err := tx.QueryRow(ctx, `SELECT name,transport,command,args,descriptor_digest FROM mcp_server_definitions
WHERE company_id=$1 AND id=$2 FOR SHARE`, companyID, serverID).Scan(&name, &transport, &command, &argsJSON, &descriptorDigest)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	}
	if err != nil {
		return err
	}
	return verifyStdioMCPDefinitionValues(name, transport, command, argsJSON, descriptorDigest, manifest)
}

func verifyStdioMCPDefinitionValues(name, transport string, command *string, argsJSON []byte, descriptorDigest string, manifest capabilitysource.StdioMCPBundleManifestData) error {
	if command == nil || transport != "stdio" || name != manifest.Name || *command != manifest.Command {
		return core.Denied
	}
	var args []string
	if json.Unmarshal(argsJSON, &args) != nil || !reflect.DeepEqual(args, manifest.Args) {
		return core.Denied
	}
	canonicalArgs, err := json.Marshal(args)
	if err != nil {
		return core.Integrity
	}
	computedDescriptorDigest, err := canonicalMCPDescriptorDigest(name, transport, *command, canonicalArgs)
	if err != nil || computedDescriptorDigest != descriptorDigest {
		return core.Integrity
	}
	return nil
}

func (k *Kernel) lockCapabilityMCPPackageImport(ctx context.Context, companyID, requestID, serverID, revision string) (func(), error) {
	return k.lockCapabilitySourceImport(ctx, companyID, requestID, "mcp", serverID, revision)
}

func (k *Kernel) preflightStdioMCPPackageWrite(ctx context.Context, companyID, serverID, revision, requestID, wantFingerprint string) error {
	var existingFingerprint string
	err := k.pool.QueryRow(ctx, `SELECT fingerprint FROM receipts WHERE company_id=$1 AND actor='local-owner' AND key=$2`, companyID, requestID).Scan(&existingFingerprint)
	if err == nil {
		if existingFingerprint != wantFingerprint {
			return core.Conflict
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var existing int
	err = k.pool.QueryRow(ctx, `SELECT 1 FROM mcp_server_package_revisions WHERE company_id=$1 AND server_id=$2 AND revision=$3`, companyID, serverID, revision).Scan(&existing)
	if err == nil {
		return core.Conflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return nil
}
