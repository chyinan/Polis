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
	Qualification    json.RawMessage `json:"qualification"`
	CreatedAt        string          `json:"createdAt"`
}

type EmployeeCapabilityBinding struct {
	CompanyID        string          `json:"companyId"`
	ID               string          `json:"id"`
	EmployeeID       string          `json:"employeeId"`
	Status           string          `json:"status"`
	SkillRevisionIDs []string        `json:"skillRevisionIds"`
	MCPDefinitionIDs []string        `json:"mcpDefinitionIds"`
	Qualification    json.RawMessage `json:"qualification"`
	CreatedAt        string          `json:"createdAt"`
}

type SkillLoad struct {
	CompanyID       string   `json:"companyId"`
	ID              string   `json:"id"`
	TaskID          string   `json:"taskId"`
	EmployeeID      string   `json:"employeeId"`
	SkillRevisionID string   `json:"skillRevisionId"`
	ContentDigest   string   `json:"contentDigest"`
	Body            string   `json:"body"`
	References      []string `json:"references"`
	ScriptsExecuted bool     `json:"scriptsExecuted"`
	Repeated        bool     `json:"repeated"`
	CreatedAt       string   `json:"createdAt"`
}

type CapabilityCatalog struct {
	Skills     []SkillRevision             `json:"skills"`
	MCPServers []MCPServerDefinition       `json:"mcpServers"`
	Bindings   []EmployeeCapabilityBinding `json:"bindings"`
	Loads      []SkillLoad                 `json:"loads"`
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
	catalog := CapabilityCatalog{Skills: []SkillRevision{}, MCPServers: []MCPServerDefinition{}, Bindings: []EmployeeCapabilityBinding{}, Loads: []SkillLoad{}}
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
	mcpRows, err := tx.Query(ctx, `SELECT id,name,transport,endpoint,command,args,descriptor_digest,status,qualification,created_at::text
FROM mcp_server_definitions WHERE company_id=$1 ORDER BY created_at DESC,id`, companyID)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	for mcpRows.Next() {
		var item MCPServerDefinition
		if err = mcpRows.Scan(&item.ID, &item.Name, &item.Transport, &item.Endpoint, &item.Command, &item.Args, &item.DescriptorDigest, &item.Status, &item.Qualification, &item.CreatedAt); err != nil {
			mcpRows.Close()
			return CapabilityCatalog{}, err
		}
		if len(item.Qualification) == 0 {
			item.Qualification = json.RawMessage(`{}`)
		}
		item.CompanyID = companyID
		catalog.MCPServers = append(catalog.MCPServers, item)
	}
	mcpRows.Close()
	if err = mcpRows.Err(); err != nil {
		return CapabilityCatalog{}, err
	}
	bindingRows, err := tx.Query(ctx, `SELECT id,employee_id,status,skill_revision_ids,mcp_definition_ids,qualification,created_at::text
FROM employee_capability_bindings WHERE company_id=$1 ORDER BY created_at DESC,id`, companyID)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	for bindingRows.Next() {
		var item EmployeeCapabilityBinding
		var skillIDs, mcpIDs []byte
		if err = bindingRows.Scan(&item.ID, &item.EmployeeID, &item.Status, &skillIDs, &mcpIDs, &item.Qualification, &item.CreatedAt); err != nil {
			bindingRows.Close()
			return CapabilityCatalog{}, err
		}
		if err = json.Unmarshal(skillIDs, &item.SkillRevisionIDs); err != nil {
			bindingRows.Close()
			return CapabilityCatalog{}, err
		}
		if err = json.Unmarshal(mcpIDs, &item.MCPDefinitionIDs); err != nil {
			bindingRows.Close()
			return CapabilityCatalog{}, err
		}
		if item.SkillRevisionIDs == nil {
			item.SkillRevisionIDs = []string{}
		}
		if item.MCPDefinitionIDs == nil {
			item.MCPDefinitionIDs = []string{}
		}
		item.CompanyID = companyID
		catalog.Bindings = append(catalog.Bindings, item)
	}
	bindingRows.Close()
	if err = bindingRows.Err(); err != nil {
		return CapabilityCatalog{}, err
	}
	loadRows, err := tx.Query(ctx, `SELECT l.id,l.task_id,l.employee_id,l.skill_revision_id,l.content_digest,s.manifest,l.created_at::text
FROM skill_loads l JOIN skill_revisions s ON s.company_id=l.company_id AND s.id=l.skill_revision_id
WHERE l.company_id=$1 ORDER BY l.created_at DESC,l.id`, companyID)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	for loadRows.Next() {
		var item SkillLoad
		var manifest []byte
		if err = loadRows.Scan(&item.ID, &item.TaskID, &item.EmployeeID, &item.SkillRevisionID, &item.ContentDigest, &manifest, &item.CreatedAt); err != nil {
			loadRows.Close()
			return CapabilityCatalog{}, err
		}
		view, viewErr := skillLoadView(manifest, item.ContentDigest)
		if viewErr != nil {
			loadRows.Close()
			return CapabilityCatalog{}, viewErr
		}
		item.CompanyID = companyID
		item.Body = view.Body
		item.References = view.References
		item.ScriptsExecuted = false
		catalog.Loads = append(catalog.Loads, item)
	}
	loadRows.Close()
	if err = loadRows.Err(); err != nil {
		return CapabilityCatalog{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CapabilityCatalog{}, err
	}
	return catalog, nil
}

func (k *Kernel) TXListVisibleSkills(ctx context.Context, companyID, employeeID, cursor string, limit int) (SkillSummaryPage, error) {
	if !core.ValidID(companyID) {
		return SkillSummaryPage{}, core.Malformed
	}
	if !validEmployeeID(employeeID) {
		return SkillSummaryPage{}, core.Denied
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return SkillSummaryPage{}, err
	}
	defer tx.Rollback(ctx)
	if err = ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
		return SkillSummaryPage{}, err
	}
	var bindingStatus string
	var boundRaw []byte
	err = tx.QueryRow(ctx, `SELECT status,skill_revision_ids FROM employee_capability_bindings WHERE company_id=$1 AND employee_id=$2 AND status='approved'`, companyID, employeeID).Scan(&bindingStatus, &boundRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		bindingStatus = ""
		boundRaw = []byte("[]")
		err = nil
	}
	if err != nil {
		return SkillSummaryPage{}, err
	}
	var bound []string
	if err = json.Unmarshal(boundRaw, &bound); err != nil {
		return SkillSummaryPage{}, core.Malformed
	}
	rows, err := tx.Query(ctx, `SELECT id,package_id,revision,display_name,source_ref,content_digest,manifest,status FROM skill_revisions WHERE company_id=$1`, companyID)
	if err != nil {
		return SkillSummaryPage{}, err
	}
	sources := []skillSummarySource{}
	for rows.Next() {
		var source skillSummarySource
		if err = rows.Scan(&source.ID, &source.PackageID, &source.Revision, &source.DisplayName, &source.SourceRef, &source.ContentDigest, &source.Manifest, &source.Status); err != nil {
			rows.Close()
			return SkillSummaryPage{}, err
		}
		sources = append(sources, source)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return SkillSummaryPage{}, err
	}
	page, err := visibleSkillPage(bindingStatus, bound, sources, cursor, limit)
	if err != nil {
		return SkillSummaryPage{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SkillSummaryPage{}, err
	}
	return page, nil
}

func (k *Kernel) TXListVisibleTools(ctx context.Context, companyID, employeeID, cursor string, limit int) (ToolSummaryPage, error) {
	if !core.ValidID(companyID) {
		return ToolSummaryPage{}, core.Malformed
	}
	if !validEmployeeID(employeeID) {
		return ToolSummaryPage{}, core.Denied
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ToolSummaryPage{}, err
	}
	defer tx.Rollback(ctx)
	if err = ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
		return ToolSummaryPage{}, err
	}
	var bindingStatus string
	var boundRaw []byte
	err = tx.QueryRow(ctx, `SELECT status,mcp_definition_ids FROM employee_capability_bindings WHERE company_id=$1 AND employee_id=$2 AND status='approved'`, companyID, employeeID).Scan(&bindingStatus, &boundRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		bindingStatus = ""
		boundRaw = []byte("[]")
		err = nil
	}
	if err != nil {
		return ToolSummaryPage{}, err
	}
	var bound []string
	if err = json.Unmarshal(boundRaw, &bound); err != nil {
		return ToolSummaryPage{}, core.Malformed
	}
	rows, err := tx.Query(ctx, `SELECT id,name,status,descriptor_digest,qualification FROM mcp_server_definitions WHERE company_id=$1`, companyID)
	if err != nil {
		return ToolSummaryPage{}, err
	}
	sources := []toolCatalogSource{}
	for rows.Next() {
		var source toolCatalogSource
		if err = rows.Scan(&source.ID, &source.Name, &source.Status, &source.DescriptorDigest, &source.Qualification); err != nil {
			rows.Close()
			return ToolSummaryPage{}, err
		}
		sources = append(sources, source)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return ToolSummaryPage{}, err
	}
	page, err := visibleToolPage(bindingStatus, bound, sources, cursor, limit)
	if err != nil {
		return ToolSummaryPage{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ToolSummaryPage{}, err
	}
	return page, nil
}

func (k *Kernel) TXDescribeVisibleTool(ctx context.Context, companyID, employeeID, handle string) (ToolDescriptorView, error) {
	if !core.ValidID(companyID) {
		return ToolDescriptorView{}, core.Malformed
	}
	if !validEmployeeID(employeeID) {
		return ToolDescriptorView{}, core.Denied
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ToolDescriptorView{}, err
	}
	defer tx.Rollback(ctx)
	if err = ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
		return ToolDescriptorView{}, err
	}
	var bindingStatus string
	var boundRaw []byte
	err = tx.QueryRow(ctx, `SELECT status,mcp_definition_ids FROM employee_capability_bindings WHERE company_id=$1 AND employee_id=$2 AND status='approved'`, companyID, employeeID).Scan(&bindingStatus, &boundRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		bindingStatus = ""
		boundRaw = []byte("[]")
		err = nil
	}
	if err != nil {
		return ToolDescriptorView{}, err
	}
	var bound []string
	if err = json.Unmarshal(boundRaw, &bound); err != nil {
		return ToolDescriptorView{}, core.Malformed
	}
	rows, err := tx.Query(ctx, `SELECT id,name,status,descriptor_digest,qualification FROM mcp_server_definitions WHERE company_id=$1`, companyID)
	if err != nil {
		return ToolDescriptorView{}, err
	}
	sources := []toolCatalogSource{}
	for rows.Next() {
		var source toolCatalogSource
		if err = rows.Scan(&source.ID, &source.Name, &source.Status, &source.DescriptorDigest, &source.Qualification); err != nil {
			rows.Close()
			return ToolDescriptorView{}, err
		}
		sources = append(sources, source)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return ToolDescriptorView{}, err
	}
	view, err := visibleToolDescriptor(bindingStatus, bound, sources, handle)
	if err != nil {
		return ToolDescriptorView{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ToolDescriptorView{}, err
	}
	return view, nil
}

func (k *Kernel) TXListVisibleResources(ctx context.Context, companyID, employeeID, serverID, cursor string, limit int) (ResourceSummaryPage, error) {
	if !core.ValidID(companyID) {
		return ResourceSummaryPage{}, core.Malformed
	}
	if !validEmployeeID(employeeID) {
		return ResourceSummaryPage{}, core.Denied
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ResourceSummaryPage{}, err
	}
	defer tx.Rollback(ctx)
	if err = ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
		return ResourceSummaryPage{}, err
	}
	var bindingStatus string
	var boundRaw []byte
	err = tx.QueryRow(ctx, `SELECT status,mcp_definition_ids FROM employee_capability_bindings WHERE company_id=$1 AND employee_id=$2 AND status='approved'`, companyID, employeeID).Scan(&bindingStatus, &boundRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		bindingStatus = ""
		boundRaw = []byte("[]")
		err = nil
	}
	if err != nil {
		return ResourceSummaryPage{}, err
	}
	var bound []string
	if err = json.Unmarshal(boundRaw, &bound); err != nil {
		return ResourceSummaryPage{}, core.Malformed
	}
	rows, err := tx.Query(ctx, `SELECT id,name,status,descriptor_digest,qualification FROM mcp_server_definitions WHERE company_id=$1`, companyID)
	if err != nil {
		return ResourceSummaryPage{}, err
	}
	sources := []toolCatalogSource{}
	for rows.Next() {
		var source toolCatalogSource
		if err = rows.Scan(&source.ID, &source.Name, &source.Status, &source.DescriptorDigest, &source.Qualification); err != nil {
			rows.Close()
			return ResourceSummaryPage{}, err
		}
		sources = append(sources, source)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return ResourceSummaryPage{}, err
	}
	page, err := visibleResourcePage(bindingStatus, bound, sources, serverID, cursor, limit)
	if err != nil {
		return ResourceSummaryPage{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ResourceSummaryPage{}, err
	}
	return page, nil
}

func (k *Kernel) TXImportSkillRevision(ctx context.Context, companyID string, input SkillRevisionInput) (SkillRevision, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.PackageID) || strings.TrimSpace(input.Revision) == "" || strings.TrimSpace(input.DisplayName) == "" || strings.TrimSpace(input.SourceRef) == "" || strings.TrimSpace(input.ContentDigest) == "" || (input.PublisherScope != "group" && input.PublisherScope != "company") {
		return SkillRevision{}, core.Malformed
	}
	if len(input.Manifest) == 0 {
		input.Manifest = json.RawMessage(`{}`)
	}
	if !json.Valid(input.Manifest) {
		return SkillRevision{}, core.Malformed
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = newID()
	}
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
	if !core.ValidID(companyID) || !core.ValidID(input.ID) || strings.TrimSpace(input.Name) == "" || (input.Transport != "stdio" && input.Transport != "streamable_http") || strings.TrimSpace(input.DescriptorDigest) == "" {
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

type SkillDocumentInput struct {
	ID             string
	PublisherScope string
	PackageID      string
	Revision       string
	SourceRef      string
	Document       []byte
	ScriptsPresent bool
	References     []SkillReferenceFile
}

type SkillReferenceRead struct {
	CompanyID       string `json:"companyId"`
	TaskID          string `json:"taskId"`
	EmployeeID      string `json:"employeeId"`
	SkillRevisionID string `json:"skillRevisionId"`
	Reference       string `json:"reference"`
	ContentDigest   string `json:"contentDigest"`
	Body            string `json:"body"`
	ScriptsExecuted bool   `json:"scriptsExecuted"`
}

type CapabilityBindingInput struct {
	EmployeeID       string
	SkillRevisionIDs []string
	MCPDefinitionIDs []string
	Status           string
}

func (k *Kernel) TXImportSkillDocument(ctx context.Context, companyID string, input SkillDocumentInput) (SkillRevision, error) {
	parsed, err := parseSkillDocument(input.Document, input.ScriptsPresent, input.References)
	if err != nil {
		return SkillRevision{}, err
	}
	packageID := strings.TrimSpace(input.PackageID)
	if packageID == "" {
		packageID = parsed.PackageID
	}
	if !core.ValidID(packageID) || !validRevisionLabel(input.Revision) || !validSourceRef(input.SourceRef) {
		return SkillRevision{}, core.Malformed
	}
	return k.TXImportSkillRevision(ctx, companyID, SkillRevisionInput{
		ID: input.ID, PublisherScope: input.PublisherScope, PackageID: packageID, Revision: strings.TrimSpace(input.Revision),
		DisplayName: parsed.DisplayName, SourceRef: strings.TrimSpace(input.SourceRef), ContentDigest: parsed.ContentDigest, Manifest: parsed.Manifest,
	})
}

func (k *Kernel) TXImportMCPDocument(ctx context.Context, companyID string, document []byte) (MCPServerDefinition, error) {
	parsed, err := ParseMCPDocument(document)
	if err != nil {
		return MCPServerDefinition{}, err
	}
	return k.TXRegisterMCPServerDefinition(ctx, companyID, MCPServerDefinitionInput{
		ID: parsed.ID, Name: parsed.Name, Transport: parsed.Transport, Endpoint: parsed.Endpoint, Command: parsed.Command, Args: parsed.Args, DescriptorDigest: parsed.DescriptorDigest,
	})
}

func (k *Kernel) QualifyRegisteredMCP(ctx context.Context, companyID, serverID string) (MCPServerDefinition, error) {
	if !core.ValidID(companyID) || !core.ValidID(serverID) {
		return MCPServerDefinition{}, core.Malformed
	}
	var transport, status, name string
	var endpoint, command *string
	var argsRaw []byte
	err := k.pool.QueryRow(ctx, `SELECT name,transport,endpoint,command,args,status FROM mcp_server_definitions WHERE company_id=$1 AND id=$2`, companyID, serverID).Scan(&name, &transport, &endpoint, &command, &argsRaw, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPServerDefinition{}, core.OutOfScope
	}
	if err != nil {
		return MCPServerDefinition{}, err
	}
	if status != "unverified" && status != "candidate" {
		return MCPServerDefinition{}, core.Denied
	}
	var (
		result        StdioProbeResult
		probeErr      error
		networkAccess bool
		connStatus    = "unprovisioned"
	)
	switch transport {
	case "stdio":
		if command == nil {
			return MCPServerDefinition{}, core.Denied
		}
		var args []string
		if err = json.Unmarshal(argsRaw, &args); err != nil {
			return MCPServerDefinition{}, core.Malformed
		}
		result, probeErr = QualifyStdioMCP(ctx, *command, args)
		networkAccess = false
	case "streamable_http":
		if endpoint == nil {
			return MCPServerDefinition{}, core.Denied
		}
		// Bounded qualification: local-only endpoint sandbox enforced by transport.
		result, probeErr = QualifyStreamableHTTPMCP(ctx, *endpoint)
		networkAccess = true // local loopback HTTP connection attempted (sandboxed)
		connStatus = "local_http"
	default:
		return MCPServerDefinition{}, core.Denied
	}
	if probeErr != nil {
		return MCPServerDefinition{}, probeErr
	}
	qualification, err := json.Marshal(map[string]any{
		"formatStatus": "checked", "executionStatus": "unverified", "connectionStatus": connStatus,
		"scriptsExecuted": false, "networkAccess": networkAccess, "networkRestricted": result.NetworkRestricted,
		"probeStatus": "passed", "protocolVersion": result.ProtocolVersion, "serverName": result.ServerName,
		"serverVersion": result.ServerVersion, "toolNames": result.ToolNames, "toolDescriptors": result.ToolDescriptors, "toolsCalled": result.ToolsCalled,
		"resourceSummaries": result.ResourceSummaries, "resourcesRead": 0, "transport": transport,
	})
	if err != nil {
		return MCPServerDefinition{}, err
	}
	var item MCPServerDefinition
	err = k.pool.QueryRow(ctx, `UPDATE mcp_server_definitions
SET status='candidate', descriptor_digest=$3, qualification=$4
WHERE company_id=$1 AND id=$2 AND status IN ('unverified','candidate')
RETURNING id,name,transport,endpoint,command,args,descriptor_digest,status,qualification,created_at::text`, companyID, serverID, result.DescriptorDigest, qualification).Scan(&item.ID, &item.Name, &item.Transport, &item.Endpoint, &item.Command, &item.Args, &item.DescriptorDigest, &item.Status, &item.Qualification, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPServerDefinition{}, core.Conflict
	}
	if err != nil {
		return MCPServerDefinition{}, err
	}
	item.CompanyID = companyID
	return item, nil
}

func (k *Kernel) TXLoadSkill(ctx context.Context, companyID, taskID, employeeID, skillID string) (SkillLoad, error) {
	if !core.ValidID(companyID) || !core.ValidID(taskID) || !core.ValidID(skillID) {
		return SkillLoad{}, core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return SkillLoad{}, err
	}
	defer tx.Rollback(ctx)
	if err = ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
		return SkillLoad{}, err
	}
	var taskOwner, taskState string
	err = tx.QueryRow(ctx, `SELECT owner,state FROM tasks WHERE company_id=$1 AND id=$2`, companyID, taskID).Scan(&taskOwner, &taskState)
	if errors.Is(err, pgx.ErrNoRows) {
		return SkillLoad{}, core.OutOfScope
	}
	if err != nil {
		return SkillLoad{}, err
	}
	var skillStatus, digest string
	var manifest []byte
	err = tx.QueryRow(ctx, `SELECT status,content_digest,manifest FROM skill_revisions WHERE company_id=$1 AND id=$2`, companyID, skillID).Scan(&skillStatus, &digest, &manifest)
	if errors.Is(err, pgx.ErrNoRows) {
		return SkillLoad{}, core.OutOfScope
	}
	if err != nil {
		return SkillLoad{}, err
	}
	var bindingStatus string
	var boundRaw []byte
	err = tx.QueryRow(ctx, `SELECT status,skill_revision_ids FROM employee_capability_bindings WHERE company_id=$1 AND employee_id=$2 AND status='approved'`, companyID, employeeID).Scan(&bindingStatus, &boundRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return SkillLoad{}, core.Denied
	}
	if err != nil {
		return SkillLoad{}, err
	}
	var bound []string
	if err = json.Unmarshal(boundRaw, &bound); err != nil {
		return SkillLoad{}, core.Malformed
	}
	if err = authorizeSkillLoad(taskOwner, taskState, employeeID, skillID, skillStatus, bindingStatus, bound); err != nil {
		return SkillLoad{}, err
	}
	view, err := skillLoadView(manifest, digest)
	if err != nil {
		return SkillLoad{}, err
	}
	loadID := newID()
	var item SkillLoad
	err = tx.QueryRow(ctx, `INSERT INTO skill_loads(company_id,id,task_id,employee_id,skill_revision_id,content_digest)
VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(company_id,task_id,employee_id,skill_revision_id) DO NOTHING
RETURNING id,created_at::text`, companyID, loadID, taskID, employeeID, skillID, digest).Scan(&item.ID, &item.CreatedAt)
	repeated := false
	if errors.Is(err, pgx.ErrNoRows) {
		repeated = true
		err = tx.QueryRow(ctx, `SELECT id,content_digest,created_at::text FROM skill_loads WHERE company_id=$1 AND task_id=$2 AND employee_id=$3 AND skill_revision_id=$4`, companyID, taskID, employeeID, skillID).Scan(&item.ID, &item.ContentDigest, &item.CreatedAt)
	}
	if err != nil {
		return SkillLoad{}, err
	}
	if !repeated {
		item.ContentDigest = digest
	}
	if item.ContentDigest != digest {
		return SkillLoad{}, core.Integrity
	}
	if err = tx.Commit(ctx); err != nil {
		return SkillLoad{}, err
	}
	item.CompanyID = companyID
	item.TaskID = taskID
	item.EmployeeID = employeeID
	item.SkillRevisionID = skillID
	item.Body = view.Body
	item.References = view.References
	item.ScriptsExecuted = false
	item.Repeated = repeated
	return item, nil
}

func (k *Kernel) TXReadSkillReference(ctx context.Context, companyID, taskID, employeeID, skillID, reference string) (SkillReferenceRead, error) {
	if !core.ValidID(companyID) || !core.ValidID(taskID) || !core.ValidID(skillID) || !safeSkillReference(reference) {
		return SkillReferenceRead{}, core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return SkillReferenceRead{}, err
	}
	defer tx.Rollback(ctx)
	if err = ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
		return SkillReferenceRead{}, err
	}
	var loadedDigest string
	err = tx.QueryRow(ctx, `SELECT content_digest FROM skill_loads WHERE company_id=$1 AND task_id=$2 AND employee_id=$3 AND skill_revision_id=$4`, companyID, taskID, employeeID, skillID).Scan(&loadedDigest)
	if errors.Is(err, pgx.ErrNoRows) {
		return SkillReferenceRead{}, core.Denied
	}
	if err != nil {
		return SkillReferenceRead{}, err
	}
	var digest string
	var manifest []byte
	err = tx.QueryRow(ctx, `SELECT content_digest,manifest FROM skill_revisions WHERE company_id=$1 AND id=$2`, companyID, skillID).Scan(&digest, &manifest)
	if errors.Is(err, pgx.ErrNoRows) {
		return SkillReferenceRead{}, core.OutOfScope
	}
	if err != nil {
		return SkillReferenceRead{}, err
	}
	if loadedDigest != digest {
		return SkillReferenceRead{}, core.Integrity
	}
	view, err := skillReferenceView(manifest, reference)
	if err != nil {
		return SkillReferenceRead{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SkillReferenceRead{}, err
	}
	return SkillReferenceRead{CompanyID: companyID, TaskID: taskID, EmployeeID: employeeID, SkillRevisionID: skillID, Reference: view.Reference, ContentDigest: view.ContentDigest, Body: view.Body, ScriptsExecuted: false}, nil
}

func (k *Kernel) TXSetSkillRevisionStatus(ctx context.Context, companyID, skillID, action string) (SkillRevision, error) {
	if !core.ValidID(companyID) || !core.ValidID(skillID) {
		return SkillRevision{}, core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return SkillRevision{}, err
	}
	defer tx.Rollback(ctx)
	if err = ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
		return SkillRevision{}, err
	}
	var current string
	err = tx.QueryRow(ctx, `SELECT status FROM skill_revisions WHERE company_id=$1 AND id=$2 FOR UPDATE`, companyID, skillID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return SkillRevision{}, core.OutOfScope
	}
	if err != nil {
		return SkillRevision{}, err
	}
	next, err := nextSkillCatalogStatus(current, action)
	if err != nil {
		return SkillRevision{}, err
	}
	var item SkillRevision
	err = tx.QueryRow(ctx, `UPDATE skill_revisions SET status=$3 WHERE company_id=$1 AND id=$2
RETURNING id,publisher_scope,package_id,revision,display_name,source_ref,content_digest,manifest,status,created_at::text`, companyID, skillID, next).Scan(&item.ID, &item.PublisherScope, &item.PackageID, &item.Revision, &item.DisplayName, &item.SourceRef, &item.ContentDigest, &item.Manifest, &item.Status, &item.CreatedAt)
	if err != nil {
		return SkillRevision{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SkillRevision{}, err
	}
	item.CompanyID = companyID
	return item, nil
}

func (k *Kernel) TXUpsertEmployeeCapabilityBinding(ctx context.Context, companyID string, input CapabilityBindingInput) (EmployeeCapabilityBinding, error) {
	if !core.ValidID(companyID) || duplicateIDs(input.SkillRevisionIDs) || duplicateIDs(input.MCPDefinitionIDs) {
		return EmployeeCapabilityBinding{}, core.Malformed
	}
	if input.SkillRevisionIDs == nil {
		input.SkillRevisionIDs = []string{}
	}
	if input.MCPDefinitionIDs == nil {
		input.MCPDefinitionIDs = []string{}
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return EmployeeCapabilityBinding{}, err
	}
	defer tx.Rollback(ctx)
	if err = ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
		return EmployeeCapabilityBinding{}, err
	}
	var employeeID string
	err = tx.QueryRow(ctx, `SELECT id FROM employees WHERE company_id=$1 AND id=$2`, companyID, input.EmployeeID).Scan(&employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return EmployeeCapabilityBinding{}, core.OutOfScope
	}
	if err != nil {
		return EmployeeCapabilityBinding{}, err
	}
	skills, err := loadCapabilityRefs(ctx, tx, `SELECT id,status FROM skill_revisions WHERE company_id=$1 AND id = ANY($2::text[])`, companyID, input.SkillRevisionIDs)
	if err != nil {
		return EmployeeCapabilityBinding{}, err
	}
	mcps, err := loadCapabilityRefs(ctx, tx, `SELECT id,status FROM mcp_server_definitions WHERE company_id=$1 AND id = ANY($2::text[])`, companyID, input.MCPDefinitionIDs)
	if err != nil {
		return EmployeeCapabilityBinding{}, err
	}
	if err = validateEmployeeCapabilityBinding(input.EmployeeID, input.Status, skills, mcps); err != nil {
		return EmployeeCapabilityBinding{}, err
	}
	skillIDs, err := json.Marshal(input.SkillRevisionIDs)
	if err != nil {
		return EmployeeCapabilityBinding{}, err
	}
	mcpIDs, err := json.Marshal(input.MCPDefinitionIDs)
	if err != nil {
		return EmployeeCapabilityBinding{}, err
	}
	qualification, err := json.Marshal(offlineQualification(false, "checked"))
	if err != nil {
		return EmployeeCapabilityBinding{}, err
	}
	var bindingID string
	err = tx.QueryRow(ctx, `SELECT id FROM employee_capability_bindings WHERE company_id=$1 AND employee_id=$2 AND status IN ('draft','approved','disabled') FOR UPDATE`, companyID, input.EmployeeID).Scan(&bindingID)
	if errors.Is(err, pgx.ErrNoRows) {
		bindingID = newID()
		err = nil
	}
	if err != nil {
		return EmployeeCapabilityBinding{}, err
	}
	var item EmployeeCapabilityBinding
	var storedSkills, storedMCPs []byte
	err = tx.QueryRow(ctx, `INSERT INTO employee_capability_bindings(company_id,id,employee_id,status,skill_revision_ids,mcp_definition_ids,qualification)
VALUES($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT(company_id,id) DO UPDATE SET status=EXCLUDED.status, skill_revision_ids=EXCLUDED.skill_revision_ids, mcp_definition_ids=EXCLUDED.mcp_definition_ids, qualification=EXCLUDED.qualification
RETURNING id,employee_id,status,skill_revision_ids,mcp_definition_ids,qualification,created_at::text`, companyID, bindingID, input.EmployeeID, input.Status, skillIDs, mcpIDs, qualification).Scan(&item.ID, &item.EmployeeID, &item.Status, &storedSkills, &storedMCPs, &item.Qualification, &item.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return EmployeeCapabilityBinding{}, core.Conflict
		}
		return EmployeeCapabilityBinding{}, err
	}
	if err = json.Unmarshal(storedSkills, &item.SkillRevisionIDs); err != nil || json.Unmarshal(storedMCPs, &item.MCPDefinitionIDs) != nil {
		return EmployeeCapabilityBinding{}, core.Malformed
	}
	if err = tx.Commit(ctx); err != nil {
		return EmployeeCapabilityBinding{}, err
	}
	item.CompanyID = companyID
	return item, nil
}

func loadCapabilityRefs(ctx context.Context, tx pgx.Tx, query, companyID string, ids []string) ([]CapabilityRef, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := tx.Query(ctx, query, companyID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	refs := make([]CapabilityRef, 0, len(ids))
	for rows.Next() {
		var ref CapabilityRef
		if err = rows.Scan(&ref.ID, &ref.Status); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(refs) != len(ids) {
		return nil, core.OutOfScope
	}
	return refs, nil
}

func duplicateIDs(ids []string) bool {
	seen := map[string]struct{}{}
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
}
