-- +goose Up
-- Research operations are durable control-plane intents. This table stores
-- bounded search/fetch input and an explicit unavailable result; it does not
-- grant an HTTP, search, MCP or browser egress path.
CREATE TABLE research_operations (
 company_id text NOT NULL,
 operation_id text NOT NULL,
 mission_id text NOT NULL,
 task_id text NOT NULL,
 session_id text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('search','fetch')),
 query text,
 target_url text,
 request_sha256 text NOT NULL CHECK(request_sha256 ~ '^[a-f0-9]{64}$'),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,operation_id),
 UNIQUE(company_id,request_id),
 CHECK((kind='search')=(query IS NOT NULL AND target_url IS NULL)),
 CHECK((kind='fetch')=(query IS NULL AND target_url IS NOT NULL)),
 CHECK(query IS NULL OR (octet_length(query) BETWEEN 1 AND 4096 AND query !~ E'[\\r\\n]')),
 CHECK(target_url IS NULL OR (target_url ~ '^https://' AND octet_length(substring(target_url from 9)) > 0 AND substring(target_url from 9) !~ '^[/?:#@]' AND substring(target_url from 9) !~ '[[:space:]]' AND position('@' in target_url)=0 AND position('#' in target_url)=0 AND target_url !~ E'[\\r\\n]')),
 FOREIGN KEY(company_id,task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,session_id,task_id) REFERENCES worker_sessions(company_id,id,task_id)
);
CREATE INDEX research_operations_task_recent ON research_operations(company_id,task_id,created_at DESC,operation_id DESC);
CREATE TABLE research_operation_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL,
 operation_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('unavailable','requested','running','succeeded','failed','unknown')),
 reason_code text NOT NULL CHECK(reason_code ~ '^[a-z0-9_:-]{1,128}$'),
 result jsonb NOT NULL DEFAULT '{}'::jsonb CHECK(jsonb_typeof(result)='object'),
 actor text NOT NULL CHECK(octet_length(btrim(actor)) BETWEEN 1 AND 80),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,operation_id) REFERENCES research_operations(company_id,operation_id)
);
CREATE INDEX research_operation_events_latest ON research_operation_events(company_id,operation_id,event_seq DESC);
CREATE TRIGGER research_operations_immutable
 BEFORE UPDATE OR DELETE ON research_operations
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER research_operations_no_truncate
 BEFORE TRUNCATE ON research_operations FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER research_operation_events_immutable
 BEFORE UPDATE OR DELETE ON research_operation_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER research_operation_events_no_truncate
 BEFORE TRUNCATE ON research_operation_events FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM research_operation_events) OR EXISTS(SELECT 1 FROM research_operations) THEN
  RAISE EXCEPTION 'cannot discard research operation history';
 END IF;
END
$$;
DROP TRIGGER research_operation_events_no_truncate ON research_operation_events;
DROP TRIGGER research_operation_events_immutable ON research_operation_events;
DROP INDEX research_operation_events_latest;
DROP TABLE research_operation_events;
DROP TRIGGER research_operations_no_truncate ON research_operations;
DROP TRIGGER research_operations_immutable ON research_operations;
DROP INDEX research_operations_task_recent;
DROP TABLE research_operations;
