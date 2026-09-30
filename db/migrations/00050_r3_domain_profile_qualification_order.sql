-- +goose Up
ALTER TABLE domain_workflow_profile_qualification_events
 ADD COLUMN event_seq bigserial NOT NULL;
ALTER TABLE domain_workflow_profile_qualification_events
 ADD CONSTRAINT domain_workflow_profile_qualification_event_seq_key UNIQUE(event_seq);
DROP INDEX domain_workflow_profile_qualification_latest;
CREATE INDEX domain_workflow_profile_qualification_latest
 ON domain_workflow_profile_qualification_events(company_id,profile_id,event_seq DESC);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enforce_domain_profile_qualification_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 input_digest text;
 input_state text;
 previous_decision text;
BEGIN
 PERFORM 1 FROM companies WHERE id=NEW.company_id FOR UPDATE;
 IF NEW.decision='qualified' THEN
  SELECT content_digest,state INTO input_digest,input_state
   FROM mission_inputs
   WHERE company_id=NEW.company_id AND input_id=NEW.evidence_input_id AND revision=NEW.evidence_input_revision
   FOR KEY SHARE;
  IF input_digest IS DISTINCT FROM NEW.evidence_sha256 OR input_state NOT IN ('usable','partial') THEN
   RAISE EXCEPTION 'domain profile qualification report is not a stored company MissionInput' USING ERRCODE='23514';
  END IF;
 END IF;
 SELECT decision INTO previous_decision
  FROM domain_workflow_profile_qualification_events
  WHERE company_id=NEW.company_id AND profile_id=NEW.profile_id
  ORDER BY event_seq DESC LIMIT 1 FOR UPDATE;
 IF NEW.decision='revoked' AND previous_decision IS DISTINCT FROM 'qualified' THEN
  RAISE EXCEPTION 'domain profile qualification is not currently qualified' USING ERRCODE='23514';
 END IF;
 IF NEW.decision='qualified' AND previous_decision='qualified' THEN
  RAISE EXCEPTION 'domain profile qualification is already active' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM domain_workflow_profile_qualification_events) THEN
  RAISE EXCEPTION 'cannot roll back ordered domain profile qualification decisions';
 END IF;
END
$$;
-- +goose StatementEnd
DROP INDEX domain_workflow_profile_qualification_latest;
ALTER TABLE domain_workflow_profile_qualification_events
 DROP CONSTRAINT domain_workflow_profile_qualification_event_seq_key;
ALTER TABLE domain_workflow_profile_qualification_events DROP COLUMN event_seq;
CREATE INDEX domain_workflow_profile_qualification_latest
 ON domain_workflow_profile_qualification_events(company_id,profile_id,created_at DESC,event_id DESC);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enforce_domain_profile_qualification_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 input_digest text;
 input_state text;
 previous_decision text;
BEGIN
 IF NEW.decision='qualified' THEN
  SELECT content_digest,state INTO input_digest,input_state
   FROM mission_inputs
   WHERE company_id=NEW.company_id AND input_id=NEW.evidence_input_id AND revision=NEW.evidence_input_revision
   FOR KEY SHARE;
  IF input_digest IS DISTINCT FROM NEW.evidence_sha256 OR input_state NOT IN ('usable','partial') THEN
   RAISE EXCEPTION 'domain profile qualification report is not a stored company MissionInput' USING ERRCODE='23514';
  END IF;
 END IF;
 SELECT decision INTO previous_decision
  FROM domain_workflow_profile_qualification_events
  WHERE company_id=NEW.company_id AND profile_id=NEW.profile_id
  ORDER BY created_at DESC,event_id DESC LIMIT 1 FOR UPDATE;
 IF NEW.decision='revoked' AND previous_decision IS DISTINCT FROM 'qualified' THEN
  RAISE EXCEPTION 'domain profile qualification is not currently qualified' USING ERRCODE='23514';
 END IF;
 IF NEW.decision='qualified' AND previous_decision='qualified' THEN
  RAISE EXCEPTION 'domain profile qualification is already active' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
