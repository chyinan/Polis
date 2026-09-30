-- +goose Up
-- Persist the exact local stdio MCP command/package/schema observation.
-- Observations are not executable grants until a separate owner decision is
-- appended and the caller's current Employee binding is rechecked.
CREATE TABLE mcp_runtime_qualification_records (
 company_id text NOT NULL,
 runtime_qualification_id text NOT NULL,
 capability_id text NOT NULL,
 capability_qualification_id text NOT NULL,
 version_digest text NOT NULL CHECK(version_digest ~ '^[a-f0-9]{64}$'),
 descriptor_digest text NOT NULL CHECK(descriptor_digest ~ '^[a-f0-9]{64}$'),
 runtime_profile text NOT NULL CHECK(runtime_profile='polis-controlled-stdio-mcp-2026-07-28@1'),
 host_os text NOT NULL CHECK(host_os='windows'),
 host_profile text NOT NULL CHECK(host_profile='windows_appcontainer_deny_all@1'),
 command_sha256 text NOT NULL CHECK(command_sha256 ~ '^[a-f0-9]{64}$'),
 package_manifest_sha256 text NOT NULL CHECK(package_manifest_sha256 ~ '^[a-f0-9]{64}$'),
 server_name text NOT NULL CHECK(octet_length(server_name) BETWEEN 1 AND 128),
 server_version text NOT NULL CHECK(octet_length(server_version) BETWEEN 1 AND 128),
 protocol_version text NOT NULL CHECK(protocol_version='2026-07-28'),
 tool_schema_sha256 text NOT NULL CHECK(tool_schema_sha256 ~ '^[a-f0-9]{64}$'),
 tools jsonb NOT NULL CHECK(jsonb_typeof(tools)='array' AND octet_length(tools::text)<=262144),
 process_spec jsonb NOT NULL CHECK(jsonb_typeof(process_spec)='object' AND octet_length(process_spec::text)<=262144),
 evidence_digest text NOT NULL CHECK(evidence_digest ~ '^[a-f0-9]{64}$'),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,runtime_qualification_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,capability_id) REFERENCES mcp_server_definitions(company_id,id),
 FOREIGN KEY(company_id,capability_qualification_id) REFERENCES capability_qualification_records(company_id,qualification_id)
);
CREATE INDEX mcp_runtime_qualification_current ON mcp_runtime_qualification_records(company_id,capability_id,version_digest,created_at DESC);
CREATE TRIGGER mcp_runtime_qualification_records_immutable
 BEFORE UPDATE OR DELETE ON mcp_runtime_qualification_records
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER mcp_runtime_qualification_records_no_truncate
 BEFORE TRUNCATE ON mcp_runtime_qualification_records
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE mcp_runtime_qualification_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL UNIQUE,
 event_id text NOT NULL,
 runtime_qualification_id text NOT NULL,
 capability_id text NOT NULL,
 version_digest text NOT NULL CHECK(version_digest ~ '^[a-f0-9]{64}$'),
 status text NOT NULL CHECK(status IN ('observed_unqualified','qualified','schema_drift','revoked')),
 tool_schema_sha256 text NOT NULL CHECK(tool_schema_sha256 ~ '^[a-f0-9]{64}$'),
 observed_tool_schema_sha256 text CHECK(observed_tool_schema_sha256 IS NULL OR observed_tool_schema_sha256 ~ '^[a-f0-9]{64}$'),
 rationale text NOT NULL CHECK(octet_length(rationale)<=512),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,runtime_qualification_id) REFERENCES mcp_runtime_qualification_records(company_id,runtime_qualification_id)
);
CREATE INDEX mcp_runtime_qualification_events_current ON mcp_runtime_qualification_events(company_id,runtime_qualification_id,event_seq DESC);
CREATE TRIGGER mcp_runtime_qualification_events_immutable
 BEFORE UPDATE OR DELETE ON mcp_runtime_qualification_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER mcp_runtime_qualification_events_no_truncate
 BEFORE TRUNCATE ON mcp_runtime_qualification_events
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM mcp_runtime_qualification_events) OR EXISTS(SELECT 1 FROM mcp_runtime_qualification_records) THEN
  RAISE EXCEPTION 'cannot roll back MCP runtime qualification evidence';
 END IF;
END $$;
DROP TRIGGER mcp_runtime_qualification_events_no_truncate ON mcp_runtime_qualification_events;
DROP TRIGGER mcp_runtime_qualification_events_immutable ON mcp_runtime_qualification_events;
DROP INDEX mcp_runtime_qualification_events_current;
DROP TABLE mcp_runtime_qualification_events;
DROP TRIGGER mcp_runtime_qualification_records_no_truncate ON mcp_runtime_qualification_records;
DROP TRIGGER mcp_runtime_qualification_records_immutable ON mcp_runtime_qualification_records;
DROP INDEX mcp_runtime_qualification_current;
DROP TABLE mcp_runtime_qualification_records;
