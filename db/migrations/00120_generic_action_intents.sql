-- +goose Up
-- Generic ActionIntent is an audit and deny boundary until a concrete
-- resource-specific DispatchPermit policy is implemented. It never executes
-- an external effect by itself.
CREATE TABLE generic_action_intents (
 company_id text NOT NULL,
 intent_id text NOT NULL,
 mission_id text NOT NULL,
 task_id text NOT NULL,
 session_id text NOT NULL,
 action_kind text NOT NULL CHECK(action_kind IN ('shared_write','external_write','notification','provider_request')),
 resource_key text NOT NULL CHECK(octet_length(resource_key) BETWEEN 3 AND 256),
 target_sha256 text NOT NULL CHECK(target_sha256 ~ '^[a-f0-9]{64}$'),
 input_sha256 text NOT NULL CHECK(input_sha256 ~ '^[a-f0-9]{64}$'),
 idempotency_key text NOT NULL CHECK(idempotency_key ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,intent_id),
 UNIQUE(company_id,idempotency_key),
 FOREIGN KEY(company_id,task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,session_id,task_id) REFERENCES worker_sessions(company_id,id,task_id),
 CHECK(resource_key !~ E'[\\r\\n\\t]' AND resource_key !~ '//' AND resource_key ~ '^[a-z0-9_.-]+:[^:]+$')
);
CREATE INDEX generic_action_intents_task_recent ON generic_action_intents(company_id,task_id,created_at DESC,intent_id);
CREATE TABLE generic_action_intent_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL,
 intent_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('denied','authorized','dispatching','completed','failed','outcome_unknown','revoked')),
 reason_code text NOT NULL CHECK(reason_code ~ '^[a-z0-9_:-]{1,128}$'),
 actor text NOT NULL CHECK(octet_length(btrim(actor)) BETWEEN 1 AND 80),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,intent_id) REFERENCES generic_action_intents(company_id,intent_id)
);
CREATE INDEX generic_action_intent_events_latest ON generic_action_intent_events(company_id,intent_id,event_seq DESC);
CREATE TRIGGER generic_action_intents_immutable
 BEFORE UPDATE OR DELETE ON generic_action_intents
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER generic_action_intents_no_truncate
 BEFORE TRUNCATE ON generic_action_intents FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER generic_action_intent_events_immutable
 BEFORE UPDATE OR DELETE ON generic_action_intent_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER generic_action_intent_events_no_truncate
 BEFORE TRUNCATE ON generic_action_intent_events FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM generic_action_intent_events) OR EXISTS(SELECT 1 FROM generic_action_intents) THEN
  RAISE EXCEPTION 'cannot discard generic ActionIntent history';
 END IF;
END
$$;
DROP TRIGGER generic_action_intent_events_no_truncate ON generic_action_intent_events;
DROP TRIGGER generic_action_intent_events_immutable ON generic_action_intent_events;
DROP INDEX generic_action_intent_events_latest;
DROP TABLE generic_action_intent_events;
DROP TRIGGER generic_action_intents_no_truncate ON generic_action_intents;
DROP TRIGGER generic_action_intents_immutable ON generic_action_intents;
DROP INDEX generic_action_intents_task_recent;
DROP TABLE generic_action_intents;
