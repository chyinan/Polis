-- +goose Up
CREATE TABLE feedback_collection_policy_events (
 company_id text NOT NULL,
 source_id text NOT NULL,
 event_seq bigint NOT NULL,
 event_id text NOT NULL,
 request_id text NOT NULL,
 enabled boolean NOT NULL,
 interval_seconds integer NOT NULL CHECK(interval_seconds BETWEEN 900 AND 86400),
 rationale text NOT NULL CHECK(char_length(rationale) BETWEEN 1 AND 512),
 actor text NOT NULL CHECK(actor='local-owner'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,source_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,source_id) REFERENCES feedback_source_bindings(company_id,source_id)
);
CREATE TRIGGER feedback_collection_policy_events_immutable
BEFORE UPDATE OR DELETE ON feedback_collection_policy_events
FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();

CREATE TABLE feedback_collection_schedule_state (
 company_id text NOT NULL,
 source_id text NOT NULL,
 interval_seconds integer NOT NULL CHECK(interval_seconds BETWEEN 900 AND 86400),
 next_poll_at timestamptz,
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,source_id),
 FOREIGN KEY(company_id,source_id) REFERENCES feedback_source_bindings(company_id,source_id)
);

CREATE TABLE feedback_collection_poll_attempts (
 company_id text NOT NULL,
 source_id text NOT NULL,
 request_id text NOT NULL,
 scheduled_for timestamptz NOT NULL,
 status text NOT NULL CHECK(status IN ('reserved','completed','partial','failed','outcome_unknown')),
 lease_until timestamptz,
 reason_code text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 finished_at timestamptz,
 PRIMARY KEY(company_id,source_id,request_id),
 UNIQUE(company_id,source_id,scheduled_for),
 FOREIGN KEY(company_id,source_id) REFERENCES feedback_source_bindings(company_id,source_id),
 CHECK((status='reserved' AND lease_until IS NOT NULL AND finished_at IS NULL) OR
       (status<>'reserved' AND lease_until IS NULL AND finished_at IS NOT NULL)),
 CHECK(reason_code='' OR reason_code ~ '^[a-z0-9_-]{1,96}$')
);

-- +goose StatementBegin
CREATE FUNCTION guard_feedback_collection_poll_attempt_transition() RETURNS trigger AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  RAISE EXCEPTION 'scheduled feedback poll attempts cannot be deleted';
 END IF;
 IF NEW.company_id<>OLD.company_id OR NEW.source_id<>OLD.source_id OR NEW.request_id<>OLD.request_id OR NEW.scheduled_for<>OLD.scheduled_for OR NEW.created_at<>OLD.created_at THEN
  RAISE EXCEPTION 'scheduled feedback poll attempt identity is immutable';
 END IF;
 IF OLD.status<>'reserved' OR NEW.status NOT IN ('completed','partial','failed','outcome_unknown') THEN
  RAISE EXCEPTION 'invalid scheduled feedback poll attempt transition';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER feedback_collection_poll_attempt_transition
BEFORE UPDATE OR DELETE ON feedback_collection_poll_attempts
FOR EACH ROW EXECUTE FUNCTION guard_feedback_collection_poll_attempt_transition();
