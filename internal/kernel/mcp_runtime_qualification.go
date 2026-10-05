// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/capabilitysource"
	"polis/internal/core"
	"polis/internal/mcpowner"
	"polis/internal/mcptransport"
)

const stdioMCPHostProfileWindowsAppContainer = "windows_appcontainer_deny_all@1"

type StdioMCPRuntimeObservationInput struct {
	CapabilityID              string
	CapabilityQualificationID string
	Observation               mcpowner.RuntimeObservation
	RequestID                 string
}

type StdioMCPRuntimeQualification struct {
	CompanyID                 string `json:"companyId"`
	RuntimeQualificationID    string `json:"runtimeQualificationId"`
	CapabilityID              string `json:"capabilityId"`
	CapabilityQualificationID string `json:"capabilityQualificationId"`
	Transport                 string `json:"transport"`
	Endpoint                  string `json:"endpoint,omitempty"`
	VersionDigest             string `json:"versionDigest"`
	DescriptorDigest          string `json:"descriptorDigest"`
	RuntimeProfile            string `json:"runtimeProfile"`
	HostOS                    string `json:"hostOs"`
	HostProfile               string `json:"hostProfile"`
	CommandSHA256             string `json:"commandSha256"`
	PackageManifestSHA256     string `json:"packageManifestSha256"`
	ServerName                string `json:"serverName"`
	ServerVersion             string `json:"serverVersion"`
	ProtocolVersion           string `json:"protocolVersion"`
	ToolSchemaSHA256          string `json:"toolSchemaSha256"`
	Status                    string `json:"status"`
	EvidenceDigest            string `json:"evidenceDigest"`
	CreatedAt                 string `json:"createdAt"`
}

type StdioMCPRuntimeDecisionInput struct {
	RuntimeQualificationID string
	Rationale              string
	RequestID              string
}

type StdioMCPToolSchemaDriftInput struct {
	RuntimeQualificationID   string
	ObservedToolSchemaSHA256 string
	RequestID                string
}

type StdioMCPToolAuthorization struct {
	RuntimeQualification StdioMCPRuntimeQualification
	ProcessSpec          mcpowner.ProcessSpec
	Tools                []mcptransport.StdioToolDefinition
	Transport            string
	Endpoint             string
	GrantRevision        int64
	TargetSHA256         string
}

func (k *Kernel) TXRecordStdioMCPRuntimeQualification(ctx context.Context, companyID string, input StdioMCPRuntimeObservationInput) (StdioMCPRuntimeQualification, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.CapabilityID) || !core.ValidID(input.CapabilityQualificationID) || !core.ValidID(input.RequestID) {
		return StdioMCPRuntimeQualification{}, core.Malformed
	}
	if !input.Observation.Valid() {
		return StdioMCPRuntimeQualification{}, core.Denied
	}
	processSpec := input.Observation.ProcessSpec()
	tools := input.Observation.Tools()
	hostOS, hostProfile := input.Observation.Host()
	serverIdentity := input.Observation.ServerIdentity()
	toolSchemaDigest := input.Observation.ToolSchemaSHA256()
	if hostOS != "windows" || hostProfile != stdioMCPHostProfileWindowsAppContainer {
		return StdioMCPRuntimeQualification{}, core.Denied
	}
	if err := mcpowner.ValidateProcessSpec(processSpec); err != nil {
		return StdioMCPRuntimeQualification{}, core.Malformed
	}
	if processSpec.ApprovedToolSchemaSHA256 == "" || len(tools) == 0 || len(tools) > mcptransport.MaxStdioToolCount {
		return StdioMCPRuntimeQualification{}, core.Malformed
	}
	toolsJSON, err := json.Marshal(tools)
	if err != nil || len(toolsJSON) > mcptransport.MaxStdioToolListBytes {
		return StdioMCPRuntimeQualification{}, core.Malformed
	}
	toolDigest, err := mcptransport.StdioToolSchemaDigest(toolsJSON)
	if err != nil || toolDigest != processSpec.ApprovedToolSchemaSHA256 || toolDigest != toolSchemaDigest {
		return StdioMCPRuntimeQualification{}, core.Denied
	}
	processSpecJSON, err := json.Marshal(processSpec)
	if err != nil || len(processSpecJSON) > 256<<10 {
		return StdioMCPRuntimeQualification{}, core.TooLarge
	}
	runtimeQualificationID := stableCapabilityID("mcp-runtime", companyID, input.RequestID)
	scope := k.LocalScope(companyID)
	writeInput := struct {
		CapabilityID              string
		CapabilityQualificationID string
		HostOS                    string
		HostProfile               string
		ProcessSpec               mcpowner.ProcessSpec
		Tools                     []mcptransport.StdioToolDefinition
		ServerIdentity            mcptransport.StdioServerIdentity
		ToolSchemaSHA256          string
	}{input.CapabilityID, input.CapabilityQualificationID, hostOS, hostProfile, processSpec, tools, serverIdentity, toolDigest}
	_, err = k.TXWrite(ctx, scope, nil, input.RequestID, "capability.mcp.runtime.observe", writeInput, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if processSpec.SourcePackageRevisionID != "" {
			if err := lockCapabilityMCPServerInTransaction(ctx, tx, companyID, input.CapabilityID); err != nil {
				return Receipt{}, err
			}
		}
		var name, transport, currentDescriptorDigest, capabilityStatus string
		var endpoint, command *string
		var descriptorArgs []byte
		if err := tx.QueryRow(ctx, `SELECT name,transport,endpoint,command,args,descriptor_digest,status
FROM mcp_server_definitions WHERE company_id=$1 AND id=$2 FOR SHARE`, companyID, input.CapabilityID).Scan(&name, &transport, &endpoint, &command, &descriptorArgs, &currentDescriptorDigest, &capabilityStatus); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if transport != "stdio" || endpoint != nil || command == nil || capabilityStatus != "approved" {
			return Receipt{}, core.Denied
		}
		var descriptorArgumentList []string
		if json.Unmarshal(descriptorArgs, &descriptorArgumentList) != nil {
			return Receipt{}, core.Integrity
		}
		computedDescriptorDigest, digestErr := canonicalMCPDescriptorDigest(name, transport, *command, descriptorArgs)
		if digestErr != nil || computedDescriptorDigest != currentDescriptorDigest || !stdioMCPLaunchMatchesDescriptor(hostOS, processSpec, *command, descriptorArgumentList) {
			return Receipt{}, core.Denied
		}
		var metadataVersion, metadataProfile, metadataStatus string
		if err := tx.QueryRow(ctx, `SELECT version_digest,profile,status FROM capability_qualification_records
WHERE company_id=$1 AND qualification_id=$2 AND capability_kind='mcp' AND capability_id=$3`, companyID, input.CapabilityQualificationID, input.CapabilityID).Scan(&metadataVersion, &metadataProfile, &metadataStatus); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if metadataVersion != currentDescriptorDigest || metadataProfile != "stdio_mcp@1" || metadataStatus != "metadata_verified" {
			return Receipt{}, core.Denied
		}
		var decision string
		if err := tx.QueryRow(ctx, `SELECT decision FROM capability_decisions
WHERE company_id=$1 AND capability_kind='mcp' AND capability_id=$2 AND version_digest=$3 AND qualification_id=$4
ORDER BY created_at DESC,decision_id DESC LIMIT 1`, companyID, input.CapabilityID, currentDescriptorDigest, input.CapabilityQualificationID).Scan(&decision); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Denied
		} else if err != nil {
			return Receipt{}, err
		}
		if decision != "approved" {
			return Receipt{}, core.Denied
		}
		if processSpec.SourcePackageRevisionID != "" {
			var sourceServerID, sourceManifestDigest string
			var sourceManifestJSON []byte
			if err := tx.QueryRow(ctx, `SELECT server_id,manifest_digest,manifest FROM mcp_server_package_revisions
WHERE company_id=$1 AND package_revision_id=$2 FOR SHARE`, companyID, processSpec.SourcePackageRevisionID).Scan(&sourceServerID, &sourceManifestDigest, &sourceManifestJSON); errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.Denied
			} else if err != nil {
				return Receipt{}, err
			}
			if sourceServerID != input.CapabilityID || sourceManifestDigest != processSpec.SourcePackageManifestSHA256 {
				return Receipt{}, core.Denied
			}
			var latestPackageID, latestManifestDigest string
			if err := tx.QueryRow(ctx, `SELECT package_revision_id,manifest_digest FROM mcp_server_package_revisions
WHERE company_id=$1 AND server_id=$2 ORDER BY created_at DESC,package_revision_id LIMIT 1 FOR SHARE`, companyID, sourceServerID).Scan(&latestPackageID, &latestManifestDigest); err != nil {
				return Receipt{}, err
			}
			if latestPackageID != processSpec.SourcePackageRevisionID || latestManifestDigest != sourceManifestDigest {
				return Receipt{}, core.Denied
			}
			var intentCapabilityID, intentQualificationID, intentPackageID, intentPackageDigest, intentStatus string
			if err := tx.QueryRow(ctx, `SELECT i.capability_id,i.capability_qualification_id,i.package_revision_id,i.package_manifest_sha256,e.status
FROM mcp_runtime_observation_intents i
JOIN LATERAL (SELECT status FROM mcp_runtime_observation_intent_events
 WHERE company_id=i.company_id AND observation_request_id=i.request_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE i.company_id=$1 AND i.request_id=$2 FOR SHARE OF i`, companyID, input.RequestID).Scan(
				&intentCapabilityID, &intentQualificationID, &intentPackageID, &intentPackageDigest, &intentStatus); errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.Denied
			} else if err != nil {
				return Receipt{}, err
			}
			if intentStatus != "reserved" || intentCapabilityID != input.CapabilityID || intentQualificationID != input.CapabilityQualificationID ||
				intentPackageID != processSpec.SourcePackageRevisionID || intentPackageDigest != processSpec.SourcePackageManifestSHA256 {
				return Receipt{}, core.Conflict
			}
			var sourceManifest capabilitysource.StdioMCPBundleManifestData
			if json.Unmarshal(sourceManifestJSON, &sourceManifest) != nil || capabilitysource.ValidateStdioMCPBundleManifest(sourceManifest, sourceManifestDigest) != nil ||
				!stdioMCPProcessSpecMatchesSourceManifest(hostOS, processSpec, sourceManifest, sourceManifestDigest) {
				return Receipt{}, core.Denied
			}
		}
		evidenceBytes, marshalErr := json.Marshal(struct {
			CompanyID                 string `json:"companyId"`
			RuntimeQualificationID    string `json:"runtimeQualificationId"`
			CapabilityID              string `json:"capabilityId"`
			CapabilityQualificationID string `json:"capabilityQualificationId"`
			VersionDigest             string `json:"versionDigest"`
			DescriptorDigest          string `json:"descriptorDigest"`
			RuntimeProfile            string `json:"runtimeProfile"`
			HostOS                    string `json:"hostOs"`
			HostProfile               string `json:"hostProfile"`
			CommandSHA256             string `json:"commandSha256"`
			PackageManifestSHA256     string `json:"packageManifestSha256"`
			ServerName                string `json:"serverName"`
			ServerVersion             string `json:"serverVersion"`
			ProtocolVersion           string `json:"protocolVersion"`
			ToolSchemaSHA256          string `json:"toolSchemaSha256"`
		}{
			CompanyID: companyID, RuntimeQualificationID: runtimeQualificationID, CapabilityID: input.CapabilityID,
			CapabilityQualificationID: input.CapabilityQualificationID, VersionDigest: currentDescriptorDigest,
			DescriptorDigest: currentDescriptorDigest, RuntimeProfile: mcptransport.StdioProfile20260728,
			HostOS: hostOS, HostProfile: hostProfile, CommandSHA256: processSpec.CommandSHA256,
			PackageManifestSHA256: processSpec.PackageManifestSHA256, ServerName: serverIdentity.Name,
			ServerVersion: serverIdentity.Version, ProtocolVersion: mcptransport.ProtocolVersion20260728,
			ToolSchemaSHA256: toolDigest,
		})
		if marshalErr != nil {
			return Receipt{}, core.Integrity
		}
		evidenceDigest := digestCapabilityBytes(evidenceBytes)
		_, err := tx.Exec(ctx, `INSERT INTO mcp_runtime_qualification_records(
company_id,runtime_qualification_id,capability_id,capability_qualification_id,version_digest,descriptor_digest,
runtime_profile,host_os,host_profile,command_sha256,package_manifest_sha256,server_name,server_version,
protocol_version,tool_schema_sha256,tools,process_spec,evidence_digest,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
			companyID, runtimeQualificationID, input.CapabilityID, input.CapabilityQualificationID, currentDescriptorDigest, currentDescriptorDigest,
			mcptransport.StdioProfile20260728, hostOS, hostProfile, processSpec.CommandSHA256,
			processSpec.PackageManifestSHA256, serverIdentity.Name, serverIdentity.Version,
			mcptransport.ProtocolVersion20260728, toolDigest, toolsJSON, processSpecJSON, evidenceDigest, input.RequestID)
		if err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		eventID := stableCapabilityID("mcp-runtime-event", companyID, input.RequestID)
		if _, err = tx.Exec(ctx, `INSERT INTO mcp_runtime_qualification_events(
company_id,event_id,runtime_qualification_id,capability_id,version_digest,status,tool_schema_sha256,rationale,request_id)
VALUES($1,$2,$3,$4,$5,'observed_unqualified',$6,'local runtime observation recorded',$7)`, companyID, eventID, runtimeQualificationID, input.CapabilityID, currentDescriptorDigest, toolDigest, input.RequestID); err != nil {
			return Receipt{}, err
		}
		if processSpec.SourcePackageRevisionID != "" {
			if err = appendMCPRuntimeObservationIntentEvent(ctx, tx, companyID, input.RequestID, processSpec.SourcePackageRevisionID,
				processSpec.SourcePackageManifestSHA256, "completed", runtimeQualificationID, "runtime discovery completed and process tree stopped"); err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: runtimeQualificationID, Status: "observed_unqualified"}, nil
	})
	if err != nil {
		return StdioMCPRuntimeQualification{}, err
	}
	return k.getStdioMCPRuntimeQualification(ctx, companyID, runtimeQualificationID)
}

func (k *Kernel) TXApproveStdioMCPRuntimeQualification(ctx context.Context, companyID string, input StdioMCPRuntimeDecisionInput) error {
	if !core.ValidID(companyID) || !core.ValidID(input.RuntimeQualificationID) || !core.ValidID(input.RequestID) || strings.TrimSpace(input.Rationale) == "" || len(input.Rationale) > 512 {
		return core.Malformed
	}
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "capability.mcp.runtime.approve", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var capabilityID, versionDigest, metadataQualificationID, currentStatus, transport, runtimeProfile string
		var descriptorDigest, schemaDigest string
		var endpoint *string
		var processSpecJSON []byte
		if err := tx.QueryRow(ctx, `SELECT r.capability_id,r.version_digest,r.descriptor_digest,r.capability_qualification_id,
e.status,e.tool_schema_sha256,r.process_spec,r.transport,r.endpoint,r.runtime_profile
FROM mcp_runtime_qualification_records r
JOIN LATERAL (SELECT status,tool_schema_sha256 FROM mcp_runtime_qualification_events
 WHERE company_id=r.company_id AND runtime_qualification_id=r.runtime_qualification_id
 ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE r.company_id=$1 AND r.runtime_qualification_id=$2 FOR SHARE OF r`, companyID, input.RuntimeQualificationID).Scan(&capabilityID, &versionDigest, &descriptorDigest, &metadataQualificationID, &currentStatus, &schemaDigest, &processSpecJSON, &transport, &endpoint, &runtimeProfile); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if currentStatus != "observed_unqualified" || descriptorDigest != versionDigest {
			return Receipt{}, core.Denied
		}
		if err := lockCapabilityMCPServerInTransaction(ctx, tx, companyID, capabilityID); err != nil {
			return Receipt{}, err
		}
		var processSpec mcpowner.ProcessSpec
		if transport == "stdio" {
			if runtimeProfile != mcptransport.StdioProfile20260728 || endpoint != nil || json.Unmarshal(processSpecJSON, &processSpec) != nil {
				return Receipt{}, core.Integrity
			}
			if processSpec.SourcePackageRevisionID == "" {
				// Legacy metadata-only stdio definitions remain locally qualified by
				// their fixed process specification.
			} else {
				var latestPackageID, latestManifestDigest string
				if err := tx.QueryRow(ctx, `SELECT package_revision_id,manifest_digest FROM mcp_server_package_revisions
WHERE company_id=$1 AND server_id=$2 ORDER BY created_at DESC,package_revision_id LIMIT 1 FOR SHARE`, companyID, capabilityID).Scan(&latestPackageID, &latestManifestDigest); err != nil {
					return Receipt{}, core.Denied
				}
				if latestPackageID != processSpec.SourcePackageRevisionID || latestManifestDigest != processSpec.SourcePackageManifestSHA256 {
					return Receipt{}, core.Denied
				}
			}
		} else if transport == "streamable_http" {
			if runtimeProfile != mcptransport.StreamableHTTPProfile20260728 || endpoint == nil || processSpecJSON == nil || string(processSpecJSON) != "{}" {
				return Receipt{}, core.Integrity
			}
			var currentEndpoint *string
			var currentName string
			if err := tx.QueryRow(ctx, `SELECT name,endpoint FROM mcp_server_definitions WHERE company_id=$1 AND id=$2 FOR SHARE`, companyID, capabilityID).Scan(&currentName, &currentEndpoint); err != nil {
				return Receipt{}, err
			}
			if currentEndpoint == nil || *currentEndpoint != *endpoint {
				return Receipt{}, core.Denied
			}
			canonicalEndpoint, digest, err := canonicalMCPStreamableHTTPDescriptorDigest(currentName, *currentEndpoint)
			if err != nil || canonicalEndpoint != *currentEndpoint || digest != versionDigest {
				return Receipt{}, core.Denied
			}
		} else {
			return Receipt{}, core.Denied
		}
		var currentDigest, capabilityStatus string
		if err := tx.QueryRow(ctx, `SELECT descriptor_digest,status FROM mcp_server_definitions WHERE company_id=$1 AND id=$2 FOR SHARE`, companyID, capabilityID).Scan(&currentDigest, &capabilityStatus); err != nil {
			return Receipt{}, err
		}
		if currentDigest != versionDigest || capabilityStatus != "approved" {
			return Receipt{}, core.Denied
		}
		var decision string
		if err := tx.QueryRow(ctx, `SELECT decision FROM capability_decisions
WHERE company_id=$1 AND capability_kind='mcp' AND capability_id=$2 AND version_digest=$3 AND qualification_id=$4
ORDER BY created_at DESC,decision_id DESC LIMIT 1`, companyID, capabilityID, versionDigest, metadataQualificationID).Scan(&decision); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Denied
		} else if err != nil {
			return Receipt{}, err
		}
		if decision != "approved" {
			return Receipt{}, core.Denied
		}
		eventID := stableCapabilityID("mcp-runtime-approval", companyID, input.RequestID)
		if _, err := tx.Exec(ctx, `INSERT INTO mcp_runtime_qualification_events(
company_id,event_id,runtime_qualification_id,capability_id,version_digest,status,tool_schema_sha256,rationale,request_id)
VALUES($1,$2,$3,$4,$5,'qualified',$6,$7,$8)`, companyID, eventID, input.RuntimeQualificationID, capabilityID, versionDigest, schemaDigest, strings.TrimSpace(input.Rationale), input.RequestID); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: input.RuntimeQualificationID, Status: "qualified"}, nil
	})
	return err
}

func (k *Kernel) TXRecordStdioMCPToolSchemaDrift(ctx context.Context, companyID string, input StdioMCPToolSchemaDriftInput) error {
	if !core.ValidID(companyID) || !core.ValidID(input.RuntimeQualificationID) || !validCapabilityDigest(input.ObservedToolSchemaSHA256) || !core.ValidID(input.RequestID) {
		return core.Malformed
	}
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "capability.mcp.runtime.schema_drift", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var capabilityID, versionDigest, storedSchema, status string
		if err := tx.QueryRow(ctx, `SELECT r.capability_id,r.version_digest,r.tool_schema_sha256,e.status
FROM mcp_runtime_qualification_records r
JOIN LATERAL (SELECT status FROM mcp_runtime_qualification_events
 WHERE company_id=r.company_id AND runtime_qualification_id=r.runtime_qualification_id
 ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE r.company_id=$1 AND r.runtime_qualification_id=$2 FOR SHARE OF r`, companyID, input.RuntimeQualificationID).Scan(&capabilityID, &versionDigest, &storedSchema, &status); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if input.ObservedToolSchemaSHA256 == storedSchema || (status != "qualified" && status != "observed_unqualified") {
			return Receipt{}, core.Conflict
		}
		eventID := stableCapabilityID("mcp-runtime-drift", companyID, input.RequestID)
		if _, err := tx.Exec(ctx, `INSERT INTO mcp_runtime_qualification_events(
company_id,event_id,runtime_qualification_id,capability_id,version_digest,status,tool_schema_sha256,observed_tool_schema_sha256,rationale,request_id)
VALUES($1,$2,$3,$4,$5,'schema_drift',$6,$7,'tool schema changed after qualification',$8)`, companyID, eventID, input.RuntimeQualificationID, capabilityID, versionDigest, storedSchema, input.ObservedToolSchemaSHA256, input.RequestID); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: input.RuntimeQualificationID, Status: "schema_drift"}, nil
	})
	return err
}

func revokeStdioMCPRuntimeQualifications(ctx context.Context, tx pgx.Tx, companyID, capabilityID, versionDigest, decisionID, rationale string) error {
	rows, err := tx.Query(ctx, `SELECT r.runtime_qualification_id,r.tool_schema_sha256,e.status
FROM mcp_runtime_qualification_records r
JOIN LATERAL (SELECT status FROM mcp_runtime_qualification_events
 WHERE company_id=r.company_id AND runtime_qualification_id=r.runtime_qualification_id
 ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE r.company_id=$1 AND r.capability_id=$2 AND r.version_digest=$3 AND e.status IN ('observed_unqualified','qualified')
ORDER BY r.runtime_qualification_id`, companyID, capabilityID, versionDigest)
	if err != nil {
		return err
	}
	type activeRuntimeQualification struct{ id, schema string }
	active := make([]activeRuntimeQualification, 0)
	for rows.Next() {
		var item activeRuntimeQualification
		var status string
		if err = rows.Scan(&item.id, &item.schema, &status); err != nil {
			rows.Close()
			return err
		}
		active = append(active, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, item := range active {
		requestID := stableCapabilityID("mcp-runtime-revoke", decisionID, item.id)
		eventID := stableCapabilityID("mcp-runtime-revoked", decisionID, item.id)
		if _, err = tx.Exec(ctx, `INSERT INTO mcp_runtime_qualification_events(
company_id,event_id,runtime_qualification_id,capability_id,version_digest,status,tool_schema_sha256,rationale,request_id)
VALUES($1,$2,$3,$4,$5,'revoked',$6,$7,$8)`, companyID, eventID, item.id, capabilityID, versionDigest, item.schema, rationale, requestID); err != nil {
			return err
		}
	}
	return nil
}

func (k *Kernel) AuthorizeStdioMCPToolCall(ctx context.Context, binding Binding, capabilityID, toolName, toolSchemaSHA256 string) (StdioMCPToolAuthorization, error) {
	var authorization StdioMCPToolAuthorization
	if ctx == nil || !core.ValidID(capabilityID) || toolName == "" || len(toolName) > mcptransport.MaxStdioToolNameBytes || !validCapabilityDigest(toolSchemaSHA256) {
		return authorization, core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return authorization, err
	}
	defer tx.Rollback(ctx)
	if err = lockCapabilityMCPServerInTransaction(ctx, tx, binding.scope.company, capabilityID); err != nil {
		return authorization, err
	}
	if err = k.guard(ctx, tx, binding.scope, &binding); err != nil {
		return authorization, err
	}
	authorization, err = authorizeStdioMCPToolCallInTransaction(ctx, tx, binding, capabilityID, toolName, toolSchemaSHA256)
	if err != nil {
		return StdioMCPToolAuthorization{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return StdioMCPToolAuthorization{}, err
	}
	return authorization, nil
}

func authorizeStdioMCPToolCallInTransaction(ctx context.Context, tx pgx.Tx, binding Binding, capabilityID, toolName, toolSchemaSHA256 string) (StdioMCPToolAuthorization, error) {
	var authorization StdioMCPToolAuthorization
	versionDigest, capabilityStatus, err := readCapabilityVersion(ctx, tx, binding.scope.company, "mcp", capabilityID)
	if err != nil {
		return authorization, err
	}
	if capabilityStatus != "approved" {
		return authorization, core.Denied
	}
	var transport string
	var serverEndpoint *string
	if err = tx.QueryRow(ctx, `SELECT transport,endpoint FROM mcp_server_definitions WHERE company_id=$1 AND id=$2`, binding.scope.company, capabilityID).Scan(&transport, &serverEndpoint); err != nil {
		return authorization, err
	}
	if transport != "stdio" && transport != "streamable_http" {
		return authorization, core.Denied
	}
	var employeeEvent, employeeDigest, employeeQualification string
	var employeeGrantRevision int64
	if err = tx.QueryRow(ctx, `SELECT event,version_digest,qualification_id,event_seq FROM employee_capability_events
WHERE company_id=$1 AND employee_id=$2 AND capability_kind='mcp' AND capability_id=$3
ORDER BY event_seq DESC LIMIT 1`, binding.scope.company, binding.employee, capabilityID).Scan(&employeeEvent, &employeeDigest, &employeeQualification, &employeeGrantRevision); errors.Is(err, pgx.ErrNoRows) {
		return authorization, core.Denied
	} else if err != nil {
		return authorization, err
	}
	if employeeEvent != "bound" || employeeDigest != versionDigest {
		return authorization, core.Denied
	}
	var metadataDecision string
	if err = tx.QueryRow(ctx, `SELECT decision FROM capability_decisions
WHERE company_id=$1 AND capability_kind='mcp' AND capability_id=$2 AND version_digest=$3 AND qualification_id=$4
ORDER BY created_at DESC,decision_id DESC LIMIT 1`, binding.scope.company, capabilityID, versionDigest, employeeQualification).Scan(&metadataDecision); errors.Is(err, pgx.ErrNoRows) {
		return authorization, core.Denied
	} else if err != nil {
		return authorization, err
	}
	if metadataDecision != "approved" {
		return authorization, core.Denied
	}
	var runtimeQualificationID, runtimeStatus, runtimeSchema, runtimeTransport, runtimeProfile, runtimeEndpoint string
	var runtimeEvidence, toolsJSON []byte
	err = tx.QueryRow(ctx, `SELECT r.runtime_qualification_id,e.status,e.tool_schema_sha256,r.transport,r.runtime_profile,COALESCE(r.endpoint,''),r.process_spec,r.tools
FROM mcp_runtime_qualification_records r
JOIN LATERAL (SELECT status,tool_schema_sha256 FROM mcp_runtime_qualification_events
 WHERE company_id=r.company_id AND runtime_qualification_id=r.runtime_qualification_id
 ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE r.company_id=$1 AND r.capability_id=$2 AND r.capability_qualification_id=$3 AND r.version_digest=$4
ORDER BY r.created_at DESC,r.runtime_qualification_id DESC LIMIT 1`, binding.scope.company, capabilityID, employeeQualification, versionDigest).Scan(&runtimeQualificationID, &runtimeStatus, &runtimeSchema, &runtimeTransport, &runtimeProfile, &runtimeEndpoint, &runtimeEvidence, &toolsJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return authorization, core.Denied
	}
	if err != nil {
		return authorization, err
	}
	if runtimeStatus != "qualified" || runtimeSchema != toolSchemaSHA256 {
		return authorization, core.Denied
	}
	var processSpec mcpowner.ProcessSpec
	switch runtimeTransport {
	case "stdio":
		if runtimeProfile != mcptransport.StdioProfile20260728 || runtimeEndpoint != "" || json.Unmarshal(runtimeEvidence, &processSpec) != nil || mcpowner.ValidateProcessSpec(processSpec) != nil || processSpec.ApprovedToolSchemaSHA256 != runtimeSchema {
			return authorization, core.Integrity
		}
		if processSpec.SourcePackageRevisionID != "" {
			var latestPackageID, latestManifestDigest string
			if err = tx.QueryRow(ctx, `SELECT package_revision_id,manifest_digest FROM mcp_server_package_revisions
WHERE company_id=$1 AND server_id=$2 ORDER BY created_at DESC,package_revision_id LIMIT 1 FOR SHARE`, binding.scope.company, capabilityID).Scan(&latestPackageID, &latestManifestDigest); err != nil {
				return authorization, core.Denied
			}
			if latestPackageID != processSpec.SourcePackageRevisionID || latestManifestDigest != processSpec.SourcePackageManifestSHA256 {
				return authorization, core.Denied
			}
		}
	case "streamable_http":
		if runtimeProfile != mcptransport.StreamableHTTPProfile20260728 || serverEndpoint == nil || runtimeEndpoint != *serverEndpoint || string(runtimeEvidence) != "{}" {
			return authorization, core.Integrity
		}
		var serverName string
		if err = tx.QueryRow(ctx, `SELECT name FROM mcp_server_definitions WHERE company_id=$1 AND id=$2`, binding.scope.company, capabilityID).Scan(&serverName); err != nil {
			return authorization, err
		}
		canonicalEndpoint, descriptorDigest, digestErr := canonicalMCPStreamableHTTPDescriptorDigest(serverName, runtimeEndpoint)
		if digestErr != nil || canonicalEndpoint != runtimeEndpoint || descriptorDigest != versionDigest {
			return authorization, core.Integrity
		}
	default:
		return authorization, core.Denied
	}
	if err = json.Unmarshal(toolsJSON, &authorization.Tools); err != nil {
		return authorization, core.Integrity
	}
	computedSchemaDigest, schemaErr := mcptransport.StdioToolSchemaDigest(toolsJSON)
	if runtimeTransport == "streamable_http" {
		var httpToolList []byte
		httpToolList, schemaErr = json.Marshal(struct {
			Tools []mcptransport.StdioToolDefinition `json:"tools"`
		}{authorization.Tools})
		if schemaErr == nil {
			_, computedSchemaDigest, schemaErr = mcptransport.PrepareStreamableHTTPToolList(httpToolList)
		}
	}
	if schemaErr != nil || computedSchemaDigest != runtimeSchema {
		return authorization, core.Integrity
	}
	if !containsStdioTool(authorization.Tools, toolName) {
		return authorization, core.Denied
	}
	var runtimeRecord StdioMCPRuntimeQualification
	if err = tx.QueryRow(ctx, `SELECT r.company_id,r.runtime_qualification_id,r.capability_id,r.capability_qualification_id,
r.transport,COALESCE(r.endpoint,''),r.version_digest,r.descriptor_digest,r.runtime_profile,r.host_os,r.host_profile,COALESCE(r.command_sha256,''),COALESCE(r.package_manifest_sha256,''),
r.server_name,r.server_version,r.protocol_version,r.tool_schema_sha256,r.evidence_digest,r.created_at::text
FROM mcp_runtime_qualification_records r WHERE r.company_id=$1 AND r.runtime_qualification_id=$2`, binding.scope.company, runtimeQualificationID).Scan(
		&runtimeRecord.CompanyID, &runtimeRecord.RuntimeQualificationID, &runtimeRecord.CapabilityID, &runtimeRecord.CapabilityQualificationID,
		&runtimeRecord.Transport, &runtimeRecord.Endpoint, &runtimeRecord.VersionDigest, &runtimeRecord.DescriptorDigest, &runtimeRecord.RuntimeProfile, &runtimeRecord.HostOS,
		&runtimeRecord.HostProfile, &runtimeRecord.CommandSHA256, &runtimeRecord.PackageManifestSHA256, &runtimeRecord.ServerName,
		&runtimeRecord.ServerVersion, &runtimeRecord.ProtocolVersion, &runtimeRecord.ToolSchemaSHA256, &runtimeRecord.EvidenceDigest,
		&runtimeRecord.CreatedAt); err != nil {
		return authorization, err
	}
	if runtimeRecord.Transport != runtimeTransport || runtimeRecord.CapabilityID != capabilityID || runtimeRecord.CapabilityQualificationID != employeeQualification || runtimeRecord.VersionDigest != versionDigest || runtimeRecord.DescriptorDigest != versionDigest {
		return authorization, core.Integrity
	}
	if runtimeTransport == "stdio" {
		if runtimeRecord.HostOS != "windows" || runtimeRecord.HostProfile != stdioMCPHostProfileWindowsAppContainer ||
			processSpec.CommandSHA256 != runtimeRecord.CommandSHA256 || processSpec.PackageManifestSHA256 != runtimeRecord.PackageManifestSHA256 ||
			processSpec.ExpectedServer.Name != runtimeRecord.ServerName || processSpec.ExpectedServer.Version != runtimeRecord.ServerVersion {
			return authorization, core.Integrity
		}
	} else if runtimeRecord.HostOS != "remote" || runtimeRecord.HostProfile != "https_public_dns_pinned@1" || runtimeRecord.Endpoint != runtimeEndpoint || runtimeRecord.ServerName == "" || runtimeRecord.ServerVersion != "unreported" {
		return authorization, core.Integrity
	}
	authorization.RuntimeQualification = runtimeRecord
	authorization.ProcessSpec = processSpec
	authorization.Transport = runtimeTransport
	authorization.Endpoint = runtimeEndpoint
	authorization.GrantRevision = employeeGrantRevision
	authorization.TargetSHA256 = fingerprint(struct {
		CompanyID, EmployeeID, CapabilityID, CapabilityVersion, CapabilityQualificationID string
		RuntimeQualificationID, Transport, Endpoint, CommandSHA256, PackageManifestSHA256 string
		ToolName, ToolSchemaSHA256                                                        string
	}{binding.scope.company, binding.employee, capabilityID, versionDigest, employeeQualification,
		runtimeQualificationID, runtimeTransport, runtimeEndpoint, runtimeRecord.CommandSHA256, runtimeRecord.PackageManifestSHA256,
		toolName, toolSchemaSHA256})
	return authorization, nil
}

func containsStdioTool(tools []mcptransport.StdioToolDefinition, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func (k *Kernel) getStdioMCPRuntimeQualification(ctx context.Context, companyID, runtimeQualificationID string) (StdioMCPRuntimeQualification, error) {
	var record StdioMCPRuntimeQualification
	err := k.pool.QueryRow(ctx, `SELECT r.company_id,r.runtime_qualification_id,r.capability_id,r.capability_qualification_id,
r.transport,COALESCE(r.endpoint,''),r.version_digest,r.descriptor_digest,r.runtime_profile,r.host_os,r.host_profile,COALESCE(r.command_sha256,''),COALESCE(r.package_manifest_sha256,''),
r.server_name,r.server_version,r.protocol_version,r.tool_schema_sha256,e.status,r.evidence_digest,r.created_at::text
FROM mcp_runtime_qualification_records r
JOIN LATERAL (SELECT status FROM mcp_runtime_qualification_events
 WHERE company_id=r.company_id AND runtime_qualification_id=r.runtime_qualification_id
 ORDER BY event_seq DESC LIMIT 1) e ON true
	WHERE r.company_id=$1 AND r.runtime_qualification_id=$2`, companyID, runtimeQualificationID).Scan(
		&record.CompanyID, &record.RuntimeQualificationID, &record.CapabilityID, &record.CapabilityQualificationID,
		&record.Transport, &record.Endpoint, &record.VersionDigest, &record.DescriptorDigest, &record.RuntimeProfile, &record.HostOS, &record.HostProfile,
		&record.CommandSHA256, &record.PackageManifestSHA256, &record.ServerName, &record.ServerVersion,
		&record.ProtocolVersion, &record.ToolSchemaSHA256, &record.Status, &record.EvidenceDigest, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return StdioMCPRuntimeQualification{}, core.OutOfScope
	}
	return record, err
}

func listStdioMCPRuntimeQualifications(ctx context.Context, tx pgx.Tx, companyID string) ([]StdioMCPRuntimeQualification, error) {
	rows, err := tx.Query(ctx, `SELECT r.company_id,r.runtime_qualification_id,r.capability_id,r.capability_qualification_id,
r.transport,COALESCE(r.endpoint,''),r.version_digest,r.descriptor_digest,r.runtime_profile,r.host_os,r.host_profile,COALESCE(r.command_sha256,''),COALESCE(r.package_manifest_sha256,''),
r.server_name,r.server_version,r.protocol_version,r.tool_schema_sha256,e.status,r.evidence_digest,r.created_at::text
FROM mcp_runtime_qualification_records r
JOIN LATERAL (SELECT status FROM mcp_runtime_qualification_events
 WHERE company_id=r.company_id AND runtime_qualification_id=r.runtime_qualification_id
 ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE r.company_id=$1 ORDER BY r.created_at DESC,r.runtime_qualification_id`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	qualifications := make([]StdioMCPRuntimeQualification, 0)
	for rows.Next() {
		var item StdioMCPRuntimeQualification
		if err = rows.Scan(&item.CompanyID, &item.RuntimeQualificationID, &item.CapabilityID, &item.CapabilityQualificationID,
			&item.Transport, &item.Endpoint, &item.VersionDigest, &item.DescriptorDigest, &item.RuntimeProfile, &item.HostOS, &item.HostProfile,
			&item.CommandSHA256, &item.PackageManifestSHA256, &item.ServerName, &item.ServerVersion,
			&item.ProtocolVersion, &item.ToolSchemaSHA256, &item.Status, &item.EvidenceDigest, &item.CreatedAt); err != nil {
			return nil, err
		}
		qualifications = append(qualifications, item)
	}
	return qualifications, rows.Err()
}

func validateStdioMCPRuntimeObservation(processSpec mcpowner.ProcessSpec, tools []mcptransport.StdioToolDefinition, serverIdentity mcptransport.StdioServerIdentity, schemaDigest string) error {
	if processSpec.Launch.NetworkPolicy != "deny_all" || processSpec.Launch.RegistryProxyEndpoint != "" || processSpec.ApprovedToolSchemaSHA256 == "" || !validCapabilityDigest(processSpec.CommandSHA256) || !validCapabilityDigest(processSpec.PackageManifestSHA256) || !validCapabilityDigest(schemaDigest) {
		return core.Denied
	}
	if serverIdentity.Name == "" || serverIdentity.Version == "" || serverIdentity.Name != processSpec.ExpectedServer.Name || serverIdentity.Version != processSpec.ExpectedServer.Version || len(tools) == 0 {
		return core.Malformed
	}
	return nil
}
