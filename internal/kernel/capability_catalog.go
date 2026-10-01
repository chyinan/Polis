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

type SkillRevision struct {
	CompanyID      string          `json:"companyId"`
	ID             string          `json:"id"`
	PublisherScope string          `json:"publisherScope"`
	PackageID      string          `json:"packageId"`
	Revision       string          `json:"revision"`
	DisplayName    string          `json:"displayName"`
	SourceRef      string          `json:"sourceRef"`
	ContentDigest  string          `json:"contentDigest"`
	Manifest       json.RawMessage `json:"manifest"`
	Status         string          `json:"status"`
	CreatedAt      string          `json:"createdAt"`
}

type MCPServerDefinition struct {
	CompanyID        string          `json:"companyId"`
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Transport        string          `json:"transport"`
	Endpoint         *string         `json:"endpoint"`
	Command          *string         `json:"command"`
	Args             json.RawMessage `json:"args"`
	DescriptorDigest string          `json:"descriptorDigest"`
	Status           string          `json:"status"`
	CreatedAt        string          `json:"createdAt"`
}

type CapabilityCatalog struct {
	Skills                []SkillRevision                `json:"skills"`
	MCPServers            []MCPServerDefinition          `json:"mcpServers"`
	MCPPackages           []StdioMCPPackageRevision      `json:"mcpPackages"`
	Qualifications        []CapabilityQualification      `json:"qualifications"`
	Bindings              []EmployeeCapabilityBinding    `json:"bindings"`
	Decisions             []CapabilityDecisionRecord     `json:"decisions"`
	RuntimeQualifications []StdioMCPRuntimeQualification `json:"runtimeQualifications"`
	Revocations           []CapabilityRevocationStatus   `json:"revocations"`
	RevocationsTruncated  bool                           `json:"revocationsTruncated"`
}

type SkillRevisionInput struct {
	ID             string
	PublisherScope string
	PackageID      string
	Revision       string
	DisplayName    string
	SourceRef      string
	ContentDigest  string
	Manifest       json.RawMessage
}

type MCPServerDefinitionInput struct {
	ID               string
	Name             string
	Transport        string
	Endpoint         *string
	Command          *string
	Args             []string
	DescriptorDigest string
}

func (k *Kernel) ListCapabilityCatalog(ctx context.Context, companyID string) (CapabilityCatalog, error) {
	if !core.ValidID(companyID) {
		return CapabilityCatalog{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return CapabilityCatalog{}, err
	}
	defer tx.Rollback(ctx)
	catalog := CapabilityCatalog{Skills: []SkillRevision{}, MCPServers: []MCPServerDefinition{}, MCPPackages: []StdioMCPPackageRevision{}, Qualifications: []CapabilityQualification{}, Bindings: []EmployeeCapabilityBinding{}, Decisions: []CapabilityDecisionRecord{}, RuntimeQualifications: []StdioMCPRuntimeQualification{}, Revocations: []CapabilityRevocationStatus{}}
	skillRows, err := tx.Query(ctx, `SELECT id,publisher_scope,package_id,revision,display_name,source_ref,content_digest,manifest,status,created_at::text
FROM skill_revisions WHERE company_id=$1 ORDER BY created_at DESC,id`, companyID)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	for skillRows.Next() {
		var item SkillRevision
		if err = skillRows.Scan(&item.ID, &item.PublisherScope, &item.PackageID, &item.Revision, &item.DisplayName, &item.SourceRef, &item.ContentDigest, &item.Manifest, &item.Status, &item.CreatedAt); err != nil {
			skillRows.Close()
			return CapabilityCatalog{}, err
		}
		item.CompanyID = companyID
		catalog.Skills = append(catalog.Skills, item)
	}
	skillRows.Close()
	if err = skillRows.Err(); err != nil {
		return CapabilityCatalog{}, err
	}
	mcpRows, err := tx.Query(ctx, `SELECT id,name,transport,endpoint,command,args,descriptor_digest,status,created_at::text
FROM mcp_server_definitions WHERE company_id=$1 ORDER BY created_at DESC,id`, companyID)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	for mcpRows.Next() {
		var item MCPServerDefinition
		if err = mcpRows.Scan(&item.ID, &item.Name, &item.Transport, &item.Endpoint, &item.Command, &item.Args, &item.DescriptorDigest, &item.Status, &item.CreatedAt); err != nil {
			mcpRows.Close()
			return CapabilityCatalog{}, err
		}
		item.CompanyID = companyID
		catalog.MCPServers = append(catalog.MCPServers, item)
	}
	mcpRows.Close()
	if err = mcpRows.Err(); err != nil {
		return CapabilityCatalog{}, err
	}
	catalog.MCPPackages, err = k.listStdioMCPPackageRevisions(ctx, tx, companyID)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	catalog.Qualifications, catalog.Bindings, catalog.Decisions, err = listCapabilityGovernance(ctx, tx, companyID)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	catalog.RuntimeQualifications, err = listStdioMCPRuntimeQualifications(ctx, tx, companyID)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	catalog.Revocations, catalog.RevocationsTruncated, err = listCapabilityRevocationStatuses(ctx, tx, companyID)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CapabilityCatalog{}, err
	}
	return catalog, nil
}

func (k *Kernel) TXImportSkillRevision(ctx context.Context, companyID string, input SkillRevisionInput) (SkillRevision, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.PackageID) || strings.TrimSpace(input.Revision) == "" || strings.TrimSpace(input.DisplayName) == "" || strings.TrimSpace(input.SourceRef) == "" || !validCapabilityDigest(strings.TrimSpace(input.ContentDigest)) || (input.PublisherScope != "group" && input.PublisherScope != "company") {
		return SkillRevision{}, core.Malformed
	}
	if len(input.Manifest) > 16<<10 || len(input.Manifest) != 0 && !json.Valid(input.Manifest) {
		return SkillRevision{}, core.Malformed
	}
	input.Manifest = metadataOnlySkillCandidateManifest(input.Manifest)
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = newID()
	}
	unlock, err := k.lockSkillPackageRevision(ctx, companyID, input.PublisherScope, input.PackageID, strings.TrimSpace(input.Revision))
	if err != nil {
		return SkillRevision{}, err
	}
	defer unlock()
	var item SkillRevision
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return SkillRevision{}, err
	}
	defer tx.Rollback(ctx)
	if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
		return SkillRevision{}, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO skill_revisions(company_id,id,publisher_scope,package_id,revision,display_name,source_ref,content_digest,manifest,status)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'candidate')
RETURNING id,publisher_scope,package_id,revision,display_name,source_ref,content_digest,manifest,status,created_at::text`, companyID, id, input.PublisherScope, input.PackageID, strings.TrimSpace(input.Revision), strings.TrimSpace(input.DisplayName), strings.TrimSpace(input.SourceRef), strings.TrimSpace(input.ContentDigest), []byte(input.Manifest)).Scan(&item.ID, &item.PublisherScope, &item.PackageID, &item.Revision, &item.DisplayName, &item.SourceRef, &item.ContentDigest, &item.Manifest, &item.Status, &item.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return SkillRevision{}, core.Conflict
		}
		return SkillRevision{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SkillRevision{}, err
	}
	item.CompanyID = companyID
	return item, nil
}

func (k *Kernel) TXRegisterMCPServerDefinition(ctx context.Context, companyID string, input MCPServerDefinitionInput) (MCPServerDefinition, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.ID) || strings.TrimSpace(input.Name) == "" || (input.Transport != "stdio" && input.Transport != "streamable_http") || !validCapabilityDigest(strings.TrimSpace(input.DescriptorDigest)) {
		return MCPServerDefinition{}, core.Malformed
	}
	if input.Transport == "stdio" && strings.TrimSpace(valueOrEmpty(input.Command)) == "" {
		return MCPServerDefinition{}, core.Malformed
	}
	if input.Transport == "streamable_http" && strings.TrimSpace(valueOrEmpty(input.Endpoint)) == "" {
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
	var item MCPServerDefinition
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return MCPServerDefinition{}, err
	}
	defer tx.Rollback(ctx)
	if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
		return MCPServerDefinition{}, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO mcp_server_definitions(company_id,id,name,transport,endpoint,command,args,descriptor_digest,status)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,'unverified')
RETURNING id,name,transport,endpoint,command,args,descriptor_digest,status,created_at::text`, companyID, input.ID, strings.TrimSpace(input.Name), input.Transport, input.Endpoint, input.Command, args, strings.TrimSpace(input.DescriptorDigest)).Scan(&item.ID, &item.Name, &item.Transport, &item.Endpoint, &item.Command, &item.Args, &item.DescriptorDigest, &item.Status, &item.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return MCPServerDefinition{}, core.Conflict
		}
		return MCPServerDefinition{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MCPServerDefinition{}, err
	}
	item.CompanyID = companyID
	return item, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func ensureCapabilityCatalogCompanyWritable(ctx context.Context, tx pgx.Tx, companyID string) error {
	var state string
	if err := tx.QueryRow(ctx, "SELECT state FROM companies WHERE id=$1", companyID).Scan(&state); errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	} else if err != nil {
		return err
	}
	return validateCapabilityCatalogCompanyState(state)
}
