// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/capabilitysource"
	"polis/internal/core"
	"polis/internal/intake"
)

const maxSkillLoadTextBytes = 64 << 10
const maxSkillCatalogItems = 64
const maxSkillLoadHistoryItems = 32

type SkillLoadRequest struct {
	SkillID      string
	RelativePath string
}

type SkillLoadDocument struct {
	LoadReference   string `json:"loadReference"`
	SkillID         string `json:"skillId"`
	PackageID       string `json:"packageId"`
	Revision        string `json:"revision"`
	VersionDigest   string `json:"versionDigest"`
	RelativePath    string `json:"relativePath"`
	MediaType       string `json:"mediaType"`
	ContentDigest   string `json:"contentDigest"`
	ContentBoundary string `json:"contentBoundary"`
	Content         string `json:"content"`
}

type SkillLoadUse struct {
	LoadReference string `json:"loadReference"`
	EmployeeID    string `json:"employeeId"`
	TaskID        string `json:"taskId"`
	SessionID     string `json:"sessionId"`
	SkillID       string `json:"skillId"`
	PackageID     string `json:"packageId"`
	Revision      string `json:"revision"`
	VersionDigest string `json:"versionDigest"`
	RelativePath  string `json:"relativePath"`
	MediaType     string `json:"mediaType"`
	ContentDigest string `json:"contentDigest"`
	LoadedAt      string `json:"loadedAt,omitempty"`
}

func (k *Kernel) TXLoadBoundReadOnlySkill(ctx context.Context, binding Binding, key string, request SkillLoadRequest) (Receipt, SkillLoadDocument, error) {
	if !core.ValidID(request.SkillID) || !validSkillLoadPath(request.RelativePath) || len(key) == 0 || len(key) > 256 {
		return Receipt{}, SkillLoadDocument{}, core.Malformed
	}
	input := struct {
		SkillID      string
		RelativePath string
	}{request.SkillID, request.RelativePath}
	var document SkillLoadDocument
	receipt, err := k.TXWrite(ctx, binding.scope, &binding, key, "capability.skill.load", input, func(tx pgx.Tx) (Receipt, error) {
		var loadErr error
		document, loadErr = k.readAuthorizedSkillText(ctx, tx, binding, request)
		if loadErr != nil {
			return Receipt{}, loadErr
		}
		document.LoadReference = stableCapabilityID("skill-load", binding.session, document.SkillID, document.VersionDigest, document.RelativePath)
		var alreadyLoaded bool
		if loadErr = tx.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM events WHERE company_id=$1 AND kind='capability.skill.loaded'
AND payload->>'sessionId'=$2 AND payload->>'loadReference'=$3
)`, binding.scope.company, binding.session, document.LoadReference).Scan(&alreadyLoaded); loadErr != nil {
			return Receipt{}, loadErr
		}
		if alreadyLoaded {
			return Receipt{ID: document.LoadReference, Status: "already_loaded"}, nil
		}
		use := SkillLoadUse{
			LoadReference: document.LoadReference, EmployeeID: binding.employee, TaskID: binding.task, SessionID: binding.session,
			SkillID: document.SkillID, PackageID: document.PackageID, Revision: document.Revision,
			VersionDigest: document.VersionDigest, RelativePath: document.RelativePath,
			MediaType: document.MediaType, ContentDigest: document.ContentDigest,
		}
		if loadErr = appendEvent(ctx, tx, binding.scope, "capability.skill.loaded", use); loadErr != nil {
			return Receipt{}, loadErr
		}
		return Receipt{ID: document.LoadReference, Status: "loaded"}, nil
	})
	if err != nil {
		return Receipt{}, SkillLoadDocument{}, err
	}
	if document.LoadReference == "" {
		// TXWrite replay deliberately skips its callback. Recheck current grants
		// before returning text from a prior identical call after a revocation.
		tx, beginErr := k.pool.BeginTx(ctx, pgx.TxOptions{})
		if beginErr != nil {
			return Receipt{}, SkillLoadDocument{}, beginErr
		}
		defer tx.Rollback(ctx)
		// Hold the same company serialization lock used by all grant/revoke
		// commands through both authorization recheck and CAS content read.
		if beginErr = k.guard(ctx, tx, binding.scope, &binding); beginErr != nil {
			return Receipt{}, SkillLoadDocument{}, beginErr
		}
		document, beginErr = k.readAuthorizedSkillText(ctx, tx, binding, request)
		if beginErr != nil {
			return Receipt{}, SkillLoadDocument{}, beginErr
		}
		if beginErr = tx.Commit(ctx); beginErr != nil {
			return Receipt{}, SkillLoadDocument{}, beginErr
		}
		document.LoadReference = receipt.ID
	}
	return receipt, document, nil
}

func (k *Kernel) readAuthorizedSkillText(ctx context.Context, tx pgx.Tx, binding Binding, request SkillLoadRequest) (SkillLoadDocument, error) {
	var document SkillLoadDocument
	var sourceRef, contentDigest, status string
	var manifestJSON []byte
	if err := tx.QueryRow(ctx, `SELECT package_id,revision,source_ref,content_digest,manifest,status
FROM skill_revisions WHERE company_id=$1 AND id=$2`, binding.scope.company, request.SkillID).Scan(
		&document.PackageID, &document.Revision, &sourceRef, &contentDigest, &manifestJSON, &status,
	); errors.Is(err, pgx.ErrNoRows) {
		return SkillLoadDocument{}, core.OutOfScope
	} else if err != nil {
		return SkillLoadDocument{}, err
	}
	if status != "approved" || sourceRef != readOnlySkillSourceRef || !validCapabilityDigest(contentDigest) || len(manifestJSON) == 0 || len(manifestJSON) > 1<<20 {
		return SkillLoadDocument{}, core.Denied
	}
	var qualificationID, versionDigest, qualificationStatus string
	if err := tx.QueryRow(ctx, `SELECT event,version_digest,qualification_id
FROM employee_capability_events
WHERE company_id=$1 AND employee_id=$2 AND capability_kind='skill' AND capability_id=$3
ORDER BY event_seq DESC LIMIT 1`, binding.scope.company, binding.employee, request.SkillID).Scan(&status, &versionDigest, &qualificationID); errors.Is(err, pgx.ErrNoRows) {
		return SkillLoadDocument{}, core.Denied
	} else if err != nil {
		return SkillLoadDocument{}, err
	}
	if status != "bound" || versionDigest != contentDigest {
		return SkillLoadDocument{}, core.Denied
	}
	if err := tx.QueryRow(ctx, `SELECT status FROM capability_qualification_records
WHERE company_id=$1 AND qualification_id=$2 AND capability_kind='skill' AND capability_id=$3 AND version_digest=$4`, binding.scope.company, qualificationID, request.SkillID, contentDigest).Scan(&qualificationStatus); errors.Is(err, pgx.ErrNoRows) {
		return SkillLoadDocument{}, core.Denied
	} else if err != nil {
		return SkillLoadDocument{}, err
	}
	if qualificationStatus != "metadata_verified" {
		return SkillLoadDocument{}, core.Denied
	}
	var decision string
	if err := tx.QueryRow(ctx, `SELECT decision FROM capability_decisions
WHERE company_id=$1 AND capability_kind='skill' AND capability_id=$2 AND version_digest=$3 AND qualification_id=$4
ORDER BY created_at DESC,decision_id DESC LIMIT 1`, binding.scope.company, request.SkillID, contentDigest, qualificationID).Scan(&decision); errors.Is(err, pgx.ErrNoRows) {
		return SkillLoadDocument{}, core.Denied
	} else if err != nil {
		return SkillLoadDocument{}, err
	}
	if decision != "approved" {
		return SkillLoadDocument{}, core.Denied
	}
	var manifest capabilitysource.ReadOnlySkillManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil || len(manifest.Files) == 0 || len(manifest.Files) > intake.MaxDirectoryFiles {
		return SkillLoadDocument{}, core.Integrity
	}
	files := make([]capabilitysource.SkillBundleFile, 0, len(manifest.Files))
	var totalBytes int64
	var selected *capabilitysource.SkillBundleFile
	for _, entry := range manifest.Files {
		if !validCapabilityDigest(entry.ContentSHA256) || !validSkillLoadPath(entry.RelativePath) || entry.ByteSize <= 0 || entry.ByteSize > intake.MaxDirectoryBytes || totalBytes > intake.MaxDirectoryBytes-entry.ByteSize {
			return SkillLoadDocument{}, core.Integrity
		}
		totalBytes += entry.ByteSize
		content, err := readBlob(k.root, binding.scope.company, entry.ContentSHA256)
		if err != nil || int64(len(content)) != entry.ByteSize {
			return SkillLoadDocument{}, core.Integrity
		}
		file := capabilitysource.SkillBundleFile{
			RelativePath: entry.RelativePath, MediaType: entry.MediaType, ByteSize: entry.ByteSize,
			ContentSHA256: entry.ContentSHA256, Content: content,
		}
		files = append(files, file)
		if entry.RelativePath == request.RelativePath {
			copyOfFile := file
			selected = &copyOfFile
		}
	}
	if err := capabilitysource.VerifyReadOnlySkillBundle(manifest, contentDigest, files); err != nil {
		return SkillLoadDocument{}, core.Integrity
	}
	if selected == nil {
		return SkillLoadDocument{}, core.OutOfScope
	}
	if selected.MediaType != "text/markdown" && selected.MediaType != "text/plain" {
		return SkillLoadDocument{}, core.Denied
	}
	if len(selected.Content) > maxSkillLoadTextBytes {
		return SkillLoadDocument{}, core.TooLarge
	}
	return SkillLoadDocument{
		SkillID: request.SkillID, PackageID: document.PackageID, Revision: document.Revision,
		VersionDigest: contentDigest, RelativePath: selected.RelativePath, MediaType: selected.MediaType,
		ContentDigest: selected.ContentSHA256, ContentBoundary: "approved_static_text_no_additional_permissions",
		Content: string(selected.Content),
	}, nil
}

func validSkillLoadPath(value string) bool {
	if value == "" || len(value) > 1024 || strings.Contains(value, `\`) || path.IsAbs(value) || path.Clean(value) != value {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func listBoundReadOnlySkills(ctx context.Context, tx pgx.Tx, companyID, employeeID string) ([]BoundSkillSummary, bool, error) {
	rows, err := tx.Query(ctx, `SELECT s.id,s.package_id,s.revision,s.display_name,s.content_digest,s.manifest,q.profile
FROM skill_revisions s
JOIN LATERAL (
 SELECT event,version_digest,qualification_id FROM employee_capability_events e
 WHERE e.company_id=s.company_id AND e.employee_id=$2 AND e.capability_kind='skill' AND e.capability_id=s.id
 ORDER BY e.event_seq DESC LIMIT 1
) bound ON bound.event='bound' AND bound.version_digest=s.content_digest
JOIN capability_qualification_records q ON q.company_id=s.company_id AND q.qualification_id=bound.qualification_id
 AND q.capability_kind='skill' AND q.capability_id=s.id AND q.version_digest=s.content_digest AND q.status='metadata_verified'
WHERE s.company_id=$1 AND s.status='approved' AND s.source_ref=$3
 AND (SELECT d.decision FROM capability_decisions d WHERE d.company_id=s.company_id AND d.capability_kind='skill'
      AND d.capability_id=s.id AND d.version_digest=s.content_digest AND d.qualification_id=bound.qualification_id
      ORDER BY d.created_at DESC,d.decision_id DESC LIMIT 1)='approved'
ORDER BY s.package_id,s.revision,s.id LIMIT $4`, companyID, employeeID, readOnlySkillSourceRef, maxSkillCatalogItems+1)
	if err != nil {
		return nil, false, err
	}
	items := make([]BoundSkillSummary, 0, maxSkillCatalogItems+1)
	for rows.Next() {
		var item BoundSkillSummary
		var manifestJSON []byte
		if err = rows.Scan(&item.SkillID, &item.PackageID, &item.Revision, &item.DisplayName, &item.VersionDigest, &manifestJSON, &item.Profile); err != nil {
			rows.Close()
			return nil, false, err
		}
		var manifest capabilitysource.ReadOnlySkillManifest
		if len(manifestJSON) == 0 || len(manifestJSON) > 1<<20 || json.Unmarshal(manifestJSON, &manifest) != nil || manifest.SchemaVersion != capabilitysource.ReadOnlySkillBundleSchema || !manifest.ReadOnly || manifest.Name != item.PackageID {
			rows.Close()
			return nil, false, core.Integrity
		}
		item.Description = manifest.Description
		item.References = make([]SkillReferenceSummary, 0, len(manifest.Files))
		for _, file := range manifest.Files {
			if file.RelativePath == "SKILL.md" {
				item.References = append(item.References, SkillReferenceSummary{
					RelativePath: file.RelativePath, MediaType: file.MediaType, ByteSize: file.ByteSize,
					ContentDigest: file.ContentSHA256, Loadable: file.MediaType == "text/markdown" && file.ByteSize <= maxSkillLoadTextBytes,
				})
				continue
			}
			item.References = append(item.References, SkillReferenceSummary{
				RelativePath: file.RelativePath, MediaType: file.MediaType, ByteSize: file.ByteSize,
				ContentDigest: file.ContentSHA256,
				Loadable:      (file.MediaType == "text/markdown" || file.MediaType == "text/plain") && file.ByteSize <= maxSkillLoadTextBytes,
			})
		}
		items = append(items, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(items) > maxSkillCatalogItems
	if truncated {
		items = items[:maxSkillCatalogItems]
	}
	return items, truncated, nil
}

func listTaskSkillLoads(ctx context.Context, tx pgx.Tx, companyID, taskID string) ([]SkillLoadUse, bool, error) {
	rows, err := tx.Query(ctx, `SELECT payload->>'loadReference',payload->>'employeeId',payload->>'taskId',payload->>'sessionId',payload->>'skillId',
payload->>'packageId',payload->>'revision',payload->>'versionDigest',payload->>'relativePath',payload->>'mediaType',payload->>'contentDigest',payload->>'occurred_at'
FROM events WHERE company_id=$1 AND kind='capability.skill.loaded' AND payload->>'taskId'=$2
ORDER BY company_seq DESC LIMIT $3`, companyID, taskID, maxSkillLoadHistoryItems+1)
	if err != nil {
		return nil, false, err
	}
	items := make([]SkillLoadUse, 0, maxSkillLoadHistoryItems+1)
	for rows.Next() {
		var item SkillLoadUse
		if err = rows.Scan(&item.LoadReference, &item.EmployeeID, &item.TaskID, &item.SessionID, &item.SkillID, &item.PackageID, &item.Revision, &item.VersionDigest, &item.RelativePath, &item.MediaType, &item.ContentDigest, &item.LoadedAt); err != nil {
			rows.Close()
			return nil, false, err
		}
		items = append(items, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(items) > maxSkillLoadHistoryItems
	if truncated {
		items = items[:maxSkillLoadHistoryItems]
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return items, truncated, nil
}
