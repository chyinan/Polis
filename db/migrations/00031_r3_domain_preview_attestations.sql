-- pattern: Imperative Shell
-- +goose Up
ALTER TABLE domain_workflow_evidence_reviews
 ADD COLUMN previewed_evidence jsonb NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(previewed_evidence)='array'),
 ADD COLUMN review_contract_revision smallint NOT NULL DEFAULT 0 CHECK(review_contract_revision IN (0,1));
ALTER TABLE domain_workflow_evidence_reviews ALTER COLUMN review_contract_revision SET DEFAULT 1;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enforce_domain_evidence_review_independence() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 submission_readiness text;
 reviewer_role text;
 preview_entry jsonb;
 entry_path text;
BEGIN
 SELECT readiness_status INTO submission_readiness
 FROM domain_workflow_evidence_submissions
 WHERE company_id=NEW.company_id AND record_id=NEW.record_id;
 IF submission_readiness IS DISTINCT FROM 'ready_for_review' THEN
  RAISE EXCEPTION 'domain evidence submission is not ready for review' USING ERRCODE='23514';
 END IF;
 SELECT role_name INTO reviewer_role FROM employees
 WHERE company_id=NEW.company_id AND id=NEW.reviewer_employee_id FOR SHARE;
 IF reviewer_role IS DISTINCT FROM 'review' THEN
  RAISE EXCEPTION 'domain evidence reviewer must have review role' USING ERRCODE='23514';
 END IF;
 IF EXISTS (
  SELECT 1 FROM domain_workflow_evidence_items
  WHERE company_id=NEW.company_id AND record_id=NEW.record_id AND assessed_by_employee_id=NEW.reviewer_employee_id
 ) THEN
  RAISE EXCEPTION 'domain evidence reviewer must be independent of assessors' USING ERRCODE='23514';
 END IF;
 IF NEW.review_contract_revision IS DISTINCT FROM 1 OR jsonb_typeof(NEW.previewed_evidence) IS DISTINCT FROM 'array' THEN
  RAISE EXCEPTION 'domain evidence review must bind preview attestations' USING ERRCODE='23514';
 END IF;
 IF jsonb_array_length(NEW.previewed_evidence)=0 THEN
  RAISE EXCEPTION 'domain evidence review must bind preview attestations' USING ERRCODE='23514';
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
  RAISE EXCEPTION 'domain evidence review must preview every evidence area' USING ERRCODE='23514';
 END IF;
  IF EXISTS (
   SELECT 1 FROM jsonb_array_elements(NEW.previewed_evidence) preview(value)
   WHERE NOT EXISTS (
   SELECT 1 FROM domain_workflow_evidence_items evidence_item
   WHERE evidence_item.company_id=NEW.company_id AND evidence_item.record_id=NEW.record_id
    AND evidence_item.area=preview.value->>'area'
    AND evidence_item.input_sha256=preview.value->>'sourceDigest'
  )
 ) THEN
   RAISE EXCEPTION 'domain evidence preview attestation does not match a referenced source' USING ERRCODE='23514';
  END IF;
  IF EXISTS (
   SELECT 1 FROM jsonb_array_elements(NEW.previewed_evidence) preview(value)
   WHERE COALESCE(preview.value->>'mediaType','') NOT IN ('text/plain','text/markdown','text/csv','application/json','image/png','image/jpeg')
  ) THEN
   RAISE EXCEPTION 'domain evidence preview media type is not allowed' USING ERRCODE='23514';
  END IF;
 FOR preview_entry IN SELECT value FROM jsonb_array_elements(NEW.previewed_evidence) AS item(value) LOOP
  entry_path := preview_entry->>'relativePath';
  IF char_length(entry_path)>1024 OR left(entry_path,1)='/' OR position(chr(92) in entry_path)>0
   OR EXISTS(SELECT 1 FROM unnest(string_to_array(entry_path,'/')) AS path_segment WHERE path_segment IN ('','.','..')) THEN
   RAISE EXCEPTION 'domain evidence preview path is invalid' USING ERRCODE='23514';
  END IF;
  IF preview_entry->>'mediaType'='application/pdf'
   OR entry_path='pdf/extraction.json'
   OR regexp_replace(entry_path, '^.*/', '')='.polis-git-source.json' THEN
   RAISE EXCEPTION 'domain evidence review cannot attest to metadata-only preview' USING ERRCODE='23514';
  END IF;
 END LOOP;
 IF (SELECT count(*) FROM jsonb_array_elements(NEW.previewed_evidence)) <>
  (SELECT count(DISTINCT (value->>'area',value->>'relativePath')) FROM jsonb_array_elements(NEW.previewed_evidence) AS item(value)) THEN
  RAISE EXCEPTION 'domain evidence preview attestation contains duplicates' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enforce_domain_evidence_review_independence() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 submission_readiness text;
 reviewer_role text;
BEGIN
 SELECT readiness_status INTO submission_readiness
 FROM domain_workflow_evidence_submissions
 WHERE company_id=NEW.company_id AND record_id=NEW.record_id;
 IF submission_readiness IS DISTINCT FROM 'ready_for_review' THEN
  RAISE EXCEPTION 'domain evidence submission is not ready for review' USING ERRCODE='23514';
 END IF;
 SELECT role_name INTO reviewer_role FROM employees
 WHERE company_id=NEW.company_id AND id=NEW.reviewer_employee_id FOR SHARE;
 IF reviewer_role IS DISTINCT FROM 'review' THEN
  RAISE EXCEPTION 'domain evidence reviewer must have review role' USING ERRCODE='23514';
 END IF;
 IF EXISTS (
  SELECT 1 FROM domain_workflow_evidence_items
  WHERE company_id=NEW.company_id AND record_id=NEW.record_id AND assessed_by_employee_id=NEW.reviewer_employee_id
 ) THEN
  RAISE EXCEPTION 'domain evidence reviewer must be independent of assessors' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
ALTER TABLE domain_workflow_evidence_reviews DROP COLUMN review_contract_revision, DROP COLUMN previewed_evidence;
