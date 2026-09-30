-- +goose Up
CREATE TABLE domain_workflow_evidence_submissions (
 company_id text NOT NULL,
 record_id text NOT NULL,
 profile_id text NOT NULL,
 profile_revision text NOT NULL,
 readiness_status text NOT NULL CHECK(readiness_status IN ('incomplete','ready_for_review')),
 qualification_status text NOT NULL DEFAULT 'not_run' CHECK(qualification_status='not_run'),
 execution_enabled boolean NOT NULL DEFAULT false CHECK(execution_enabled=false),
 evidence_digest text NOT NULL CHECK(evidence_digest ~ '^[a-f0-9]{64}$'),
 evidence jsonb NOT NULL CHECK(
  jsonb_typeof(evidence)='object'
  AND COALESCE(evidence->>'profileId'=profile_id,false)
  AND COALESCE(evidence->>'profileRevision'=profile_revision,false)
  AND COALESCE(jsonb_typeof(evidence->'evidence')='array',false)
 ),
 reason_codes jsonb NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(reason_codes)='array'),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,record_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);
CREATE TABLE domain_workflow_evidence_items (
 company_id text NOT NULL,
 record_id text NOT NULL,
 area text NOT NULL CHECK(area IN ('quality','intervention','recovery','cost','organization_benefit')),
 input_id text NOT NULL,
 input_revision bigint NOT NULL CHECK(input_revision > 0),
 input_sha256 text NOT NULL CHECK(input_sha256 ~ '^[a-f0-9]{64}$'),
 method_sha256 text NOT NULL CHECK(method_sha256 ~ '^[a-f0-9]{64}$'),
 assessed_by_employee_id text NOT NULL,
 PRIMARY KEY(company_id,record_id,area),
 FOREIGN KEY(company_id,record_id) REFERENCES domain_workflow_evidence_submissions(company_id,record_id),
 FOREIGN KEY(company_id,input_id,input_revision) REFERENCES mission_inputs(company_id,input_id,revision),
 FOREIGN KEY(company_id,assessed_by_employee_id) REFERENCES employees(company_id,id)
);
CREATE INDEX domain_workflow_evidence_submissions_recent
 ON domain_workflow_evidence_submissions(company_id,created_at DESC,record_id DESC);
CREATE INDEX domain_workflow_evidence_items_source
 ON domain_workflow_evidence_items(company_id,input_id,input_revision);
CREATE TRIGGER domain_workflow_evidence_submissions_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_evidence_submissions
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER domain_workflow_evidence_items_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_evidence_items
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DROP TRIGGER domain_workflow_evidence_items_immutable ON domain_workflow_evidence_items;
DROP INDEX domain_workflow_evidence_items_source;
DROP TABLE domain_workflow_evidence_items;
DROP TRIGGER domain_workflow_evidence_submissions_immutable ON domain_workflow_evidence_submissions;
DROP INDEX domain_workflow_evidence_submissions_recent;
DROP TABLE domain_workflow_evidence_submissions;
