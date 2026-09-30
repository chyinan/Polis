-- +goose Up
ALTER TABLE feedback_collection_poll_attempts
 ADD COLUMN started_at timestamptz;

ALTER TABLE feedback_collection_poll_attempts
 DROP CONSTRAINT feedback_collection_poll_attempts_status_check,
 ADD CONSTRAINT feedback_collection_poll_attempts_status_check
 CHECK(status IN ('reserved','started','completed','partial','failed','outcome_unknown'));

ALTER TABLE feedback_collection_poll_attempts
 DROP CONSTRAINT feedback_collection_poll_attempts_check,
 ADD CONSTRAINT feedback_collection_poll_attempts_check
 CHECK((status='reserved' AND lease_until IS NOT NULL AND finished_at IS NULL AND started_at IS NULL) OR
       (status='started' AND lease_until IS NOT NULL AND finished_at IS NULL AND started_at IS NOT NULL) OR
       (status NOT IN ('reserved','started') AND lease_until IS NULL AND finished_at IS NOT NULL));

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION guard_feedback_collection_poll_attempt_transition() RETURNS trigger AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  RAISE EXCEPTION 'scheduled feedback poll attempts cannot be deleted';
 END IF;
 IF NEW.company_id<>OLD.company_id OR NEW.source_id<>OLD.source_id OR NEW.request_id<>OLD.request_id OR NEW.scheduled_for<>OLD.scheduled_for OR NEW.created_at<>OLD.created_at THEN
  RAISE EXCEPTION 'scheduled feedback poll attempt identity is immutable';
 END IF;
 IF OLD.status='reserved' AND NEW.status='started' AND OLD.started_at IS NULL AND NEW.started_at IS NOT NULL AND OLD.lease_until=NEW.lease_until AND NEW.finished_at IS NULL THEN
  RETURN NEW;
 END IF;
 IF OLD.status IN ('reserved','started') AND NEW.status IN ('completed','partial','failed','outcome_unknown') AND NEW.started_at IS NOT DISTINCT FROM OLD.started_at THEN
  RETURN NEW;
 END IF;
 RAISE EXCEPTION 'invalid scheduled feedback poll attempt transition';
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
