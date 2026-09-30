-- +goose Up
ALTER TABLE mcp_runtime_qualification_records
 ADD CONSTRAINT mcp_runtime_qualification_descriptor_matches_version
  CHECK(descriptor_digest=version_digest),
 ADD CONSTRAINT mcp_runtime_qualification_process_spec_matches_digests
  CHECK(COALESCE(process_spec->>'CommandSHA256','')=command_sha256
    AND COALESCE(process_spec->>'PackageManifestSHA256','')=package_manifest_sha256
    AND COALESCE(process_spec->>'ApprovedToolSchemaSHA256','')=tool_schema_sha256
    AND COALESCE(process_spec->'Launch'->>'NetworkPolicy','')='deny_all'
    AND COALESCE(process_spec->'Launch'->>'RegistryProxyEndpoint','')='');

ALTER TABLE mcp_runtime_qualification_events
 ADD CONSTRAINT mcp_runtime_qualification_event_rationale_bound
  CHECK(octet_length(rationale)<=512),
 ADD CONSTRAINT mcp_runtime_qualification_event_drift_shape
  CHECK((status='schema_drift' AND observed_tool_schema_sha256 IS NOT NULL AND observed_tool_schema_sha256<>tool_schema_sha256)
     OR (status<>'schema_drift' AND observed_tool_schema_sha256 IS NULL));

-- +goose StatementBegin
CREATE FUNCTION enforce_mcp_runtime_qualification_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 runtime_record RECORD;
 prior_status text;
 capability_status text;
 approval_count integer;
BEGIN
 SELECT capability_id,version_digest,capability_qualification_id,tool_schema_sha256
 INTO runtime_record
 FROM mcp_runtime_qualification_records
 WHERE company_id=NEW.company_id AND runtime_qualification_id=NEW.runtime_qualification_id
 FOR SHARE;
 IF NOT FOUND OR NEW.capability_id IS DISTINCT FROM runtime_record.capability_id
    OR NEW.version_digest IS DISTINCT FROM runtime_record.version_digest
    OR NEW.tool_schema_sha256 IS DISTINCT FROM runtime_record.tool_schema_sha256 THEN
  RAISE EXCEPTION 'MCP runtime event differs from its immutable qualification record' USING ERRCODE='23514';
 END IF;

 SELECT status INTO capability_status FROM mcp_server_definitions
 WHERE company_id=NEW.company_id AND id=runtime_record.capability_id
   AND descriptor_digest=runtime_record.version_digest FOR SHARE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'MCP runtime event refers to a stale capability revision' USING ERRCODE='23514';
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

CREATE TRIGGER mcp_runtime_qualification_events_validate
 BEFORE INSERT ON mcp_runtime_qualification_events
 FOR EACH ROW EXECUTE FUNCTION enforce_mcp_runtime_qualification_event();

-- +goose Down
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM mcp_runtime_qualification_events) OR EXISTS(SELECT 1 FROM mcp_runtime_qualification_records) THEN
  RAISE EXCEPTION 'cannot roll back MCP runtime qualification evidence';
 END IF;
END $$;
DROP TRIGGER mcp_runtime_qualification_events_validate ON mcp_runtime_qualification_events;
DROP FUNCTION enforce_mcp_runtime_qualification_event();
ALTER TABLE mcp_runtime_qualification_events
 DROP CONSTRAINT mcp_runtime_qualification_event_drift_shape,
 DROP CONSTRAINT mcp_runtime_qualification_event_rationale_bound;
ALTER TABLE mcp_runtime_qualification_records
 DROP CONSTRAINT mcp_runtime_qualification_process_spec_matches_digests,
 DROP CONSTRAINT mcp_runtime_qualification_descriptor_matches_version;
