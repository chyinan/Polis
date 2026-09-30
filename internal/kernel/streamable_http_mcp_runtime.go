// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/mcptransport"
)

type StreamableHTTPMCPRuntimeObservationInput struct {
	CapabilityID              string
	CapabilityQualificationID string
	ToolList                  json.RawMessage
	RequestID                 string
}

// StreamableHTTPMCPObservationTarget returns the endpoint only when the
// descriptor has a current metadata qualification and a current human approval.
// Callers must hold LockStdioMCPPackageServer through the network read and the
// subsequent TXRecordStreamableHTTPMCPRuntimeQualification call.
func (k *Kernel) StreamableHTTPMCPObservationTarget(ctx context.Context, companyID, capabilityID, qualificationID string) (string, error) {
	if ctx == nil || !core.ValidID(companyID) || !core.ValidID(capabilityID) || !core.ValidID(qualificationID) {
		return "", core.Malformed
	}
	var name, descriptorDigest, status, transport, metadataVersion, metadataProfile, metadataStatus, decision string
	var endpoint *string
	err := k.pool.QueryRow(ctx, `SELECT d.name,d.endpoint,d.descriptor_digest,d.status,d.transport,
q.version_digest,q.profile,q.status,COALESCE(decision.decision,'')
FROM mcp_server_definitions d
JOIN capability_qualification_records q
  ON q.company_id=d.company_id AND q.capability_id=d.id AND q.capability_kind='mcp'
LEFT JOIN LATERAL (
 SELECT decision FROM capability_decisions
 WHERE company_id=d.company_id AND capability_kind='mcp' AND capability_id=d.id
   AND qualification_id=q.qualification_id AND version_digest=q.version_digest
 ORDER BY created_at DESC,decision_id DESC LIMIT 1
) decision ON true
WHERE d.company_id=$1 AND d.id=$2 AND q.qualification_id=$3`, companyID, capabilityID, qualificationID).Scan(
		&name, &endpoint, &descriptorDigest, &status, &transport, &metadataVersion, &metadataProfile, &metadataStatus, &decision)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", core.OutOfScope
	}
	if err != nil {
		return "", err
	}
	if endpoint == nil || status != "approved" || transport != "streamable_http" || metadataProfile != "streamable_http_mcp_2026_07_28@1" ||
		metadataStatus != "metadata_verified" || metadataVersion != descriptorDigest || decision != "approved" {
		return "", core.Denied
	}
	canonical, computedDigest, err := canonicalMCPStreamableHTTPDescriptorDigest(name, *endpoint)
	if err != nil || canonical != *endpoint || computedDigest != descriptorDigest {
		return "", core.Integrity
	}
	return canonical, nil
}

func (k *Kernel) TXRecordStreamableHTTPMCPRuntimeQualification(ctx context.Context, companyID string, input StreamableHTTPMCPRuntimeObservationInput) (StdioMCPRuntimeQualification, error) {
	if ctx == nil || !core.ValidID(companyID) || !core.ValidID(input.CapabilityID) || !core.ValidID(input.CapabilityQualificationID) || !core.ValidID(input.RequestID) {
		return StdioMCPRuntimeQualification{}, core.Malformed
	}
	tools, schemaDigest, err := mcptransport.PrepareStreamableHTTPToolList(input.ToolList)
	if err != nil {
		return StdioMCPRuntimeQualification{}, core.Malformed
	}
	toolsJSON, err := json.Marshal(tools)
	if err != nil || len(toolsJSON) > mcptransport.MaxStdioToolListBytes {
		return StdioMCPRuntimeQualification{}, core.TooLarge
	}
	runtimeQualificationID := stableCapabilityID("mcp-http-runtime", companyID, input.RequestID)
	scope := k.LocalScope(companyID)
	writeInput := struct {
		CapabilityID              string
		CapabilityQualificationID string
		ToolSchemaSHA256          string
		ToolList                  json.RawMessage
	}{input.CapabilityID, input.CapabilityQualificationID, schemaDigest, input.ToolList}
	_, err = k.TXWrite(ctx, scope, nil, input.RequestID, "capability.mcp.http.runtime.observe", writeInput, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if err := lockCapabilityMCPServerInTransaction(ctx, tx, companyID, input.CapabilityID); err != nil {
			return Receipt{}, err
		}
		var name, transport, descriptorDigest, capabilityStatus string
		var endpoint *string
		if err := tx.QueryRow(ctx, `SELECT name,transport,endpoint,descriptor_digest,status
FROM mcp_server_definitions WHERE company_id=$1 AND id=$2 FOR SHARE`, companyID, input.CapabilityID).Scan(&name, &transport, &endpoint, &descriptorDigest, &capabilityStatus); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if transport != "streamable_http" || endpoint == nil || capabilityStatus != "approved" {
			return Receipt{}, core.Denied
		}
		canonicalEndpoint, canonicalDigest, digestErr := canonicalMCPStreamableHTTPDescriptorDigest(name, *endpoint)
		if digestErr != nil || canonicalEndpoint != *endpoint || canonicalDigest != descriptorDigest {
			return Receipt{}, core.Integrity
		}
		var metadataVersion, metadataProfile, metadataStatus string
		if err := tx.QueryRow(ctx, `SELECT version_digest,profile,status FROM capability_qualification_records
WHERE company_id=$1 AND qualification_id=$2 AND capability_kind='mcp' AND capability_id=$3 FOR SHARE`, companyID, input.CapabilityQualificationID, input.CapabilityID).Scan(&metadataVersion, &metadataProfile, &metadataStatus); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if metadataVersion != descriptorDigest || metadataProfile != "streamable_http_mcp_2026_07_28@1" || metadataStatus != "metadata_verified" {
			return Receipt{}, core.Denied
		}
		var decision string
		if err := tx.QueryRow(ctx, `SELECT decision FROM capability_decisions
WHERE company_id=$1 AND capability_kind='mcp' AND capability_id=$2 AND version_digest=$3 AND qualification_id=$4
ORDER BY created_at DESC,decision_id DESC LIMIT 1`, companyID, input.CapabilityID, descriptorDigest, input.CapabilityQualificationID).Scan(&decision); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Denied
		} else if err != nil {
			return Receipt{}, err
		}
		if decision != "approved" {
			return Receipt{}, core.Denied
		}
		evidenceBytes, marshalErr := json.Marshal(struct {
			CompanyID                 string `json:"companyId"`
			RuntimeQualificationID    string `json:"runtimeQualificationId"`
			CapabilityID              string `json:"capabilityId"`
			CapabilityQualificationID string `json:"capabilityQualificationId"`
			Transport                 string `json:"transport"`
			Endpoint                  string `json:"endpoint"`
			VersionDigest             string `json:"versionDigest"`
			RuntimeProfile            string `json:"runtimeProfile"`
			ProtocolVersion           string `json:"protocolVersion"`
			ToolSchemaSHA256          string `json:"toolSchemaSha256"`
		}{companyID, runtimeQualificationID, input.CapabilityID, input.CapabilityQualificationID, "streamable_http", canonicalEndpoint,
			descriptorDigest, mcptransport.StreamableHTTPProfile20260728, mcptransport.ProtocolVersion20260728, schemaDigest})
		if marshalErr != nil {
			return Receipt{}, core.Integrity
		}
		evidenceDigest := digestCapabilityBytes(evidenceBytes)
		if _, err := tx.Exec(ctx, `INSERT INTO mcp_runtime_qualification_records(
company_id,runtime_qualification_id,capability_id,capability_qualification_id,version_digest,descriptor_digest,
runtime_profile,host_os,host_profile,transport,endpoint,command_sha256,package_manifest_sha256,server_name,server_version,
protocol_version,tool_schema_sha256,tools,process_spec,evidence_digest,request_id)
VALUES($1,$2,$3,$4,$5,$5,$6,'remote','https_public_dns_pinned@1','streamable_http',$7,NULL,NULL,$8,'unreported',$9,$10,$11,'{}'::jsonb,$12,$13)`,
			companyID, runtimeQualificationID, input.CapabilityID, input.CapabilityQualificationID, descriptorDigest,
			mcptransport.StreamableHTTPProfile20260728, canonicalEndpoint, name, mcptransport.ProtocolVersion20260728, schemaDigest, toolsJSON, evidenceDigest, input.RequestID); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		eventID := stableCapabilityID("mcp-http-runtime-event", companyID, input.RequestID)
		if _, err := tx.Exec(ctx, `INSERT INTO mcp_runtime_qualification_events(
company_id,event_id,runtime_qualification_id,capability_id,version_digest,status,tool_schema_sha256,rationale,request_id)
VALUES($1,$2,$3,$4,$5,'observed_unqualified',$6,'Streamable HTTP tools/list schema observed',$7)`, companyID, eventID, runtimeQualificationID, input.CapabilityID, descriptorDigest, schemaDigest, input.RequestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: runtimeQualificationID, Status: "observed_unqualified"}, nil
	})
	if err != nil {
		return StdioMCPRuntimeQualification{}, err
	}
	return k.getStdioMCPRuntimeQualification(ctx, companyID, runtimeQualificationID)
}
