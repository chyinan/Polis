-- +goose Up
-- A terminal Mission must not be resurrected when a user requests delivery
-- changes. This append-only Company backlog records the exact immutable
-- delivery/disposition that needs a separately opened Mission.
CREATE TABLE delivery_feedback_backlog_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL CHECK(event_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 delivery_id text NOT NULL,
 manifest_revision bigint NOT NULL CHECK(manifest_revision > 0),
 disposition_revision bigint NOT NULL CHECK(disposition_revision > 0),
 mission_id text NOT NULL,
 task_id text NOT NULL,
 artifact_id text NOT NULL,
 status text NOT NULL DEFAULT 'open' CHECK(status IN ('open','handled','archived')),
 reason text NOT NULL CHECK(octet_length(btrim(reason)) BETWEEN 1 AND 4096),
 actor text NOT NULL CHECK(actor='system'),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,delivery_id,manifest_revision)
  REFERENCES delivery_manifest_revisions(company_id,delivery_id,revision),
 FOREIGN KEY(company_id,delivery_id,manifest_revision,disposition_revision)
  REFERENCES delivery_user_dispositions(company_id,delivery_id,manifest_revision,revision),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,artifact_id,task_id) REFERENCES artifacts(company_id,id,task_id),
 CHECK(delivery_id=artifact_id)
);
CREATE INDEX delivery_feedback_backlog_latest
 ON delivery_feedback_backlog_events(company_id,status,created_at DESC,event_seq DESC);
CREATE TRIGGER delivery_feedback_backlog_events_immutable
 BEFORE UPDATE OR DELETE ON delivery_feedback_backlog_events FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER delivery_feedback_backlog_events_no_truncate
 BEFORE TRUNCATE ON delivery_feedback_backlog_events FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM delivery_feedback_backlog_events) THEN
  RAISE EXCEPTION 'cannot discard delivery feedback backlog history';
 END IF;
END
$$;
-- +goose StatementEnd
DROP TRIGGER delivery_feedback_backlog_events_no_truncate ON delivery_feedback_backlog_events;
DROP TRIGGER delivery_feedback_backlog_events_immutable ON delivery_feedback_backlog_events;
DROP INDEX delivery_feedback_backlog_latest;
DROP TABLE delivery_feedback_backlog_events;
