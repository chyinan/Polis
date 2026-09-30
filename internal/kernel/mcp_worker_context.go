// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/mcpowner"
	"polis/internal/mcptransport"
)

type BoundStdioMCPToolSet struct {
	CapabilityID           string                             `json:"capabilityId"`
	Name                   string                             `json:"name"`
	RuntimeQualificationID string                             `json:"runtimeQualificationId"`
	Transport              string                             `json:"transport"`
	Endpoint               string                             `json:"endpoint,omitempty"`
	ToolSchemaSHA256       string                             `json:"toolSchemaSha256"`
	Tools                  []mcptransport.StdioToolDefinition `json:"tools"`
}

type BoundMCPToolSet = BoundStdioMCPToolSet

// BoundStdioMCPToolSets preserves the stdio-only projection for deterministic
// and historical callers.
func (k *Kernel) BoundStdioMCPToolSets(ctx context.Context, binding Binding) ([]BoundStdioMCPToolSet, error) {
	return k.BoundMCPToolSets(ctx, binding, true, false)
}

// BoundMCPToolSets returns the current qualified profiles pinned to the active
// Employee. HTTP tools are included only when the trusted adapter has enabled
// their external egress gate; every call still passes AuthorizeStdioMCPToolCall.
func (k *Kernel) BoundMCPToolSets(ctx context.Context, binding Binding, includeStdio, includeStreamableHTTP bool) ([]BoundMCPToolSet, error) {
	if ctx == nil {
		return nil, core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = k.guard(ctx, tx, binding.scope, &binding); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT ON (capability_id) capability_id,event,version_digest,qualification_id
FROM employee_capability_events
WHERE company_id=$1 AND employee_id=$2 AND capability_kind='mcp'
ORDER BY capability_id,event_seq DESC`, binding.scope.company, binding.employee)
	if err != nil {
		return nil, err
	}
	type bindingRow struct{ capabilityID, event, versionDigest, qualificationID string }
	bindings := make([]bindingRow, 0, 2)
	for rows.Next() {
		var item bindingRow
		if err = rows.Scan(&item.capabilityID, &item.event, &item.versionDigest, &item.qualificationID); err != nil {
			rows.Close()
			return nil, err
		}
		if item.event == "bound" {
			bindings = append(bindings, item)
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	sets := make([]BoundMCPToolSet, 0, len(bindings))
	for _, employeeBinding := range bindings {
		versionDigest, status, versionErr := readCapabilityVersion(ctx, tx, binding.scope.company, "mcp", employeeBinding.capabilityID)
		if errors.Is(versionErr, core.OutOfScope) {
			continue
		}
		if versionErr != nil {
			return nil, versionErr
		}
		if status != "approved" || versionDigest != employeeBinding.versionDigest {
			continue
		}
		var name, transport string
		var serverEndpoint *string
		if err = tx.QueryRow(ctx, `SELECT name,transport,endpoint FROM mcp_server_definitions WHERE company_id=$1 AND id=$2`, binding.scope.company, employeeBinding.capabilityID).Scan(&name, &transport, &serverEndpoint); err != nil {
			return nil, err
		}
		if (transport == "stdio" && !includeStdio) || (transport == "streamable_http" && !includeStreamableHTTP) || (transport != "stdio" && transport != "streamable_http") {
			continue
		}
		var decision string
		if err = tx.QueryRow(ctx, `SELECT decision FROM capability_decisions
WHERE company_id=$1 AND capability_kind='mcp' AND capability_id=$2 AND version_digest=$3 AND qualification_id=$4
ORDER BY created_at DESC,decision_id DESC LIMIT 1`, binding.scope.company, employeeBinding.capabilityID, versionDigest, employeeBinding.qualificationID).Scan(&decision); errors.Is(err, pgx.ErrNoRows) {
			continue
		} else if err != nil {
			return nil, err
		}
		if decision != "approved" {
			continue
		}
		var runtimeID, runtimeStatus, runtimeSchema, runtimeProfile, hostOS, hostProfile, runtimeTransport, runtimeEndpoint string
		var toolsJSON, processSpecJSON []byte
		err = tx.QueryRow(ctx, `SELECT r.runtime_qualification_id,e.status,e.tool_schema_sha256,r.runtime_profile,r.host_os,r.host_profile,r.transport,COALESCE(r.endpoint,''),r.tools,r.process_spec
FROM mcp_runtime_qualification_records r
JOIN LATERAL (SELECT status,tool_schema_sha256 FROM mcp_runtime_qualification_events
 WHERE company_id=r.company_id AND runtime_qualification_id=r.runtime_qualification_id
 ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE r.company_id=$1 AND r.capability_id=$2 AND r.capability_qualification_id=$3 AND r.version_digest=$4
ORDER BY r.created_at DESC,r.runtime_qualification_id DESC LIMIT 1`, binding.scope.company, employeeBinding.capabilityID, employeeBinding.qualificationID, versionDigest).Scan(
			&runtimeID, &runtimeStatus, &runtimeSchema, &runtimeProfile, &hostOS, &hostProfile, &runtimeTransport, &runtimeEndpoint, &toolsJSON, &processSpecJSON)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if runtimeStatus != "qualified" {
			continue
		}
		if runtimeTransport != transport {
			return nil, core.Integrity
		}
		var tools []mcptransport.StdioToolDefinition
		if err = json.Unmarshal(toolsJSON, &tools); err != nil {
			return nil, core.Integrity
		}
		var computedDigest string
		var digestErr error
		if transport == "stdio" {
			var processSpec mcpowner.ProcessSpec
			if json.Unmarshal(processSpecJSON, &processSpec) != nil || mcpowner.ValidateProcessSpec(processSpec) != nil ||
				processSpec.ApprovedToolSchemaSHA256 != runtimeSchema || hostOS != "windows" || hostProfile != stdioMCPHostProfileWindowsAppContainer || runtimeProfile != mcptransport.StdioProfile20260728 || runtimeEndpoint != "" {
				return nil, core.Integrity
			}
			computedDigest, digestErr = mcptransport.StdioToolSchemaDigest(toolsJSON)
		} else {
			if serverEndpoint == nil || runtimeEndpoint != *serverEndpoint || hostOS != "remote" || hostProfile != "https_public_dns_pinned@1" || runtimeProfile != mcptransport.StreamableHTTPProfile20260728 || string(processSpecJSON) != "{}" {
				return nil, core.Integrity
			}
			canonicalEndpoint, descriptorDigest, endpointErr := canonicalMCPStreamableHTTPDescriptorDigest(name, runtimeEndpoint)
			if endpointErr != nil || canonicalEndpoint != runtimeEndpoint || descriptorDigest != versionDigest {
				return nil, core.Integrity
			}
			httpToolList, marshalErr := json.Marshal(struct {
				Tools []mcptransport.StdioToolDefinition `json:"tools"`
			}{tools})
			if marshalErr != nil {
				return nil, core.Integrity
			}
			_, computedDigest, digestErr = mcptransport.PrepareStreamableHTTPToolList(httpToolList)
		}
		if digestErr != nil || computedDigest != runtimeSchema || len(tools) == 0 {
			return nil, core.Integrity
		}
		item := BoundMCPToolSet{
			CapabilityID: employeeBinding.capabilityID, Name: name, RuntimeQualificationID: runtimeID,
			Transport: transport, ToolSchemaSHA256: runtimeSchema, Tools: tools,
		}
		if transport == "streamable_http" {
			item.Endpoint = runtimeEndpoint
		}
		sets = append(sets, item)
	}
	if len(sets) > 1 {
		return nil, core.Denied
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return sets, nil
}
