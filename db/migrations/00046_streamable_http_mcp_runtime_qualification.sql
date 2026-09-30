-- +goose Up
-- Extend the existing append-only MCP runtime qualification and one-shot call
-- ledger with the fixed Streamable HTTP profile. Existing stdio rows retain
-- their original shape and constraints.
ALTER TABLE mcp_runtime_qualification_records
 ADD COLUMN transport text NOT NULL DEFAULT 'stdio',
 ADD COLUMN endpoint text;

ALTER TABLE mcp_runtime_qualification_records
 DROP CONSTRAINT mcp_runtime_qualification_records_runtime_profile_check,
 DROP CONSTRAINT mcp_runtime_qualification_records_host_os_check,
 DROP CONSTRAINT mcp_runtime_qualification_records_host_profile_check,
 DROP CONSTRAINT mcp_runtime_qualification_records_command_sha256_check,
 DROP CONSTRAINT mcp_runtime_qualification_records_package_manifest_sha256_check,
 DROP CONSTRAINT mcp_runtime_qualification_process_spec_matches_digests;

ALTER TABLE mcp_runtime_qualification_records
 ALTER COLUMN command_sha256 DROP NOT NULL,
 ALTER COLUMN package_manifest_sha256 DROP NOT NULL,
 ADD CONSTRAINT mcp_runtime_qualification_records_transport_shape CHECK (
   (transport='stdio'
     AND runtime_profile='polis-controlled-stdio-mcp-2026-07-28@1'
     AND host_os='windows'
     AND host_profile='windows_appcontainer_deny_all@1'
     AND command_sha256 IS NOT NULL
     AND command_sha256 ~ '^[a-f0-9]{64}$'
     AND package_manifest_sha256 IS NOT NULL
     AND package_manifest_sha256 ~ '^[a-f0-9]{64}$'
     AND endpoint IS NULL)
   OR
   (transport='streamable_http'
     AND runtime_profile='polis-streamable-http-mcp-2026-07-28@1'
     AND host_os='remote'
     AND host_profile='https_public_dns_pinned@1'
     AND command_sha256 IS NULL
     AND package_manifest_sha256 IS NULL
     AND endpoint IS NOT NULL
     AND process_spec='{}'::jsonb)
 ),
 ADD CONSTRAINT mcp_runtime_qualification_records_http_endpoint_bound CHECK (
   endpoint IS NULL OR (octet_length(endpoint) BETWEEN 9 AND 512 AND endpoint ~ '^https://[^/?#]+(/[^?#]*)?$')
 ),
 ADD CONSTRAINT mcp_runtime_qualification_process_spec_matches_digests CHECK (
   (transport='stdio'
     AND COALESCE(process_spec->>'CommandSHA256','')=command_sha256
     AND COALESCE(process_spec->>'PackageManifestSHA256','')=package_manifest_sha256
     AND COALESCE(process_spec->>'ApprovedToolSchemaSHA256','')=tool_schema_sha256
     AND COALESCE(process_spec->'Launch'->>'NetworkPolicy','')='deny_all'
     AND COALESCE(process_spec->'Launch'->>'RegistryProxyEndpoint','')='')
   OR (transport='streamable_http' AND process_spec='{}'::jsonb)
 );

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enforce_mcp_runtime_qualification_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 runtime_record RECORD;
 prior_status text;
 capability_status text;
 capability_transport text;
 capability_endpoint text;
 approval_count integer;
BEGIN
 SELECT capability_id,version_digest,capability_qualification_id,tool_schema_sha256,transport,endpoint,runtime_profile
 INTO runtime_record
 FROM mcp_runtime_qualification_records
 WHERE company_id=NEW.company_id AND runtime_qualification_id=NEW.runtime_qualification_id
 FOR SHARE;
 IF NOT FOUND OR NEW.capability_id IS DISTINCT FROM runtime_record.capability_id
    OR NEW.version_digest IS DISTINCT FROM runtime_record.version_digest
    OR NEW.tool_schema_sha256 IS DISTINCT FROM runtime_record.tool_schema_sha256 THEN
  RAISE EXCEPTION 'MCP runtime event differs from its immutable qualification record' USING ERRCODE='23514';
 END IF;

 SELECT status,transport,endpoint INTO capability_status,capability_transport,capability_endpoint
 FROM mcp_server_definitions
 WHERE company_id=NEW.company_id AND id=runtime_record.capability_id
   AND descriptor_digest=runtime_record.version_digest FOR SHARE;
 IF NOT FOUND OR capability_transport IS DISTINCT FROM runtime_record.transport
    OR capability_endpoint IS DISTINCT FROM runtime_record.endpoint
    OR (runtime_record.transport='stdio' AND runtime_record.runtime_profile<>'polis-controlled-stdio-mcp-2026-07-28@1')
    OR (runtime_record.transport='streamable_http' AND runtime_record.runtime_profile<>'polis-streamable-http-mcp-2026-07-28@1') THEN
  RAISE EXCEPTION 'MCP runtime qualification is not bound to the current transport descriptor' USING ERRCODE='23514';
 END IF;

 SELECT status INTO prior_status FROM mcp_runtime_qualification_events
 WHERE company_id=NEW.company_id AND runtime_qualification_id=NEW.runtime_qualification_id
 ORDER BY event_seq DESC LIMIT 1;
 IF NEW.status='observed_unqualified' THEN
  IF prior_status IS NOT NULL OR capability_status IS DISTINCT FROM 'approved' THEN
   RAISE EXCEPTION 'MCP runtime observation requires the current approved descriptor and may be recorded once' USING ERRCODE='23514';
  END IF;
 ELSIF NEW.status='qualified' THEN
  IF prior_status IS DISTINCT FROM 'observed_unqualified' OR capability_status IS DISTINCT FROM 'approved' THEN
   RAISE EXCEPTION 'MCP runtime approval requires an observation for the current approved descriptor' USING ERRCODE='23514';
  END IF;
  SELECT count(*) INTO approval_count FROM capability_decisions
  WHERE company_id=NEW.company_id AND capability_kind='mcp' AND capability_id=runtime_record.capability_id
    AND version_digest=runtime_record.version_digest
    AND qualification_id=runtime_record.capability_qualification_id AND decision='approved';
  IF approval_count=0 THEN
   RAISE EXCEPTION 'MCP runtime approval requires current metadata approval' USING ERRCODE='23514';
  END IF;
 ELSIF NEW.status='schema_drift' THEN
  IF prior_status IS NULL OR prior_status NOT IN ('observed_unqualified','qualified') THEN
   RAISE EXCEPTION 'MCP schema drift requires an active observed runtime' USING ERRCODE='23514';
  END IF;
 ELSIF NEW.status='revoked' THEN
  IF prior_status IS NULL OR prior_status NOT IN ('observed_unqualified','qualified') THEN
   RAISE EXCEPTION 'MCP runtime revocation requires an active qualification state' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enforce_mcp_tool_call_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 intent_record RECORD;
 prior_status text;
 runtime_status text;
 session_employee text;
 bound_event text;
 bound_version text;
 bound_qualification text;
 capability_descriptor text;
 capability_status text;
 metadata_decision text;
 result_document jsonb;
 expected_boundary text;
BEGIN
 SELECT i.session_id,i.employee_id,i.capability_id,i.runtime_qualification_id,i.tool_name,i.tool_schema_sha256,
        r.capability_id AS runtime_capability_id,r.tool_schema_sha256 AS runtime_schema,r.tools,
        r.capability_qualification_id,r.version_digest,r.runtime_profile
 INTO intent_record
 FROM mcp_tool_call_intents i
 JOIN mcp_runtime_qualification_records r
   ON r.company_id=i.company_id AND r.runtime_qualification_id=i.runtime_qualification_id
 WHERE i.company_id=NEW.company_id AND i.intent_id=NEW.intent_id
 FOR UPDATE OF i;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'MCP tool-call event has no immutable intent' USING ERRCODE='23503';
 END IF;
 SELECT employee_id INTO session_employee FROM worker_sessions
 WHERE company_id=NEW.company_id AND id=intent_record.session_id FOR SHARE;
 IF NOT FOUND OR session_employee IS DISTINCT FROM intent_record.employee_id
    OR intent_record.runtime_capability_id IS DISTINCT FROM intent_record.capability_id
    OR intent_record.runtime_schema IS DISTINCT FROM intent_record.tool_schema_sha256
    OR NOT (intent_record.tools @> jsonb_build_array(jsonb_build_object('name',intent_record.tool_name))) THEN
  RAISE EXCEPTION 'MCP tool-call intent does not match its session and pinned runtime' USING ERRCODE='23514';
 END IF;
 SELECT status INTO prior_status FROM mcp_tool_call_events
 WHERE company_id=NEW.company_id AND intent_id=NEW.intent_id
 ORDER BY event_seq DESC LIMIT 1;
 IF NEW.status='dispatching' THEN
  IF prior_status IS NOT NULL THEN
   RAISE EXCEPTION 'MCP tool-call intent can be dispatched only once' USING ERRCODE='23514';
  END IF;
  SELECT event,version_digest,qualification_id INTO bound_event,bound_version,bound_qualification
  FROM employee_capability_events
  WHERE company_id=NEW.company_id AND employee_id=intent_record.employee_id
    AND capability_kind='mcp' AND capability_id=intent_record.capability_id
  ORDER BY event_seq DESC LIMIT 1;
  SELECT descriptor_digest,status INTO capability_descriptor,capability_status
  FROM mcp_server_definitions WHERE company_id=NEW.company_id AND id=intent_record.capability_id;
  SELECT decision INTO metadata_decision FROM capability_decisions
  WHERE company_id=NEW.company_id AND capability_kind='mcp' AND capability_id=intent_record.capability_id
    AND version_digest=intent_record.version_digest AND qualification_id=intent_record.capability_qualification_id
  ORDER BY created_at DESC,decision_id DESC LIMIT 1;
  IF bound_event IS DISTINCT FROM 'bound'
     OR bound_version IS DISTINCT FROM intent_record.version_digest
     OR bound_qualification IS DISTINCT FROM intent_record.capability_qualification_id
     OR capability_descriptor IS DISTINCT FROM intent_record.version_digest
     OR capability_status IS DISTINCT FROM 'approved'
     OR metadata_decision IS DISTINCT FROM 'approved' THEN
   RAISE EXCEPTION 'MCP tool-call intent requires current approval and employee binding' USING ERRCODE='23514';
  END IF;
  SELECT e.status INTO runtime_status FROM mcp_runtime_qualification_events e
  WHERE e.company_id=NEW.company_id AND e.runtime_qualification_id=intent_record.runtime_qualification_id
  ORDER BY e.event_seq DESC LIMIT 1;
  IF runtime_status IS DISTINCT FROM 'qualified' THEN
   RAISE EXCEPTION 'MCP tool-call intent requires a current qualified runtime' USING ERRCODE='23514';
  END IF;
 ELSIF prior_status IS DISTINCT FROM 'dispatching' THEN
  RAISE EXCEPTION 'MCP tool-call result must follow a pending one-shot intent' USING ERRCODE='23514';
 END IF;
 IF NEW.status='completed' THEN
  BEGIN
   result_document := convert_from(NEW.result,'UTF8')::jsonb;
  EXCEPTION WHEN others THEN
   RAISE EXCEPTION 'MCP tool-call result is not valid UTF-8 JSON' USING ERRCODE='23514';
  END;
  expected_boundary := CASE intent_record.runtime_profile
    WHEN 'polis-controlled-stdio-mcp-2026-07-28@1' THEN 'untrusted_stdio_mcp_text'
    WHEN 'polis-streamable-http-mcp-2026-07-28@1' THEN 'untrusted_streamable_http_mcp_text'
    ELSE NULL END;
  IF expected_boundary IS NULL OR jsonb_typeof(result_document) IS DISTINCT FROM 'object'
     OR result_document->>'toolName' IS DISTINCT FROM intent_record.tool_name
     OR result_document->>'toolSchemaSha256' IS DISTINCT FROM intent_record.tool_schema_sha256
     OR result_document->>'contentBoundary' IS DISTINCT FROM expected_boundary THEN
   RAISE EXCEPTION 'MCP tool-call result does not match its approved untrusted-text boundary' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd

-- +goose Down
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM mcp_runtime_qualification_records WHERE transport='streamable_http') THEN
  RAISE EXCEPTION 'cannot roll back Streamable HTTP runtime qualification evidence';
 END IF;
END $$;

ALTER TABLE mcp_runtime_qualification_records
 DROP CONSTRAINT mcp_runtime_qualification_records_transport_shape,
 DROP CONSTRAINT mcp_runtime_qualification_records_http_endpoint_bound,
 DROP CONSTRAINT mcp_runtime_qualification_process_spec_matches_digests;
ALTER TABLE mcp_runtime_qualification_records
 ALTER COLUMN command_sha256 SET NOT NULL,
 ALTER COLUMN package_manifest_sha256 SET NOT NULL,
 DROP COLUMN endpoint,
 DROP COLUMN transport,
 ADD CONSTRAINT mcp_runtime_qualification_records_runtime_profile_check CHECK(runtime_profile='polis-controlled-stdio-mcp-2026-07-28@1'),
 ADD CONSTRAINT mcp_runtime_qualification_records_host_os_check CHECK(host_os='windows'),
 ADD CONSTRAINT mcp_runtime_qualification_records_host_profile_check CHECK(host_profile='windows_appcontainer_deny_all@1'),
 ADD CONSTRAINT mcp_runtime_qualification_records_command_sha256_check CHECK(command_sha256 ~ '^[a-f0-9]{64}$'),
 ADD CONSTRAINT mcp_runtime_qualification_records_package_manifest_sha256_check CHECK(package_manifest_sha256 ~ '^[a-f0-9]{64}$'),
 ADD CONSTRAINT mcp_runtime_qualification_process_spec_matches_digests CHECK(COALESCE(process_spec->>'CommandSHA256','')=command_sha256
   AND COALESCE(process_spec->>'PackageManifestSHA256','')=package_manifest_sha256
   AND COALESCE(process_spec->>'ApprovedToolSchemaSHA256','')=tool_schema_sha256
   AND COALESCE(process_spec->'Launch'->>'NetworkPolicy','')='deny_all'
   AND COALESCE(process_spec->'Launch'->>'RegistryProxyEndpoint','')='');

-- Restore the stdio-only runtime-event profile binding and result marker.
-- Existing records are immutable and remain unchanged by this downgrade.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enforce_mcp_runtime_qualification_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 runtime_record RECORD;
 prior_status text;
 capability_status text;
 approval_count integer;
BEGIN
 SELECT capability_id,version_digest,capability_qualification_id,tool_schema_sha256
 INTO runtime_record FROM mcp_runtime_qualification_records
 WHERE company_id=NEW.company_id AND runtime_qualification_id=NEW.runtime_qualification_id FOR SHARE;
 IF NOT FOUND OR NEW.capability_id IS DISTINCT FROM runtime_record.capability_id
    OR NEW.version_digest IS DISTINCT FROM runtime_record.version_digest
    OR NEW.tool_schema_sha256 IS DISTINCT FROM runtime_record.tool_schema_sha256 THEN
  RAISE EXCEPTION 'MCP runtime event differs from its immutable qualification record' USING ERRCODE='23514';
 END IF;
 SELECT status INTO capability_status FROM mcp_server_definitions
 WHERE company_id=NEW.company_id AND id=runtime_record.capability_id
   AND descriptor_digest=runtime_record.version_digest FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'MCP runtime event refers to a stale capability revision' USING ERRCODE='23514'; END IF;
 SELECT status INTO prior_status FROM mcp_runtime_qualification_events
 WHERE company_id=NEW.company_id AND runtime_qualification_id=NEW.runtime_qualification_id ORDER BY event_seq DESC LIMIT 1;
 IF NEW.status='observed_unqualified' THEN
  IF prior_status IS NOT NULL OR capability_status IS DISTINCT FROM 'approved' THEN RAISE EXCEPTION 'MCP runtime observation requires the current approved descriptor and may be recorded once' USING ERRCODE='23514'; END IF;
 ELSIF NEW.status='qualified' THEN
  IF prior_status IS DISTINCT FROM 'observed_unqualified' OR capability_status IS DISTINCT FROM 'approved' THEN RAISE EXCEPTION 'MCP runtime approval requires an observation for the current approved descriptor' USING ERRCODE='23514'; END IF;
  SELECT count(*) INTO approval_count FROM capability_decisions WHERE company_id=NEW.company_id AND capability_kind='mcp'
    AND capability_id=runtime_record.capability_id AND version_digest=runtime_record.version_digest
    AND qualification_id=runtime_record.capability_qualification_id AND decision='approved';
  IF approval_count=0 THEN RAISE EXCEPTION 'MCP runtime approval requires current metadata approval' USING ERRCODE='23514'; END IF;
 ELSIF NEW.status='schema_drift' THEN
  IF prior_status IS NULL OR prior_status NOT IN ('observed_unqualified','qualified') THEN RAISE EXCEPTION 'MCP schema drift requires an active observed runtime' USING ERRCODE='23514'; END IF;
 ELSIF NEW.status='revoked' THEN
  IF prior_status IS NULL OR prior_status NOT IN ('observed_unqualified','qualified') THEN RAISE EXCEPTION 'MCP runtime revocation requires an active qualification state' USING ERRCODE='23514'; END IF;
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd

-- The previous tool-call trigger is restored by the next down migration in a
-- full rollback; rolling back this schema is blocked once runtime evidence exists.
