// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

func (k *Kernel) TXImportSkillRevisionCommand(ctx context.Context, companyID string, input SkillRevisionInput, requestID string) (SkillRevision, error) {
	if !core.ValidID(companyID) || !core.ValidID(requestID) || !core.ValidID(input.PackageID) || strings.TrimSpace(input.Revision) == "" || strings.TrimSpace(input.DisplayName) == "" || strings.TrimSpace(input.SourceRef) == "" || !validCapabilityDigest(strings.TrimSpace(input.ContentDigest)) || (input.PublisherScope != "group" && input.PublisherScope != "company") {
		return SkillRevision{}, core.Malformed
	}
	if len(input.Manifest) > 16<<10 || len(input.Manifest) != 0 && !json.Valid(input.Manifest) {
		return SkillRevision{}, core.Malformed
	}
	input.Manifest = metadataOnlySkillCandidateManifest(input.Manifest)
	if strings.TrimSpace(input.ID) == "" {
		input.ID = stableCapabilityID("skill", companyID, requestID)
	}
	if !core.ValidID(input.ID) {
		return SkillRevision{}, core.Malformed
	}
	unlock, err := k.lockSkillPackageRevision(ctx, companyID, input.PublisherScope, input.PackageID, strings.TrimSpace(input.Revision))
	if err != nil {
		return SkillRevision{}, err
	}
	defer unlock()
	scope := k.LocalScope(companyID)
	_, err = k.TXWrite(ctx, scope, nil, requestID, "capability.skill.import", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO skill_revisions(company_id,id,publisher_scope,package_id,revision,display_name,source_ref,content_digest,manifest,status)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'candidate')`, companyID, input.ID, input.PublisherScope, input.PackageID, strings.TrimSpace(input.Revision), strings.TrimSpace(input.DisplayName), strings.TrimSpace(input.SourceRef), strings.TrimSpace(input.ContentDigest), []byte(input.Manifest)); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: input.ID, Status: "candidate"}, nil
	})
	if err != nil {
		return SkillRevision{}, err
	}
	return k.getSkillRevision(ctx, companyID, input.ID)
}

func metadataOnlySkillCandidateManifest(clientManifest json.RawMessage) json.RawMessage {
	declaredReadOnly := false
	if len(clientManifest) != 0 {
		var metadata struct {
			ReadOnly bool `json:"readOnly"`
		}
		_ = json.Unmarshal(clientManifest, &metadata)
		declaredReadOnly = metadata.ReadOnly
	}
	manifest, _ := json.Marshal(struct {
		SchemaVersion     string `json:"schemaVersion"`
		ReadOnlyDeclared  bool   `json:"readOnlyDeclared"`
		SourceBytesStored bool   `json:"sourceBytesStored"`
	}{"polis-skill-metadata-candidate@1", declaredReadOnly, false})
	return manifest
}

func (k *Kernel) TXRegisterMCPServerDefinitionCommand(ctx context.Context, companyID string, input MCPServerDefinitionInput, requestID string) (MCPServerDefinition, error) {
	if !core.ValidID(companyID) || !core.ValidID(requestID) || strings.TrimSpace(input.Name) == "" || (input.Transport != "stdio" && input.Transport != "streamable_http") || !validCapabilityDigest(strings.TrimSpace(input.DescriptorDigest)) {
		return MCPServerDefinition{}, core.Malformed
	}
	if input.Transport == "stdio" && strings.TrimSpace(valueOrEmpty(input.Command)) == "" {
		return MCPServerDefinition{}, core.Malformed
	}
	if input.Transport == "streamable_http" && strings.TrimSpace(valueOrEmpty(input.Endpoint)) == "" {
		return MCPServerDefinition{}, core.Malformed
	}
	if strings.TrimSpace(input.ID) == "" {
		input.ID = stableCapabilityID("mcp", companyID, requestID)
	}
	if !core.ValidID(input.ID) {
		return MCPServerDefinition{}, core.Malformed
	}
	args, err := json.Marshal(input.Args)
	if err != nil {
		return MCPServerDefinition{}, core.Malformed
	}
	if input.Transport == "stdio" {
		canonicalDigest, digestErr := canonicalMCPDescriptorDigest(strings.TrimSpace(input.Name), input.Transport, valueOrEmpty(input.Command), args)
		if digestErr != nil || canonicalDigest != strings.TrimSpace(input.DescriptorDigest) {
			return MCPServerDefinition{}, core.Malformed
		}
	} else {
		if input.Endpoint == nil || input.Command != nil || len(input.Args) != 0 {
			return MCPServerDefinition{}, core.Malformed
		}
		canonicalEndpoint, canonicalDigest, digestErr := canonicalMCPStreamableHTTPDescriptorDigest(strings.TrimSpace(input.Name), *input.Endpoint)
		if digestErr != nil || canonicalDigest != strings.TrimSpace(input.DescriptorDigest) {
			return MCPServerDefinition{}, core.Malformed
		}
		input.Endpoint = &canonicalEndpoint
	}
	scope := k.LocalScope(companyID)
	_, err = k.TXWrite(ctx, scope, nil, requestID, "capability.mcp.register", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO mcp_server_definitions(company_id,id,name,transport,endpoint,command,args,descriptor_digest,status)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,'unverified')`, companyID, input.ID, strings.TrimSpace(input.Name), input.Transport, input.Endpoint, input.Command, args, strings.TrimSpace(input.DescriptorDigest)); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: input.ID, Status: "unverified"}, nil
	})
	if err != nil {
		return MCPServerDefinition{}, err
	}
	return k.getMCPServerDefinition(ctx, companyID, input.ID)
}

func (k *Kernel) getSkillRevision(ctx context.Context, companyID, skillID string) (SkillRevision, error) {
	var item SkillRevision
	err := k.pool.QueryRow(ctx, `SELECT id,publisher_scope,package_id,revision,display_name,source_ref,content_digest,manifest,status,created_at::text
FROM skill_revisions WHERE company_id=$1 AND id=$2`, companyID, skillID).Scan(&item.ID, &item.PublisherScope, &item.PackageID, &item.Revision, &item.DisplayName, &item.SourceRef, &item.ContentDigest, &item.Manifest, &item.Status, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SkillRevision{}, core.Integrity
	}
	if err != nil {
		return SkillRevision{}, err
	}
	item.CompanyID = companyID
	return item, nil
}

func (k *Kernel) getMCPServerDefinition(ctx context.Context, companyID, mcpID string) (MCPServerDefinition, error) {
	var item MCPServerDefinition
	err := k.pool.QueryRow(ctx, `SELECT id,name,transport,endpoint,command,args,descriptor_digest,status,created_at::text
FROM mcp_server_definitions WHERE company_id=$1 AND id=$2`, companyID, mcpID).Scan(&item.ID, &item.Name, &item.Transport, &item.Endpoint, &item.Command, &item.Args, &item.DescriptorDigest, &item.Status, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPServerDefinition{}, core.Integrity
	}
	if err != nil {
		return MCPServerDefinition{}, err
	}
	item.CompanyID = companyID
	return item, nil
}
