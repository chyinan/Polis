-- +goose Up
-- Preserve the one-shot permit checks. The SQL alias must not shadow the
-- unassigned PL/pgSQL RECORD used to receive this query's result.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enforce_mcp_tool_call_dispatch_permit() RETURNS trigger LANGUAGE plpgsql AS $$
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
        permit_row.action_id AS permit_action_id,permit_row.attempt_id AS permit_attempt_id,permit_row.task_id AS permit_task_id,
        permit_row.worker_generation AS permit_generation,permit_row.employee_epoch AS permit_epoch,permit_row.grant_revision AS permit_grant_revision,
        permit_row.target_sha256 AS permit_target_sha256,permit_row.input_sha256 AS permit_input_sha256,permit_row.session_id,permit_row.employee_id
 INTO p
 FROM mcp_tool_call_intents i
 JOIN mcp_tool_dispatch_permits permit_row ON permit_row.company_id=i.company_id AND permit_row.permit_id=i.dispatch_permit_id
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
 IF session_epoch IS DISTINCT FROM p.permit_epoch OR employee_epoch IS DISTINCT FROM p.permit_epoch
    OR current_grant_event IS DISTINCT FROM 'bound' OR current_grant_revision IS DISTINCT FROM p.permit_grant_revision THEN
  RAISE EXCEPTION 'MCP dispatch permit Employee epoch or grant revision is stale' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- This compatible function correction introduces no schema/data to remove.
-- Retain the non-shadowing alias rather than deliberately restoring a bug.
SELECT 1;
