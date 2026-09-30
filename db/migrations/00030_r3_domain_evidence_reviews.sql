-- +goose Up
CREATE TABLE domain_workflow_evidence_reviews (
 company_id text NOT NULL,
 review_id text NOT NULL,
 record_id text NOT NULL,
 outcome text NOT NULL CHECK(outcome IN ('evidence_references_accepted','evidence_references_rejected','more_evidence_required')),
 reviewer_employee_id text NOT NULL,
 rationale text NOT NULL CHECK(char_length(rationale) BETWEEN 1 AND 2000 AND rationale ~ '[^[:space:]]'),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,review_id),
 UNIQUE(company_id,record_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,record_id) REFERENCES domain_workflow_evidence_submissions(company_id,record_id),
 FOREIGN KEY(company_id,reviewer_employee_id) REFERENCES employees(company_id,id)
);

-- +goose StatementBegin
CREATE FUNCTION enforce_domain_evidence_review_independence() RETURNS trigger LANGUAGE plpgsql AS $$
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

CREATE TRIGGER domain_workflow_evidence_reviews_require_independent_reviewer
 BEFORE INSERT ON domain_workflow_evidence_reviews
 FOR EACH ROW EXECUTE FUNCTION enforce_domain_evidence_review_independence();
CREATE TRIGGER domain_workflow_evidence_reviews_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_evidence_reviews
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE INDEX domain_workflow_evidence_reviews_recent
 ON domain_workflow_evidence_reviews(company_id,created_at DESC,review_id DESC);

-- +goose Down
DROP TRIGGER domain_workflow_evidence_reviews_immutable ON domain_workflow_evidence_reviews;
DROP TRIGGER domain_workflow_evidence_reviews_require_independent_reviewer ON domain_workflow_evidence_reviews;
DROP INDEX domain_workflow_evidence_reviews_recent;
DROP TABLE domain_workflow_evidence_reviews;
DROP FUNCTION enforce_domain_evidence_review_independence();
