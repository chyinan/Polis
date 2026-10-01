-- +goose Up
-- Capability revocation snapshots the exact WorkerSession and unresolved MCP
-- intent set under the same company lock that closes future authorization.
CREATE TABLE capability_revocation_sessions (
 company_id text NOT NULL,
 revocation_id text NOT NULL,
 session_id text NOT NULL,
 employee_id text NOT NULL,
 capability_kind text NOT NULL CHECK(capability_kind IN ('skill','mcp')),
 capability_id text NOT NULL,
 version_digest text NOT NULL CHECK(version_digest ~ '^[a-f0-9]{64}$'),
 session_state_at_revocation text NOT NULL CHECK(session_state_at_revocation IN ('restoring','validating','activation_pending_environment','active','stopping','stopped','reconcile_required')),
 inclusion_reason text NOT NULL CHECK(inclusion_reason IN ('live_at_revocation','recorded_use')),
 skill_load_count bigint NOT NULL DEFAULT 0 CHECK(skill_load_count>=0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,revocation_id,session_id),
 FOREIGN KEY(company_id,session_id) REFERENCES worker_sessions(company_id,id),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id),
 CHECK(capability_kind='skill' OR skill_load_count=0)
);
CREATE INDEX capability_revocation_sessions_session ON capability_revocation_sessions(company_id,session_id,revocation_id);
CREATE TRIGGER capability_revocation_sessions_immutable
 BEFORE UPDATE OR DELETE ON capability_revocation_sessions
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER capability_revocation_sessions_no_truncate
 BEFORE TRUNCATE ON capability_revocation_sessions
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE capability_revocation_mcp_calls (
 company_id text NOT NULL,
 revocation_id text NOT NULL,
 session_id text NOT NULL,
 intent_id text NOT NULL,
 status_at_revocation text NOT NULL CHECK(status_at_revocation IN ('dispatching','completed','outcome_unknown')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,revocation_id,intent_id),
 FOREIGN KEY(company_id,revocation_id,session_id) REFERENCES capability_revocation_sessions(company_id,revocation_id,session_id),
 FOREIGN KEY(company_id,intent_id) REFERENCES mcp_tool_call_intents(company_id,intent_id)
);
CREATE INDEX capability_revocation_mcp_calls_session ON capability_revocation_mcp_calls(company_id,revocation_id,session_id,intent_id);
CREATE TRIGGER capability_revocation_mcp_calls_immutable
 BEFORE UPDATE OR DELETE ON capability_revocation_mcp_calls
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER capability_revocation_mcp_calls_no_truncate
 BEFORE TRUNCATE ON capability_revocation_mcp_calls
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- Existing revocations predate these snapshot tables. Their durable Skill/MCP
-- usage events remain the fallback projection source until a new revoke writes
-- an exact snapshot in its authorization transaction.

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM capability_revocation_sessions) OR EXISTS(SELECT 1 FROM capability_revocation_mcp_calls) THEN
  RAISE EXCEPTION 'cannot roll back capability revocation snapshots';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER capability_revocation_mcp_calls_no_truncate ON capability_revocation_mcp_calls;
DROP TRIGGER capability_revocation_mcp_calls_immutable ON capability_revocation_mcp_calls;
DROP INDEX capability_revocation_mcp_calls_session;
DROP TABLE capability_revocation_mcp_calls;
DROP TRIGGER capability_revocation_sessions_no_truncate ON capability_revocation_sessions;
DROP TRIGGER capability_revocation_sessions_immutable ON capability_revocation_sessions;
DROP INDEX capability_revocation_sessions_session;
DROP TABLE capability_revocation_sessions;
