-- +goose Up
-- Controlled MCP tool requests are admitted as short-lived one-shot permits.
-- The immutable call intent is created only when the permit is consumed, so a
-- revoked or expired permit never appears as an in-flight external call.
CREATE TABLE mcp_tool_dispatch_permits (
 company_id text NOT NULL,
 permit_id text NOT NULL,
 action_id text NOT NULL CHECK(octet_length(action_id) BETWEEN 1 AND 128),
 attempt_id text NOT NULL CHECK(octet_length(attempt_id) BETWEEN 1 AND 128),
 session_id text NOT NULL,
 employee_id text NOT NULL,
 task_id text NOT NULL,
 worker_generation bigint NOT NULL CHECK(worker_generation > 0),
 employee_epoch bigint NOT NULL CHECK(employee_epoch > 0),
 grant_revision bigint NOT NULL CHECK(grant_revision > 0),
 capability_id text NOT NULL,
 capability_version_digest text NOT NULL CHECK(capability_version_digest ~ '^[a-f0-9]{64}$'),
 capability_qualification_id text NOT NULL,
 runtime_qualification_id text NOT NULL,
 transport text NOT NULL CHECK(transport IN ('stdio','streamable_http')),
 provider_call_id text NOT NULL CHECK(octet_length(provider_call_id) BETWEEN 1 AND 128),
 tool_name text NOT NULL CHECK(octet_length(tool_name) BETWEEN 1 AND 128),
 tool_schema_sha256 text NOT NULL CHECK(tool_schema_sha256 ~ '^[a-f0-9]{64}$'),
 target_sha256 text NOT NULL CHECK(target_sha256 ~ '^[a-f0-9]{64}$'),
 input_sha256 text NOT NULL CHECK(input_sha256 ~ '^[a-f0-9]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 expires_at timestamptz NOT NULL,
 PRIMARY KEY(company_id,permit_id),
 UNIQUE(company_id,action_id),
 UNIQUE(company_id,attempt_id),
 UNIQUE(company_id,session_id,provider_call_id),
 FOREIGN KEY(company_id,session_id) REFERENCES worker_sessions(company_id,id),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,runtime_qualification_id) REFERENCES mcp_runtime_qualification_records(company_id,runtime_qualification_id),
 CHECK(expires_at > created_at AND expires_at <= created_at + interval '30 seconds')
);
CREATE INDEX mcp_tool_dispatch_permits_session ON mcp_tool_dispatch_permits(company_id,session_id,created_at,permit_id);
CREATE TRIGGER mcp_tool_dispatch_permits_immutable
 BEFORE UPDATE OR DELETE ON mcp_tool_dispatch_permits
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER mcp_tool_dispatch_permits_no_truncate
 BEFORE TRUNCATE ON mcp_tool_dispatch_permits
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE mcp_tool_dispatch_permit_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL UNIQUE,
 event_id text NOT NULL,
 permit_id text NOT NULL,
 status text NOT NULL CHECK(status IN ('issued','consumed','expired')),
 reason_code text CHECK(reason_code IS NULL OR reason_code IN ('permit_expired','worker_stopped_before_dispatch')),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,permit_id) REFERENCES mcp_tool_dispatch_permits(company_id,permit_id),
 CHECK((status IN ('issued','consumed') AND reason_code IS NULL) OR (status='expired' AND reason_code IS NOT NULL))
);
CREATE INDEX mcp_tool_dispatch_permit_events_current ON mcp_tool_dispatch_permit_events(company_id,permit_id,event_seq DESC);
CREATE TRIGGER mcp_tool_dispatch_permit_events_immutable
 BEFORE UPDATE OR DELETE ON mcp_tool_dispatch_permit_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER mcp_tool_dispatch_permit_events_no_truncate
 BEFORE TRUNCATE ON mcp_tool_dispatch_permit_events
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

ALTER TABLE mcp_tool_call_intents
 ADD COLUMN dispatch_permit_id text,
 ADD COLUMN attempt_id text,
 ADD COLUMN task_id text,
 ADD COLUMN worker_generation bigint,
 ADD COLUMN employee_epoch bigint,
 ADD COLUMN grant_revision bigint,
 ADD COLUMN target_sha256 text,
 ADD COLUMN input_sha256 text;
ALTER TABLE mcp_tool_call_intents
 ADD CONSTRAINT mcp_tool_call_intents_dispatch_permit_fk
 FOREIGN KEY(company_id,dispatch_permit_id) REFERENCES mcp_tool_dispatch_permits(company_id,permit_id),
 ADD CONSTRAINT mcp_tool_call_intents_permit_binding_check CHECK(
  (dispatch_permit_id IS NULL AND attempt_id IS NULL AND task_id IS NULL AND worker_generation IS NULL AND employee_epoch IS NULL AND grant_revision IS NULL AND target_sha256 IS NULL AND input_sha256 IS NULL)
  OR
  (dispatch_permit_id IS NOT NULL AND attempt_id IS NOT NULL AND task_id IS NOT NULL AND worker_generation IS NOT NULL AND worker_generation > 0
   AND employee_epoch IS NOT NULL AND employee_epoch > 0
   AND grant_revision IS NOT NULL AND grant_revision > 0 AND target_sha256 IS NOT NULL AND target_sha256 ~ '^[a-f0-9]{64}$'
   AND input_sha256 IS NOT NULL AND input_sha256=arguments_sha256)
 );

-- +goose StatementBegin
CREATE FUNCTION enforce_mcp_tool_call_intent_permit_binding() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 permit_record RECORD;
 permit_status text;
BEGIN
 IF NEW.dispatch_permit_id IS NULL THEN
  RAISE EXCEPTION 'new MCP tool-call intent requires a consumed dispatch permit' USING ERRCODE='23514';
 END IF;
 SELECT p.action_id,p.attempt_id,p.session_id,p.employee_id,p.task_id,p.worker_generation,p.employee_epoch,p.grant_revision,
        p.capability_id,p.capability_version_digest,p.capability_qualification_id,p.runtime_qualification_id,
        p.provider_call_id,p.tool_name,p.tool_schema_sha256,p.target_sha256,p.input_sha256
 INTO permit_record
 FROM mcp_tool_dispatch_permits p
 WHERE p.company_id=NEW.company_id AND p.permit_id=NEW.dispatch_permit_id;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'MCP tool-call intent has no dispatch permit' USING ERRCODE='23503';
 END IF;
 IF permit_record.action_id IS DISTINCT FROM NEW.intent_id OR permit_record.attempt_id IS DISTINCT FROM NEW.attempt_id
    OR permit_record.session_id IS DISTINCT FROM NEW.session_id OR permit_record.employee_id IS DISTINCT FROM NEW.employee_id
    OR permit_record.task_id IS DISTINCT FROM NEW.task_id OR permit_record.worker_generation IS DISTINCT FROM NEW.worker_generation
    OR permit_record.employee_epoch IS DISTINCT FROM NEW.employee_epoch OR permit_record.grant_revision IS DISTINCT FROM NEW.grant_revision
    OR permit_record.capability_id IS DISTINCT FROM NEW.capability_id OR permit_record.runtime_qualification_id IS DISTINCT FROM NEW.runtime_qualification_id
    OR permit_record.provider_call_id IS DISTINCT FROM NEW.provider_call_id OR permit_record.tool_name IS DISTINCT FROM NEW.tool_name
    OR permit_record.tool_schema_sha256 IS DISTINCT FROM NEW.tool_schema_sha256 OR permit_record.target_sha256 IS DISTINCT FROM NEW.target_sha256
    OR permit_record.input_sha256 IS DISTINCT FROM NEW.input_sha256 OR NEW.input_sha256 IS DISTINCT FROM NEW.arguments_sha256 THEN
  RAISE EXCEPTION 'MCP tool-call intent differs from its immutable dispatch permit' USING ERRCODE='23514';
 END IF;
 SELECT e.status INTO permit_status FROM mcp_tool_dispatch_permit_events e
 WHERE e.company_id=NEW.company_id AND e.permit_id=NEW.dispatch_permit_id ORDER BY e.event_seq DESC LIMIT 1;
 IF permit_status IS DISTINCT FROM 'consumed' THEN
  RAISE EXCEPTION 'MCP tool-call intent requires a consumed dispatch permit' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER mcp_tool_call_intents_permit_binding
 BEFORE INSERT ON mcp_tool_call_intents
 FOR EACH ROW EXECUTE FUNCTION enforce_mcp_tool_call_intent_permit_binding();

-- +goose StatementBegin
CREATE FUNCTION enforce_mcp_tool_dispatch_permit_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 p RECORD;
 prior_status text;
 session_state text;
 session_epoch bigint;
 session_employee text;
 session_task text;
 session_generation bigint;
 employee_epoch bigint;
 binding_event text;
 binding_version text;
 binding_qualification text;
 binding_revision bigint;
 current_version text;
 current_status text;
 qualification_status text;
 latest_decision text;
 runtime_status text;
BEGIN
 SELECT * INTO p FROM mcp_tool_dispatch_permits
 WHERE company_id=NEW.company_id AND permit_id=NEW.permit_id FOR UPDATE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'dispatch permit event has no immutable permit' USING ERRCODE='23503';
 END IF;
 SELECT status INTO prior_status FROM mcp_tool_dispatch_permit_events
 WHERE company_id=NEW.company_id AND permit_id=NEW.permit_id ORDER BY event_seq DESC LIMIT 1;
 IF NEW.status='issued' THEN
  IF prior_status IS NOT NULL OR p.created_at>clock_timestamp() OR p.expires_at<=clock_timestamp() THEN
   RAISE EXCEPTION 'dispatch permit can be issued only once with a future expiry' USING ERRCODE='23514';
  END IF;
 ELSIF NEW.status='consumed' THEN
  IF prior_status IS DISTINCT FROM 'issued' OR p.expires_at<=clock_timestamp() THEN
   RAISE EXCEPTION 'dispatch permit is not available for consumption' USING ERRCODE='23514';
  END IF;
 ELSIF NEW.status='expired' THEN
  IF prior_status IS DISTINCT FROM 'issued' THEN
   RAISE EXCEPTION 'only an unconsumed dispatch permit can expire' USING ERRCODE='23514';
  END IF;
 ELSE
  RAISE EXCEPTION 'unsupported dispatch permit state' USING ERRCODE='23514';
 END IF;

 SELECT s.state,s.epoch,s.employee_id,s.task_id,s.generation,e.epoch
 INTO session_state,session_epoch,session_employee,session_task,session_generation,employee_epoch
 FROM worker_sessions s JOIN employees e ON e.company_id=s.company_id AND e.id=s.employee_id
 WHERE s.company_id=NEW.company_id AND s.id=p.session_id;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'dispatch permit has no current WorkerSession' USING ERRCODE='23514';
 END IF;
 IF NEW.status='issued' OR NEW.status='consumed' THEN
  IF session_state IS DISTINCT FROM 'active' OR session_epoch IS DISTINCT FROM p.employee_epoch OR employee_epoch IS DISTINCT FROM p.employee_epoch
     OR session_employee IS DISTINCT FROM p.employee_id OR session_task IS DISTINCT FROM p.task_id OR session_generation IS DISTINCT FROM p.worker_generation THEN
   RAISE EXCEPTION 'dispatch permit session or Employee epoch is stale' USING ERRCODE='23514';
  END IF;
 ELSE
  IF p.expires_at>clock_timestamp() AND session_state NOT IN ('stopped','reconcile_required') THEN
   RAISE EXCEPTION 'dispatch permit cannot expire while it remains live' USING ERRCODE='23514';
  END IF;
 END IF;

 IF NEW.status='issued' OR NEW.status='consumed' THEN
  SELECT event,version_digest,qualification_id,event_seq
   INTO binding_event,binding_version,binding_qualification,binding_revision
  FROM employee_capability_events
  WHERE company_id=NEW.company_id AND employee_id=p.employee_id AND capability_kind='mcp' AND capability_id=p.capability_id
  ORDER BY event_seq DESC LIMIT 1;
  SELECT descriptor_digest,status INTO current_version,current_status
  FROM mcp_server_definitions WHERE company_id=NEW.company_id AND id=p.capability_id;
  SELECT decision INTO latest_decision FROM capability_decisions
  WHERE company_id=NEW.company_id AND capability_kind='mcp' AND capability_id=p.capability_id
    AND version_digest=p.capability_version_digest AND qualification_id=p.capability_qualification_id
  ORDER BY created_at DESC,decision_id DESC LIMIT 1;
  SELECT status INTO qualification_status FROM capability_qualification_records
  WHERE company_id=NEW.company_id AND qualification_id=p.capability_qualification_id;
  SELECT e.status INTO runtime_status FROM mcp_runtime_qualification_events e
  WHERE e.company_id=NEW.company_id AND e.runtime_qualification_id=p.runtime_qualification_id
  ORDER BY e.event_seq DESC LIMIT 1;
  IF binding_event IS DISTINCT FROM 'bound' OR binding_version IS DISTINCT FROM p.capability_version_digest
     OR binding_qualification IS DISTINCT FROM p.capability_qualification_id OR binding_revision IS DISTINCT FROM p.grant_revision
     OR current_version IS DISTINCT FROM p.capability_version_digest OR current_status IS DISTINCT FROM 'approved'
     OR latest_decision IS DISTINCT FROM 'approved' OR qualification_status IS DISTINCT FROM 'metadata_verified'
     OR runtime_status IS DISTINCT FROM 'qualified' THEN
   RAISE EXCEPTION 'dispatch permit authorization revision is stale or revoked' USING ERRCODE='23514';
  END IF;
 END IF;
 IF NEW.status='expired' AND ((NEW.reason_code='permit_expired' AND p.expires_at>clock_timestamp())
    OR (NEW.reason_code='worker_stopped_before_dispatch' AND session_state NOT IN ('stopped','reconcile_required'))) THEN
  RAISE EXCEPTION 'dispatch permit expiry reason does not match its lifecycle' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER mcp_tool_dispatch_permit_events_validate
 BEFORE INSERT ON mcp_tool_dispatch_permit_events
 FOR EACH ROW EXECUTE FUNCTION enforce_mcp_tool_dispatch_permit_event();

-- Every newly admitted external MCP call is linked to a permit that was
-- consumed in the same transaction as the call intent. Legacy rows remain
-- immutable evidence but cannot be used to create new calls.
-- +goose StatementBegin
CREATE FUNCTION enforce_mcp_tool_call_dispatch_permit() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 p RECORD;
 current_permit_status text;
 session_epoch bigint;
 employee_epoch bigint;
 current_grant_revision bigint;
 current_grant_event text;
BEGIN
 IF NEW.status<>'dispatching' THEN
  RETURN NEW;
 END IF;
 SELECT i.dispatch_permit_id AS intent_permit_id,i.attempt_id AS intent_attempt_id,i.task_id AS intent_task_id,
        i.worker_generation AS intent_generation,i.employee_epoch AS intent_epoch,i.grant_revision AS intent_grant_revision,
        i.target_sha256 AS intent_target_sha256,i.input_sha256 AS intent_input_sha256,
        p.action_id AS permit_action_id,p.attempt_id AS permit_attempt_id,p.task_id AS permit_task_id,
        p.worker_generation AS permit_generation,p.employee_epoch AS permit_epoch,p.grant_revision AS permit_grant_revision,
        p.target_sha256 AS permit_target_sha256,p.input_sha256 AS permit_input_sha256,p.session_id,p.employee_id
 INTO p
 FROM mcp_tool_call_intents i
 JOIN mcp_tool_dispatch_permits p ON p.company_id=i.company_id AND p.permit_id=i.dispatch_permit_id
 WHERE i.company_id=NEW.company_id AND i.intent_id=NEW.intent_id;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'MCP dispatch event has no permit-bound intent' USING ERRCODE='23503';
 END IF;
 IF p.intent_permit_id IS NULL OR p.permit_action_id IS DISTINCT FROM NEW.intent_id
    OR p.intent_attempt_id IS DISTINCT FROM p.permit_attempt_id OR p.intent_epoch IS DISTINCT FROM p.permit_epoch
    OR p.intent_task_id IS DISTINCT FROM p.permit_task_id OR p.intent_generation IS DISTINCT FROM p.permit_generation
    OR p.intent_grant_revision IS DISTINCT FROM p.permit_grant_revision OR p.intent_target_sha256 IS DISTINCT FROM p.permit_target_sha256
    OR p.intent_input_sha256 IS DISTINCT FROM p.permit_input_sha256 THEN
  RAISE EXCEPTION 'MCP dispatch event is not bound to a matching one-shot permit' USING ERRCODE='23514';
 END IF;
 SELECT e.status INTO current_permit_status FROM mcp_tool_dispatch_permit_events e
 WHERE e.company_id=NEW.company_id AND e.permit_id=p.intent_permit_id ORDER BY e.event_seq DESC LIMIT 1;
 IF current_permit_status IS DISTINCT FROM 'consumed' THEN
  RAISE EXCEPTION 'MCP dispatch permit was not consumed' USING ERRCODE='23514';
 END IF;
 SELECT s.epoch,e.epoch INTO session_epoch,employee_epoch FROM worker_sessions s
 JOIN employees e ON e.company_id=s.company_id AND e.id=s.employee_id
 WHERE s.company_id=NEW.company_id AND s.id=p.session_id AND s.state='active' AND s.employee_id=p.employee_id;
 SELECT event,event_seq INTO current_grant_event,current_grant_revision FROM employee_capability_events
 WHERE company_id=NEW.company_id AND employee_id=p.employee_id AND capability_kind='mcp'
   AND capability_id=(SELECT capability_id FROM mcp_tool_call_intents WHERE company_id=NEW.company_id AND intent_id=NEW.intent_id)
 ORDER BY event_seq DESC LIMIT 1;
 IF session_epoch IS DISTINCT FROM p.employee_epoch OR employee_epoch IS DISTINCT FROM p.employee_epoch
    OR current_grant_event IS DISTINCT FROM 'bound' OR current_grant_revision IS DISTINCT FROM p.grant_revision THEN
  RAISE EXCEPTION 'MCP dispatch permit Employee epoch or grant revision is stale' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER mcp_tool_call_events_permit_guard
 BEFORE INSERT ON mcp_tool_call_events
 FOR EACH ROW EXECUTE FUNCTION enforce_mcp_tool_call_dispatch_permit();

-- +goose Down
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM mcp_tool_dispatch_permits) OR EXISTS(SELECT 1 FROM mcp_tool_dispatch_permit_events)
    OR EXISTS(SELECT 1 FROM mcp_tool_call_intents WHERE dispatch_permit_id IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot roll back MCP dispatch permit evidence';
 END IF;
END $$;
DROP TRIGGER mcp_tool_call_events_permit_guard ON mcp_tool_call_events;
DROP FUNCTION enforce_mcp_tool_call_dispatch_permit();
DROP TRIGGER mcp_tool_dispatch_permit_events_validate ON mcp_tool_dispatch_permit_events;
DROP FUNCTION enforce_mcp_tool_dispatch_permit_event();
DROP TRIGGER mcp_tool_call_intents_permit_binding ON mcp_tool_call_intents;
DROP FUNCTION enforce_mcp_tool_call_intent_permit_binding();
ALTER TABLE mcp_tool_call_intents DROP CONSTRAINT mcp_tool_call_intents_permit_binding_check;
ALTER TABLE mcp_tool_call_intents DROP CONSTRAINT mcp_tool_call_intents_dispatch_permit_fk;
ALTER TABLE mcp_tool_call_intents DROP COLUMN dispatch_permit_id,DROP COLUMN attempt_id,DROP COLUMN task_id,DROP COLUMN worker_generation,DROP COLUMN employee_epoch,
 DROP COLUMN grant_revision,DROP COLUMN target_sha256,DROP COLUMN input_sha256;
DROP TRIGGER mcp_tool_dispatch_permit_events_no_truncate ON mcp_tool_dispatch_permit_events;
DROP TRIGGER mcp_tool_dispatch_permit_events_immutable ON mcp_tool_dispatch_permit_events;
DROP INDEX mcp_tool_dispatch_permit_events_current;
DROP TABLE mcp_tool_dispatch_permit_events;
DROP TRIGGER mcp_tool_dispatch_permits_no_truncate ON mcp_tool_dispatch_permits;
DROP TRIGGER mcp_tool_dispatch_permits_immutable ON mcp_tool_dispatch_permits;
DROP INDEX mcp_tool_dispatch_permits_session;
DROP TABLE mcp_tool_dispatch_permits;
