-- +goose Up
CREATE TABLE task_input_delivery_attempts (
 company_id text NOT NULL,
 delivery_id text NOT NULL,
 task_id text NOT NULL,
 session_id text NOT NULL,
 phase text NOT NULL CHECK(phase IN ('prepared','final')),
 outcome text NOT NULL CHECK(outcome IN ('prepared','provider_delivered','local_context_loaded','outcome_unknown','not_sent')),
 manifest_digest text NOT NULL CHECK(manifest_digest ~ '^[a-f0-9]{64}$'),
 payload_digest text NOT NULL CHECK(payload_digest ~ '^[a-f0-9]{64}$'),
 input_refs jsonb NOT NULL CHECK(jsonb_typeof(input_refs)='array'),
 input_exclusions jsonb NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(input_exclusions)='array'),
 provider_egress integer NOT NULL DEFAULT 0 CHECK(provider_egress>=0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,delivery_id,phase),
 UNIQUE(company_id,session_id,phase),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,session_id) REFERENCES worker_sessions(company_id,id),
 CHECK(
  (phase='prepared' AND outcome='prepared' AND provider_egress=0)
  OR (phase='final' AND outcome IN ('provider_delivered','local_context_loaded','outcome_unknown','not_sent')
      AND ((outcome IN ('provider_delivered','outcome_unknown') AND provider_egress>0)
        OR (outcome IN ('local_context_loaded','not_sent') AND provider_egress=0)))
 )
);
CREATE INDEX task_input_delivery_attempts_task ON task_input_delivery_attempts(company_id,task_id,created_at DESC);
CREATE TRIGGER task_input_delivery_attempts_immutable
 BEFORE UPDATE OR DELETE ON task_input_delivery_attempts
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DROP TRIGGER task_input_delivery_attempts_immutable ON task_input_delivery_attempts;
DROP INDEX task_input_delivery_attempts_task;
DROP TABLE task_input_delivery_attempts;
