-- +goose Up
CREATE TABLE domain_workflow_substantive_assessments (
 company_id text NOT NULL,
 assessment_id text NOT NULL,
 record_id text NOT NULL,
 evidence_digest text NOT NULL CHECK(evidence_digest ~ '^[a-f0-9]{64}$'),
 outcome text NOT NULL CHECK(outcome IN ('evidence_accepted','evidence_rejected','more_evidence_required')),
 reviewer_employee_id text NOT NULL,
 area_assessments jsonb NOT NULL CHECK(jsonb_typeof(area_assessments)='array'),
 previewed_evidence jsonb NOT NULL CHECK(jsonb_typeof(previewed_evidence)='array'),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,assessment_id),
 UNIQUE(company_id,record_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,record_id) REFERENCES domain_workflow_evidence_submissions(company_id,record_id),
 FOREIGN KEY(company_id,reviewer_employee_id) REFERENCES employees(company_id,id)
);

CREATE TABLE domain_workflow_substantive_area_assessments (
 company_id text NOT NULL,
 record_id text NOT NULL,
 area text NOT NULL CHECK(area IN ('quality','intervention','recovery','cost','organization_benefit')),
 outcome text NOT NULL CHECK(outcome IN ('accepted','rejected','insufficient')),
 rationale text NOT NULL CHECK(char_length(rationale) BETWEEN 1 AND 1000 AND rationale ~ '[^[:space:]]'),
 PRIMARY KEY(company_id,record_id,area),
 FOREIGN KEY(company_id,record_id) REFERENCES domain_workflow_substantive_assessments(company_id,record_id),
 FOREIGN KEY(company_id,record_id,area) REFERENCES domain_workflow_evidence_items(company_id,record_id,area)
);

CREATE INDEX domain_workflow_substantive_assessments_recent
 ON domain_workflow_substantive_assessments(company_id,created_at DESC,assessment_id DESC);

-- +goose StatementBegin
CREATE FUNCTION enforce_domain_substantive_assessment() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 submission RECORD;
 prior_review RECORD;
 reviewer_role text;
 preview_entry jsonb;
 entry_path text;
BEGIN
 SELECT profile_id,profile_revision,readiness_status,evidence_digest
 INTO submission
 FROM domain_workflow_evidence_submissions
 WHERE company_id=NEW.company_id AND record_id=NEW.record_id FOR SHARE;
 IF submission.readiness_status IS DISTINCT FROM 'ready_for_review' OR submission.evidence_digest IS DISTINCT FROM NEW.evidence_digest THEN
  RAISE EXCEPTION 'substantive assessment does not match a ready evidence submission' USING ERRCODE='23514';
 END IF;
 IF (submission.profile_id,submission.profile_revision) NOT IN (
  ('content-operations-reference','content-operations@1'),('research-simulation-reference','research-simulation@1')
 ) THEN
  RAISE EXCEPTION 'unknown domain reference profile' USING ERRCODE='23514';
 END IF;
 SELECT outcome,review_contract_revision INTO prior_review
 FROM domain_workflow_evidence_reviews WHERE company_id=NEW.company_id AND record_id=NEW.record_id;
 IF prior_review.outcome IS DISTINCT FROM 'evidence_references_accepted' OR prior_review.review_contract_revision IS DISTINCT FROM 1 THEN
  RAISE EXCEPTION 'substantive assessment requires accepted, preview-bound references' USING ERRCODE='23514';
 END IF;
 SELECT role_name INTO reviewer_role FROM employees
 WHERE company_id=NEW.company_id AND id=NEW.reviewer_employee_id FOR SHARE;
 IF reviewer_role IS DISTINCT FROM 'review' THEN
  RAISE EXCEPTION 'substantive reviewer must have review role' USING ERRCODE='23514';
 END IF;
 IF EXISTS (
  SELECT 1 FROM domain_workflow_evidence_items
  WHERE company_id=NEW.company_id AND record_id=NEW.record_id AND assessed_by_employee_id=NEW.reviewer_employee_id
 ) THEN
  RAISE EXCEPTION 'substantive reviewer must be independent of assessors' USING ERRCODE='23514';
 END IF;
 IF jsonb_typeof(NEW.area_assessments) IS DISTINCT FROM 'array' OR jsonb_array_length(NEW.area_assessments)=0
    OR jsonb_typeof(NEW.previewed_evidence) IS DISTINCT FROM 'array' OR jsonb_array_length(NEW.previewed_evidence)=0 THEN
  RAISE EXCEPTION 'substantive assessment requires area decisions and bound previews' USING ERRCODE='23514';
 END IF;
 IF EXISTS (
  SELECT 1 FROM domain_workflow_evidence_items evidence_item
  WHERE evidence_item.company_id=NEW.company_id AND evidence_item.record_id=NEW.record_id
    AND NOT EXISTS (
     SELECT 1 FROM jsonb_array_elements(NEW.previewed_evidence) preview(value)
     WHERE preview.value->>'area'=evidence_item.area
       AND preview.value->>'sourceDigest'=evidence_item.input_sha256
       AND COALESCE(preview.value->>'contentDigest','') ~ '^[a-f0-9]{64}$'
       AND preview.value->>'mediaType' IN ('text/plain','text/markdown','text/csv','application/json','image/png','image/jpeg')
       AND COALESCE(preview.value->>'relativePath','')<>''
    )
 ) THEN
  RAISE EXCEPTION 'substantive assessment must preview every evidence area' USING ERRCODE='23514';
 END IF;
 IF EXISTS (
  SELECT 1 FROM jsonb_array_elements(NEW.previewed_evidence) preview(value)
  WHERE NOT EXISTS (
   SELECT 1 FROM domain_workflow_evidence_items evidence_item
   WHERE evidence_item.company_id=NEW.company_id AND evidence_item.record_id=NEW.record_id
     AND evidence_item.area=preview.value->>'area' AND evidence_item.input_sha256=preview.value->>'sourceDigest'
  )
 ) THEN
  RAISE EXCEPTION 'substantive preview source does not match submission' USING ERRCODE='23514';
 END IF;
 IF EXISTS (
  SELECT 1 FROM jsonb_array_elements(NEW.area_assessments) item(value)
  WHERE COALESCE(item.value->>'outcome','') NOT IN ('accepted','rejected','insufficient')
     OR length(trim(COALESCE(item.value->>'rationale',''))) NOT BETWEEN 1 AND 1000
     OR COALESCE(item.value->>'area','') NOT IN ('quality','intervention','recovery','cost','organization_benefit')
 ) THEN
  RAISE EXCEPTION 'substantive area decision is invalid' USING ERRCODE='23514';
 END IF;
 FOR preview_entry IN SELECT value FROM jsonb_array_elements(NEW.previewed_evidence) AS item(value) LOOP
  entry_path := preview_entry->>'relativePath';
  IF char_length(entry_path)>1024 OR left(entry_path,1)='/' OR position(chr(92) in entry_path)>0
   OR EXISTS(SELECT 1 FROM unnest(string_to_array(entry_path,'/')) AS path_segment WHERE path_segment IN ('','.','..'))
   OR entry_path='pdf/extraction.json' OR regexp_replace(entry_path,'^.*/','')='.polis-git-source.json' THEN
   RAISE EXCEPTION 'substantive review cannot attest to an unsafe or metadata-only preview' USING ERRCODE='23514';
  END IF;
 END LOOP;
 IF jsonb_array_length(NEW.previewed_evidence) <>
  (SELECT count(DISTINCT (value->>'area',value->>'relativePath')) FROM jsonb_array_elements(NEW.previewed_evidence) AS item(value)) THEN
  RAISE EXCEPTION 'substantive preview attestation contains duplicates' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd

CREATE TRIGGER domain_workflow_substantive_assessments_validate
 BEFORE INSERT ON domain_workflow_substantive_assessments
 FOR EACH ROW EXECUTE FUNCTION enforce_domain_substantive_assessment();

-- +goose StatementBegin
CREATE FUNCTION verify_domain_substantive_assessment_areas() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 assessment RECORD;
 expected_count integer;
 area_count integer;
 derived_outcome text;
BEGIN
 SELECT a.company_id,a.record_id,a.outcome,a.area_assessments,s.profile_id,s.profile_revision
 INTO assessment
 FROM domain_workflow_substantive_assessments a
 JOIN domain_workflow_evidence_submissions s ON s.company_id=a.company_id AND s.record_id=a.record_id
 WHERE a.company_id=NEW.company_id AND a.record_id=NEW.record_id;
 IF assessment.profile_id='content-operations-reference' AND assessment.profile_revision='content-operations@1' THEN
  expected_count := 5;
  IF EXISTS (SELECT 1 FROM unnest(ARRAY['quality','intervention','recovery','cost','organization_benefit']) required(area)
   WHERE NOT EXISTS (SELECT 1 FROM domain_workflow_substantive_area_assessments item WHERE item.company_id=NEW.company_id AND item.record_id=NEW.record_id AND item.area=required.area)) THEN
   RAISE EXCEPTION 'content operations substantive area coverage is incomplete' USING ERRCODE='23514';
  END IF;
 ELSIF assessment.profile_id='research-simulation-reference' AND assessment.profile_revision='research-simulation@1' THEN
  expected_count := 4;
  IF EXISTS (SELECT 1 FROM unnest(ARRAY['quality','recovery','cost','organization_benefit']) required(area)
   WHERE NOT EXISTS (SELECT 1 FROM domain_workflow_substantive_area_assessments item WHERE item.company_id=NEW.company_id AND item.record_id=NEW.record_id AND item.area=required.area))
    OR EXISTS (SELECT 1 FROM domain_workflow_substantive_area_assessments item WHERE item.company_id=NEW.company_id AND item.record_id=NEW.record_id AND item.area='intervention') THEN
   RAISE EXCEPTION 'research simulation substantive area coverage is invalid' USING ERRCODE='23514';
  END IF;
 ELSE
  RAISE EXCEPTION 'unknown domain reference profile' USING ERRCODE='23514';
 END IF;
 SELECT count(*) INTO area_count FROM domain_workflow_substantive_area_assessments WHERE company_id=NEW.company_id AND record_id=NEW.record_id;
 IF area_count<>expected_count THEN
  RAISE EXCEPTION 'substantive assessment area count differs from profile' USING ERRCODE='23514';
 END IF;
 IF jsonb_array_length(assessment.area_assessments)<>expected_count
    OR (SELECT count(DISTINCT value->>'area') FROM jsonb_array_elements(assessment.area_assessments) AS item(value))<>expected_count
    OR EXISTS (
  SELECT 1 FROM jsonb_array_elements(assessment.area_assessments) expected(value)
  LEFT JOIN domain_workflow_substantive_area_assessments actual
   ON actual.company_id=NEW.company_id AND actual.record_id=NEW.record_id AND actual.area=expected.value->>'area'
  WHERE actual.area IS NULL OR actual.outcome IS DISTINCT FROM expected.value->>'outcome' OR actual.rationale IS DISTINCT FROM expected.value->>'rationale'
 ) OR EXISTS (
  SELECT 1 FROM domain_workflow_substantive_area_assessments actual
  WHERE actual.company_id=NEW.company_id AND actual.record_id=NEW.record_id
    AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(assessment.area_assessments) expected(value) WHERE expected.value->>'area'=actual.area AND expected.value->>'outcome'=actual.outcome AND expected.value->>'rationale'=actual.rationale)
 ) THEN
  RAISE EXCEPTION 'substantive assessment JSON and normalized rows differ' USING ERRCODE='23514';
 END IF;
 SELECT CASE
  WHEN bool_or(outcome='rejected') THEN 'evidence_rejected'
  WHEN bool_or(outcome='insufficient') THEN 'more_evidence_required'
  ELSE 'evidence_accepted'
 END INTO derived_outcome
 FROM domain_workflow_substantive_area_assessments WHERE company_id=NEW.company_id AND record_id=NEW.record_id;
 IF assessment.outcome IS DISTINCT FROM derived_outcome THEN
  RAISE EXCEPTION 'substantive overall outcome differs from area decisions' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER domain_workflow_substantive_assessments_verify_areas
 AFTER INSERT ON domain_workflow_substantive_assessments
 DEFERRABLE INITIALLY DEFERRED
 FOR EACH ROW EXECUTE FUNCTION verify_domain_substantive_assessment_areas();
CREATE TRIGGER domain_workflow_substantive_assessments_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_substantive_assessments
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER domain_workflow_substantive_area_assessments_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_substantive_area_assessments
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM domain_workflow_substantive_assessments)
    OR EXISTS(SELECT 1 FROM domain_workflow_substantive_area_assessments) THEN
  RAISE EXCEPTION 'cannot roll back substantive domain evidence assessments';
 END IF;
END
$$;
-- +goose StatementEnd
DROP TRIGGER domain_workflow_substantive_area_assessments_immutable ON domain_workflow_substantive_area_assessments;
DROP TRIGGER domain_workflow_substantive_assessments_immutable ON domain_workflow_substantive_assessments;
DROP TRIGGER domain_workflow_substantive_assessments_verify_areas ON domain_workflow_substantive_assessments;
DROP TRIGGER domain_workflow_substantive_assessments_validate ON domain_workflow_substantive_assessments;
DROP FUNCTION verify_domain_substantive_assessment_areas();
DROP FUNCTION enforce_domain_substantive_assessment();
DROP INDEX domain_workflow_substantive_assessments_recent;
DROP TABLE domain_workflow_substantive_area_assessments;
DROP TABLE domain_workflow_substantive_assessments;
