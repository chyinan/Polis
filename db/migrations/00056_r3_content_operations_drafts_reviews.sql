-- +goose Up
CREATE TABLE domain_workflow_content_source_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL,
 input_id text NOT NULL,
 input_revision bigint NOT NULL CHECK(input_revision>0),
 source_sha256 text NOT NULL CHECK(source_sha256 ~ '^[a-f0-9]{64}$'),
 state text NOT NULL CHECK(state IN ('authorized','revoked')),
 rationale text NOT NULL CHECK(char_length(rationale) BETWEEN 1 AND 1000),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 FOREIGN KEY(company_id,input_id,input_revision) REFERENCES mission_inputs(company_id,input_id,revision)
);
CREATE INDEX domain_workflow_content_source_events_latest
 ON domain_workflow_content_source_events(company_id,input_id,input_revision,event_seq DESC);

CREATE TABLE domain_workflow_content_drafts (
 company_id text NOT NULL,
 draft_input_id text NOT NULL,
 draft_revision bigint NOT NULL CHECK(draft_revision>0),
 draft_sha256 text NOT NULL CHECK(draft_sha256 ~ '^[a-f0-9]{64}$'),
 writer_employee_id text NOT NULL,
 critical_claims jsonb NOT NULL CHECK(jsonb_typeof(critical_claims)='array' AND jsonb_array_length(critical_claims) BETWEEN 1 AND 100),
 constraints_passed boolean NOT NULL,
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,draft_input_id,draft_revision),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 FOREIGN KEY(company_id,draft_input_id,draft_revision) REFERENCES mission_inputs(company_id,input_id,revision),
 FOREIGN KEY(company_id,writer_employee_id) REFERENCES employees(company_id,id)
);
CREATE INDEX domain_workflow_content_drafts_recent
 ON domain_workflow_content_drafts(company_id,created_at DESC,draft_input_id,draft_revision DESC);

CREATE TABLE domain_workflow_content_reviews (
 company_id text NOT NULL,
 review_id text NOT NULL,
 draft_input_id text NOT NULL,
 draft_revision bigint NOT NULL CHECK(draft_revision>0),
 draft_sha256 text NOT NULL CHECK(draft_sha256 ~ '^[a-f0-9]{64}$'),
 checker_employee_id text NOT NULL,
 outcome text NOT NULL CHECK(outcome IN ('accepted','inconclusive','rejected')),
 review jsonb NOT NULL CHECK(jsonb_typeof(review)='object'),
 sample jsonb NOT NULL CHECK(jsonb_typeof(sample)='object'),
 reason_codes jsonb NOT NULL CHECK(jsonb_typeof(reason_codes)='array'),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,review_id),
 UNIQUE(company_id,draft_input_id,draft_revision),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,draft_input_id,draft_revision) REFERENCES domain_workflow_content_drafts(company_id,draft_input_id,draft_revision),
 FOREIGN KEY(company_id,checker_employee_id) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);
CREATE INDEX domain_workflow_content_reviews_recent
 ON domain_workflow_content_reviews(company_id,created_at DESC,review_id DESC);

-- +goose StatementBegin
CREATE FUNCTION validate_domain_workflow_content_source_event() RETURNS trigger AS $$
DECLARE
 source_digest text;
 source_state text;
 source_media_type text;
 source_size bigint;
BEGIN
 IF NEW.state='authorized' THEN
  SELECT content_digest,state,media_type,byte_size INTO source_digest,source_state,source_media_type,source_size
  FROM mission_inputs WHERE company_id=NEW.company_id AND input_id=NEW.input_id AND revision=NEW.input_revision;
  IF NOT FOUND OR source_state<>'usable' OR source_digest<>NEW.source_sha256 OR source_size<1 OR source_size>1048576 OR
     NOT (source_media_type='application/json' OR source_media_type LIKE 'text/%') THEN
   RAISE EXCEPTION 'content source is not an exact usable bounded company MissionInput' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER domain_workflow_content_source_valid
 BEFORE INSERT ON domain_workflow_content_source_events
 FOR EACH ROW EXECUTE FUNCTION validate_domain_workflow_content_source_event();
CREATE TRIGGER domain_workflow_content_source_events_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_content_source_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose StatementBegin
CREATE FUNCTION validate_domain_workflow_content_draft() RETURNS trigger AS $$
DECLARE
 body_digest text;
 body_state text;
 body_media_type text;
 body_size bigint;
 claim jsonb;
 claim_id text;
 seen_claims text[] := ARRAY[]::text[];
 writer_exists boolean;
BEGIN
 SELECT content_digest,state,media_type,byte_size INTO body_digest,body_state,body_media_type,body_size
 FROM mission_inputs WHERE company_id=NEW.company_id AND input_id=NEW.draft_input_id AND revision=NEW.draft_revision;
 IF NOT FOUND OR body_state<>'usable' OR body_digest<>NEW.draft_sha256 OR body_size<1 OR body_size>1048576 OR
    NOT (body_media_type='application/json' OR body_media_type LIKE 'text/%') THEN
  RAISE EXCEPTION 'content draft is not an exact usable bounded company MissionInput' USING ERRCODE='23514';
 END IF;
 SELECT EXISTS(SELECT 1 FROM employees WHERE company_id=NEW.company_id AND id=NEW.writer_employee_id) INTO writer_exists;
 IF NOT writer_exists THEN
  RAISE EXCEPTION 'content draft writer is not a company employee' USING ERRCODE='23514';
 END IF;
 FOR claim IN SELECT value FROM jsonb_array_elements(NEW.critical_claims) LOOP
  IF jsonb_typeof(claim)<>'string' THEN
   RAISE EXCEPTION 'content critical claim ID must be a string' USING ERRCODE='23514';
  END IF;
  claim_id := claim #>> '{}';
  IF claim_id !~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$' OR claim_id=ANY(seen_claims) THEN
   RAISE EXCEPTION 'content critical claim IDs must be valid and unique' USING ERRCODE='23514';
  END IF;
  seen_claims := array_append(seen_claims,claim_id);
 END LOOP;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER domain_workflow_content_draft_valid
 BEFORE INSERT ON domain_workflow_content_drafts
 FOR EACH ROW EXECUTE FUNCTION validate_domain_workflow_content_draft();
CREATE TRIGGER domain_workflow_content_drafts_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_content_drafts
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose StatementBegin
CREATE FUNCTION validate_domain_workflow_content_review() RETURNS trigger AS $$
DECLARE
 writer_id text;
 draft_digest text;
 constraints_ok boolean;
 reviewer_role text;
 plan_input_id text;
 plan_revision bigint;
 plan_digest text;
 plan_state text;
 plan_source_digest text;
 claim_review jsonb;
 source jsonb;
 source_input_id text;
 source_revision bigint;
 source_digest text;
 current_source_state text;
 current_source_digest text;
BEGIN
 SELECT writer_employee_id,draft_sha256,constraints_passed INTO writer_id,draft_digest,constraints_ok
 FROM domain_workflow_content_drafts
 WHERE company_id=NEW.company_id AND draft_input_id=NEW.draft_input_id AND draft_revision=NEW.draft_revision;
 IF NOT FOUND OR draft_digest<>NEW.draft_sha256 OR NOT constraints_ok OR writer_id=NEW.checker_employee_id THEN
  RAISE EXCEPTION 'content review draft binding, constraints, or reviewer independence is invalid' USING ERRCODE='23514';
 END IF;
 SELECT role_name INTO reviewer_role FROM employees WHERE company_id=NEW.company_id AND id=NEW.checker_employee_id;
 IF reviewer_role IS DISTINCT FROM 'review' THEN
  RAISE EXCEPTION 'content reviewer must have the review role' USING ERRCODE='23514';
 END IF;
 IF (NEW.review->>'draftRevision')::bigint<>NEW.draft_revision OR NEW.review->>'checkerEmployeeId'<>NEW.checker_employee_id OR
    (NEW.sample->>'draftRevision')::bigint<>NEW.draft_revision OR NEW.sample->>'draftSha256'<>NEW.draft_sha256 OR
    NEW.sample->>'sampledByEmployeeId'<>NEW.checker_employee_id OR
    jsonb_typeof(NEW.sample->'sampledClaimIds')<>'array' OR jsonb_array_length(NEW.sample->'sampledClaimIds')=0 THEN
  RAISE EXCEPTION 'content review and human sample do not bind the same draft/reviewer' USING ERRCODE='23514';
 END IF;
 plan_input_id := NEW.sample->'plan'->>'inputId';
 plan_revision := (NEW.sample->'plan'->>'revision')::bigint;
 plan_digest := NEW.sample->'plan'->>'sha256';
 SELECT state,source_sha256 INTO current_source_state,current_source_digest
 FROM domain_workflow_content_source_events
 WHERE company_id=NEW.company_id AND input_id=plan_input_id AND input_revision=plan_revision
 ORDER BY event_seq DESC LIMIT 1;
 IF current_source_state IS DISTINCT FROM 'authorized' OR current_source_digest IS DISTINCT FROM plan_digest THEN
  RAISE EXCEPTION 'human sampling plan is not currently authorized for this exact company input' USING ERRCODE='23514';
 END IF;
 SELECT state,content_digest INTO plan_state,plan_source_digest FROM mission_inputs
 WHERE company_id=NEW.company_id AND input_id=plan_input_id AND revision=plan_revision;
 IF plan_state IS DISTINCT FROM 'usable' OR plan_source_digest IS DISTINCT FROM plan_digest THEN
  RAISE EXCEPTION 'human sampling plan input changed or is unavailable' USING ERRCODE='23514';
 END IF;
 FOR claim_review IN SELECT value FROM jsonb_array_elements(NEW.review->'claims') LOOP
  IF claim_review->>'finding'<>'verified' THEN
   CONTINUE;
  END IF;
  FOR source IN SELECT value FROM jsonb_array_elements(COALESCE(claim_review->'sources','[]'::jsonb)) LOOP
   source_input_id := source->>'inputId';
   source_revision := (source->>'revision')::bigint;
   source_digest := source->>'sha256';
   SELECT state,source_sha256 INTO current_source_state,current_source_digest
   FROM domain_workflow_content_source_events
   WHERE company_id=NEW.company_id AND input_id=source_input_id AND input_revision=source_revision
   ORDER BY event_seq DESC LIMIT 1;
   IF current_source_state IS DISTINCT FROM 'authorized' OR current_source_digest IS DISTINCT FROM source_digest THEN
    RAISE EXCEPTION 'verified content claim cites a source without current exact company authorization' USING ERRCODE='23514';
   END IF;
   SELECT content_digest,state INTO plan_source_digest,plan_state FROM mission_inputs
   WHERE company_id=NEW.company_id AND input_id=source_input_id AND revision=source_revision;
   IF plan_state IS DISTINCT FROM 'usable' OR plan_source_digest IS DISTINCT FROM source_digest THEN
    RAISE EXCEPTION 'verified content claim source is stale or unusable' USING ERRCODE='23514';
   END IF;
  END LOOP;
 END LOOP;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER domain_workflow_content_review_valid
 BEFORE INSERT ON domain_workflow_content_reviews
 FOR EACH ROW EXECUTE FUNCTION validate_domain_workflow_content_review();
CREATE TRIGGER domain_workflow_content_reviews_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_content_reviews
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
