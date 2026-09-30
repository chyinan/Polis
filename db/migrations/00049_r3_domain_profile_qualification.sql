-- +goose Up
CREATE TABLE domain_workflow_profile_qualification_events (
 company_id text NOT NULL,
 event_id text NOT NULL,
 profile_id text NOT NULL,
 profile_revision text NOT NULL,
 decision text NOT NULL CHECK(decision IN ('qualified','revoked')),
 evidence_input_id text,
 evidence_input_revision bigint,
 evidence_sha256 text,
 rationale text NOT NULL CHECK(char_length(rationale) BETWEEN 1 AND 2000 AND rationale ~ '[^[:space:]]'),
 actor text NOT NULL DEFAULT 'local-owner' CHECK(actor='local-owner'),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 FOREIGN KEY(company_id,evidence_input_id,evidence_input_revision) REFERENCES mission_inputs(company_id,input_id,revision),
 CHECK (
  (profile_id='content-operations-reference' AND profile_revision='content-operations@1') OR
  (profile_id='research-simulation-reference' AND profile_revision='research-simulation@1')
 ),
 CHECK (
  (decision='qualified' AND evidence_input_id IS NOT NULL AND evidence_input_revision IS NOT NULL AND evidence_sha256 IS NOT NULL AND evidence_sha256 ~ '^[a-f0-9]{64}$') OR
  (decision='revoked' AND evidence_input_id IS NULL AND evidence_input_revision IS NULL AND evidence_sha256 IS NULL)
 )
);

CREATE INDEX domain_workflow_profile_qualification_latest
 ON domain_workflow_profile_qualification_events(company_id,profile_id,created_at DESC,event_id DESC);

-- +goose StatementBegin
CREATE FUNCTION enforce_domain_profile_qualification_event() RETURNS trigger LANGUAGE plpgsql AS $$
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

CREATE TRIGGER domain_workflow_profile_qualification_validate
 BEFORE INSERT ON domain_workflow_profile_qualification_events
 FOR EACH ROW EXECUTE FUNCTION enforce_domain_profile_qualification_event();
CREATE TRIGGER domain_workflow_profile_qualification_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_profile_qualification_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM domain_workflow_profile_qualification_events) THEN
  RAISE EXCEPTION 'cannot roll back R3 domain profile qualification decisions';
 END IF;
END
$$;
-- +goose StatementEnd
DROP TRIGGER domain_workflow_profile_qualification_immutable ON domain_workflow_profile_qualification_events;
DROP TRIGGER domain_workflow_profile_qualification_validate ON domain_workflow_profile_qualification_events;
DROP FUNCTION enforce_domain_profile_qualification_event();
DROP TABLE domain_workflow_profile_qualification_events;
