-- +goose Up
CREATE SEQUENCE domain_workflow_content_review_order_seq;
-- PostgreSQL truncates generated constraint names at NAMEDATALEN. Resolve this
-- unique constraint by definition so the migration does not depend on the
-- truncated identifier chosen by a particular server build.
-- +goose StatementBegin
DO $$
DECLARE
 review_unique_constraint text;
BEGIN
 SELECT conname INTO review_unique_constraint
 FROM pg_constraint
 WHERE conrelid='domain_workflow_content_reviews'::regclass
   AND contype='u'
   AND pg_get_constraintdef(oid)='UNIQUE (company_id, draft_input_id, draft_revision)';
 IF review_unique_constraint IS NULL THEN
  RAISE EXCEPTION 'expected unique content review constraint was not found';
 END IF;
 EXECUTE format('ALTER TABLE domain_workflow_content_reviews DROP CONSTRAINT %I',review_unique_constraint);
END
$$;
-- +goose StatementEnd
ALTER TABLE domain_workflow_content_reviews
 ADD COLUMN review_seq bigint NOT NULL DEFAULT nextval('domain_workflow_content_review_order_seq'),
 ADD COLUMN correction_id text,
 ADD CONSTRAINT domain_workflow_content_reviews_correction_fk
  FOREIGN KEY(company_id,correction_id) REFERENCES domain_workflow_content_corrections(company_id,correction_id);
ALTER TABLE domain_workflow_content_corrections
 ADD COLUMN correction_seq bigint NOT NULL DEFAULT nextval('domain_workflow_content_review_order_seq');

CREATE UNIQUE INDEX domain_workflow_content_reviews_initial_unique
 ON domain_workflow_content_reviews(company_id,draft_input_id,draft_revision) WHERE correction_id IS NULL;
CREATE UNIQUE INDEX domain_workflow_content_reviews_correction_unique
 ON domain_workflow_content_reviews(company_id,correction_id) WHERE correction_id IS NOT NULL;

-- +goose StatementBegin
CREATE FUNCTION validate_domain_workflow_content_review_correction_link() RETURNS trigger AS $$
DECLARE
 correction_input_id text;
 correction_revision bigint;
 correction_event_seq bigint;
 pending_correction boolean;
BEGIN
 IF NEW.correction_id IS NULL THEN
  SELECT EXISTS(
   SELECT 1 FROM domain_workflow_content_corrections c
   WHERE c.company_id=NEW.company_id AND c.correction_draft_input_id=NEW.draft_input_id AND c.correction_draft_revision=NEW.draft_revision
     AND NOT EXISTS(SELECT 1 FROM domain_workflow_content_reviews r WHERE r.company_id=c.company_id AND r.correction_id=c.correction_id AND r.outcome='accepted')
  ) INTO pending_correction;
  IF pending_correction THEN
   RAISE EXCEPTION 'corrected draft requires a review linked to its correction before publication' USING ERRCODE='23514';
  END IF;
 ELSE
  SELECT correction_draft_input_id,correction_draft_revision,correction_seq
  INTO correction_input_id,correction_revision,correction_event_seq
  FROM domain_workflow_content_corrections WHERE company_id=NEW.company_id AND correction_id=NEW.correction_id;
  IF NOT FOUND OR correction_input_id<>NEW.draft_input_id OR correction_revision<>NEW.draft_revision OR NEW.review_seq<=correction_event_seq THEN
   RAISE EXCEPTION 'content correction review must follow and bind the exact correction draft' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER domain_workflow_content_review_correction_link_valid
 BEFORE INSERT ON domain_workflow_content_reviews
 FOR EACH ROW EXECUTE FUNCTION validate_domain_workflow_content_review_correction_link();

-- +goose StatementBegin
CREATE FUNCTION validate_domain_workflow_content_publication_correction_gate() RETURNS trigger AS $$
DECLARE
 correction record;
BEGIN
 FOR correction IN SELECT correction_id FROM domain_workflow_content_corrections
 WHERE company_id=NEW.company_id AND correction_draft_input_id=NEW.draft_input_id AND correction_draft_revision=NEW.draft_revision LOOP
  IF NOT EXISTS(SELECT 1 FROM domain_workflow_content_reviews r
    WHERE r.company_id=NEW.company_id AND r.correction_id=correction.correction_id AND r.outcome='accepted'
      AND r.draft_input_id=NEW.draft_input_id AND r.draft_revision=NEW.draft_revision) THEN
   RAISE EXCEPTION 'simulated publication requires an accepted post-correction review' USING ERRCODE='23514';
  END IF;
 END LOOP;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER domain_workflow_content_publication_correction_gate
 BEFORE INSERT ON domain_workflow_content_publication_simulations
 FOR EACH ROW EXECUTE FUNCTION validate_domain_workflow_content_publication_correction_gate();
