-- +goose Up
-- A call intent is committed before a controlled MCP side effect. Results and
-- unresolved outcomes are append-only so recovery can never infer permission
-- to replay an unknown call.
CREATE TABLE mcp_tool_call_intents (
 company_id text NOT NULL,
 intent_id text NOT NULL,
 session_id text NOT NULL,
 employee_id text NOT NULL,
 capability_id text NOT NULL,
 runtime_qualification_id text NOT NULL,
 provider_call_id text NOT NULL CHECK(octet_length(provider_call_id) BETWEEN 1 AND 128),
 tool_name text NOT NULL CHECK(octet_length(tool_name) BETWEEN 1 AND 128),
 tool_schema_sha256 text NOT NULL CHECK(tool_schema_sha256 ~ '^[a-f0-9]{64}$'),
 arguments_sha256 text NOT NULL CHECK(arguments_sha256 ~ '^[a-f0-9]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,intent_id),
 UNIQUE(company_id,session_id,provider_call_id),
 FOREIGN KEY(company_id,session_id) REFERENCES worker_sessions(company_id,id),
 FOREIGN KEY(company_id,runtime_qualification_id) REFERENCES mcp_runtime_qualification_records(company_id,runtime_qualification_id)
);
CREATE INDEX mcp_tool_call_intents_session ON mcp_tool_call_intents(company_id,session_id,created_at,intent_id);
CREATE TRIGGER mcp_tool_call_intents_immutable
 BEFORE UPDATE OR DELETE ON mcp_tool_call_intents
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER mcp_tool_call_intents_no_truncate
 BEFORE TRUNCATE ON mcp_tool_call_intents
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE mcp_tool_call_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL UNIQUE,
 event_id text NOT NULL,
 intent_id text NOT NULL,
 status text NOT NULL CHECK(status IN ('dispatching','completed','outcome_unknown')),
 result bytea CHECK(result IS NULL OR octet_length(result)<=73728),
 result_sha256 text CHECK(result_sha256 IS NULL OR result_sha256 ~ '^[a-f0-9]{64}$'),
 reason_code text CHECK(reason_code IS NULL OR (reason_code ~ '^[a-z][a-z0-9_]{0,79}$')),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,intent_id) REFERENCES mcp_tool_call_intents(company_id,intent_id),
 CHECK((status='dispatching' AND result IS NULL AND result_sha256 IS NULL AND reason_code IS NULL)
    OR (status='completed' AND result IS NOT NULL AND result_sha256 IS NOT NULL AND reason_code IS NULL)
    OR (status='outcome_unknown' AND result IS NULL AND result_sha256 IS NULL AND reason_code IS NOT NULL))
);
CREATE INDEX mcp_tool_call_events_current ON mcp_tool_call_events(company_id,intent_id,event_seq DESC);
CREATE TRIGGER mcp_tool_call_events_immutable
 BEFORE UPDATE OR DELETE ON mcp_tool_call_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER mcp_tool_call_events_no_truncate
 BEFORE TRUNCATE ON mcp_tool_call_events
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose StatementBegin
CREATE FUNCTION enforce_mcp_tool_call_event() RETURNS trigger LANGUAGE plpgsql AS $$
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
BEGIN
 SELECT i.session_id,i.employee_id,i.capability_id,i.runtime_qualification_id,i.tool_name,i.tool_schema_sha256,
        r.capability_id AS runtime_capability_id,r.tool_schema_sha256 AS runtime_schema,r.tools,
        r.capability_qualification_id,r.version_digest
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
  IF jsonb_typeof(result_document) IS DISTINCT FROM 'object'
     OR result_document->>'toolName' IS DISTINCT FROM intent_record.tool_name
     OR result_document->>'toolSchemaSha256' IS DISTINCT FROM intent_record.tool_schema_sha256
     OR result_document->>'contentBoundary' IS DISTINCT FROM 'untrusted_stdio_mcp_text' THEN
   RAISE EXCEPTION 'MCP tool-call result does not match its approved tool boundary' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER mcp_tool_call_events_validate
 BEFORE INSERT ON mcp_tool_call_events
 FOR EACH ROW EXECUTE FUNCTION enforce_mcp_tool_call_event();

-- +goose Down
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM mcp_tool_call_events) OR EXISTS(SELECT 1 FROM mcp_tool_call_intents) THEN
  RAISE EXCEPTION 'cannot roll back MCP tool-call evidence';
 END IF;
END $$;
DROP TRIGGER mcp_tool_call_events_validate ON mcp_tool_call_events;
DROP FUNCTION enforce_mcp_tool_call_event();
DROP TRIGGER mcp_tool_call_events_no_truncate ON mcp_tool_call_events;
DROP TRIGGER mcp_tool_call_events_immutable ON mcp_tool_call_events;
DROP INDEX mcp_tool_call_events_current;
DROP TABLE mcp_tool_call_events;
DROP TRIGGER mcp_tool_call_intents_no_truncate ON mcp_tool_call_intents;
DROP TRIGGER mcp_tool_call_intents_immutable ON mcp_tool_call_intents;
DROP INDEX mcp_tool_call_intents_session;
DROP TABLE mcp_tool_call_intents;
