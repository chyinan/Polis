-- +goose Up
CREATE TABLE domain_workflow_content_publication_simulations (
 company_id text NOT NULL,
 publication_id text NOT NULL,
 review_id text NOT NULL,
 draft_input_id text NOT NULL,
 draft_revision bigint NOT NULL CHECK(draft_revision>0),
 draft_sha256 text NOT NULL CHECK(draft_sha256 ~ '^[a-f0-9]{64}$'),
 mode text NOT NULL CHECK(mode='simulation'),
 external_side_effects boolean NOT NULL DEFAULT false CHECK(external_side_effects=false),
 receipt_sha256 text NOT NULL CHECK(receipt_sha256 ~ '^[a-f0-9]{64}$'),
 receipt_json jsonb NOT NULL CHECK(jsonb_typeof(receipt_json)='object'),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,publication_id),
 UNIQUE(company_id,review_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,review_id) REFERENCES domain_workflow_content_reviews(company_id,review_id),
 FOREIGN KEY(company_id,draft_input_id,draft_revision) REFERENCES domain_workflow_content_drafts(company_id,draft_input_id,draft_revision),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);
CREATE INDEX domain_workflow_content_publication_simulations_recent
 ON domain_workflow_content_publication_simulations(company_id,created_at DESC,publication_id DESC);

CREATE TABLE domain_workflow_content_corrections (
 company_id text NOT NULL,
 correction_id text NOT NULL,
 publication_id text NOT NULL,
 correction_draft_input_id text NOT NULL,
 correction_draft_revision bigint NOT NULL CHECK(correction_draft_revision>0),
 correction_draft_sha256 text NOT NULL CHECK(correction_draft_sha256 ~ '^[a-f0-9]{64}$'),
 rationale text NOT NULL CHECK(char_length(rationale) BETWEEN 1 AND 2000),
 state text NOT NULL DEFAULT 'review_required' CHECK(state='review_required'),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,correction_id),
 UNIQUE(company_id,publication_id,correction_draft_input_id,correction_draft_revision),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,publication_id) REFERENCES domain_workflow_content_publication_simulations(company_id,publication_id),
 FOREIGN KEY(company_id,correction_draft_input_id,correction_draft_revision) REFERENCES domain_workflow_content_drafts(company_id,draft_input_id,draft_revision),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);

CREATE TABLE domain_workflow_content_feedback (
 company_id text NOT NULL,
 feedback_id text NOT NULL,
 publication_id text NOT NULL,
 category text NOT NULL CHECK(category IN ('positive','negative','mixed','inconclusive','correction_requested')),
 note text NOT NULL CHECK(char_length(note) BETWEEN 1 AND 2000),
 state text NOT NULL CHECK(state IN ('recorded','review_required')),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,feedback_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,publication_id) REFERENCES domain_workflow_content_publication_simulations(company_id,publication_id),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);

-- +goose StatementBegin
CREATE FUNCTION validate_domain_workflow_content_publication_simulation() RETURNS trigger AS $$
DECLARE
 review_record domain_workflow_content_reviews%ROWTYPE;
 latest_draft_revision bigint;
 claim_review jsonb;
 source jsonb;
 source_input_id text;
 source_revision bigint;
 source_digest text;
 current_state text;
 current_digest text;
BEGIN
 SELECT * INTO review_record FROM domain_workflow_content_reviews
 WHERE company_id=NEW.company_id AND review_id=NEW.review_id;
 IF NOT FOUND OR review_record.outcome<>'accepted' OR review_record.draft_input_id<>NEW.draft_input_id OR
    review_record.draft_revision<>NEW.draft_revision OR review_record.draft_sha256<>NEW.draft_sha256 THEN
  RAISE EXCEPTION 'simulated publication requires the exact accepted content review' USING ERRCODE='23514';
 END IF;
 SELECT max(draft_revision) INTO latest_draft_revision FROM domain_workflow_content_drafts
 WHERE company_id=NEW.company_id AND draft_input_id=NEW.draft_input_id;
 IF latest_draft_revision<>NEW.draft_revision THEN
  RAISE EXCEPTION 'simulated publication review is stale after a newer draft' USING ERRCODE='23514';
 END IF;
 IF COALESCE(NEW.receipt_json->>'schemaVersion','')<>'polis-content-publication-simulation@1' OR
    COALESCE(NEW.receipt_json->>'draftInputId','')<>NEW.draft_input_id OR
    COALESCE(NEW.receipt_json->>'draftRevision','')<>NEW.draft_revision::text OR
    COALESCE(NEW.receipt_json->>'draftSha256','')<>NEW.draft_sha256 OR
    COALESCE(NEW.receipt_json->>'reviewId','')<>NEW.review_id OR
    COALESCE(NEW.receipt_json->>'externalSideEffects','')<>'false' THEN
  RAISE EXCEPTION 'simulated publication receipt does not bind the reviewed draft' USING ERRCODE='23514';
 END IF;
 FOR claim_review IN SELECT value FROM jsonb_array_elements(review_record.review->'claims') LOOP
  IF claim_review->>'finding'<>'verified' THEN
   CONTINUE;
  END IF;
  FOR source IN SELECT value FROM jsonb_array_elements(COALESCE(claim_review->'sources','[]'::jsonb)) LOOP
   source_input_id := source->>'inputId';
   source_revision := (source->>'revision')::bigint;
   source_digest := source->>'sha256';
   SELECT state,source_sha256 INTO current_state,current_digest FROM domain_workflow_content_source_events
   WHERE company_id=NEW.company_id AND input_id=source_input_id AND input_revision=source_revision
   ORDER BY event_seq DESC LIMIT 1;
   IF current_state IS DISTINCT FROM 'authorized' OR current_digest IS DISTINCT FROM source_digest THEN
    RAISE EXCEPTION 'simulated publication references a source that is no longer authorized' USING ERRCODE='23514';
   END IF;
  END LOOP;
 END LOOP;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER domain_workflow_content_publication_simulation_valid
 BEFORE INSERT ON domain_workflow_content_publication_simulations
 FOR EACH ROW EXECUTE FUNCTION validate_domain_workflow_content_publication_simulation();
CREATE TRIGGER domain_workflow_content_publication_simulations_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_content_publication_simulations
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose StatementBegin
CREATE FUNCTION validate_domain_workflow_content_correction() RETURNS trigger AS $$
DECLARE
 publication domain_workflow_content_publication_simulations%ROWTYPE;
 draft_sha text;
BEGIN
 SELECT * INTO publication FROM domain_workflow_content_publication_simulations
 WHERE company_id=NEW.company_id AND publication_id=NEW.publication_id;
 IF NOT FOUND OR publication.draft_input_id<>NEW.correction_draft_input_id OR
    NEW.correction_draft_revision<=publication.draft_revision THEN
  RAISE EXCEPTION 'content correction must target a newer version of the published draft' USING ERRCODE='23514';
 END IF;
 SELECT draft_sha256 INTO draft_sha FROM domain_workflow_content_drafts
 WHERE company_id=NEW.company_id AND draft_input_id=NEW.correction_draft_input_id AND draft_revision=NEW.correction_draft_revision;
 IF NOT FOUND OR draft_sha<>NEW.correction_draft_sha256 THEN
  RAISE EXCEPTION 'content correction draft digest is not the registered exact revision' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER domain_workflow_content_correction_valid
 BEFORE INSERT ON domain_workflow_content_corrections
 FOR EACH ROW EXECUTE FUNCTION validate_domain_workflow_content_correction();
CREATE TRIGGER domain_workflow_content_corrections_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_content_corrections
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose StatementBegin
CREATE FUNCTION validate_domain_workflow_content_feedback() RETURNS trigger AS $$
BEGIN
 IF (NEW.category IN ('negative','correction_requested') AND NEW.state<>'review_required') OR
    (NEW.category NOT IN ('negative','correction_requested') AND NEW.state NOT IN ('recorded','review_required')) THEN
  RAISE EXCEPTION 'content feedback state does not match its explicit review request category' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER domain_workflow_content_feedback_valid
 BEFORE INSERT ON domain_workflow_content_feedback
 FOR EACH ROW EXECUTE FUNCTION validate_domain_workflow_content_feedback();
CREATE TRIGGER domain_workflow_content_feedback_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_content_feedback
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
