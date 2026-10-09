-- +goose Up
ALTER TABLE artifacts ADD COLUMN content_format text NOT NULL DEFAULT 'opaque'
 CHECK(content_format IN ('opaque','go_patch'));
ALTER TABLE artifacts ADD CONSTRAINT artifacts_go_patch_format_check
 CHECK(content_format<>'go_patch' OR artifact_kind='deliverable');
ALTER TABLE artifacts DROP CONSTRAINT artifacts_bytes_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_bytes_check CHECK(
 (artifact_kind='deliverable' AND content_format='opaque' AND bytes BETWEEN 1 AND 4096) OR
 (artifact_kind='deliverable' AND content_format='go_patch' AND bytes BETWEEN 1 AND 25165824) OR
 (artifact_kind IN ('workspace_snapshot','operation_evidence') AND bytes BETWEEN 1 AND 8388608));

CREATE TABLE go_patch_deliveries(
 company_id text NOT NULL,
 artifact_id text NOT NULL,
 task_id text NOT NULL,
 job_id text NOT NULL,
 patch_sha256 text NOT NULL CHECK(patch_sha256 ~ '^[a-f0-9]{64}$'),
 candidate_snapshot_sha256 text NOT NULL CHECK(candidate_snapshot_sha256 ~ '^[a-f0-9]{64}$'),
 candidate_revision bigint NOT NULL CHECK(candidate_revision>0),
 job_binding_sha256 text NOT NULL CHECK(job_binding_sha256 ~ '^[a-f0-9]{64}$'),
 result_sha256 text NOT NULL CHECK(result_sha256 ~ '^[a-f0-9]{64}$'),
 replay_verified boolean NOT NULL CHECK(replay_verified),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,artifact_id),
 UNIQUE(company_id,task_id),
 FOREIGN KEY(company_id,artifact_id) REFERENCES task_validation_artifact_qualifications(company_id,artifact_id),
 FOREIGN KEY(company_id,job_id) REFERENCES go_test_job_results(company_id,job_id),
 FOREIGN KEY(company_id,task_id) REFERENCES task_validation_bindings(company_id,task_id)
);
CREATE TRIGGER go_patch_deliveries_immutable BEFORE UPDATE OR DELETE ON go_patch_deliveries
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
-- +goose StatementBegin
CREATE FUNCTION enforce_go_patch_delivery() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE job_record RECORD; result_document jsonb; receipt_document jsonb;
BEGIN
 SELECT b.*,r.outcome,r.result_sha256 AS stored_result_sha256,r.result,r.runner_receipt,
        a.digest AS artifact_digest,a.content_format,q.workspace_digest,q.workspace_revision
 INTO job_record FROM go_test_job_bindings b
 JOIN go_test_job_results r ON r.company_id=b.company_id AND r.job_id=b.job_id
 JOIN task_validation_artifact_qualifications q ON q.company_id=b.company_id AND q.task_id=b.task_id AND q.artifact_id=NEW.artifact_id
 JOIN artifacts a ON a.company_id=q.company_id AND a.id=q.artifact_id
 WHERE b.company_id=NEW.company_id AND b.job_id=NEW.job_id;
 IF NOT FOUND THEN RAISE EXCEPTION 'Go patch has no matching qualified JobRun' USING ERRCODE='23514'; END IF;
 result_document:=convert_from(job_record.result,'UTF8')::jsonb;
 receipt_document:=convert_from(job_record.runner_receipt,'UTF8')::jsonb;
 IF job_record.task_id IS DISTINCT FROM NEW.task_id OR job_record.outcome IS DISTINCT FROM 'passed'
 OR job_record.content_format IS DISTINCT FROM 'go_patch' OR job_record.artifact_digest IS DISTINCT FROM NEW.patch_sha256
 OR job_record.candidate_snapshot_sha256 IS DISTINCT FROM NEW.candidate_snapshot_sha256
 OR job_record.candidate_workspace_revision IS DISTINCT FROM NEW.candidate_revision
 OR job_record.workspace_digest IS DISTINCT FROM NEW.candidate_snapshot_sha256
 OR job_record.workspace_revision IS DISTINCT FROM NEW.candidate_revision
 OR job_record.binding_sha256 IS DISTINCT FROM NEW.job_binding_sha256
 OR job_record.stored_result_sha256 IS DISTINCT FROM NEW.result_sha256
 OR result_document->>'executionKind' IS DISTINCT FROM 'real'
 OR result_document->>'qualificationEligible' IS DISTINCT FROM 'true'
 OR receipt_document->>'processStarted' IS DISTINCT FROM 'true'
 OR receipt_document->>'exitObserved' IS DISTINCT FROM 'true'
 OR receipt_document->>'exitCode' IS DISTINCT FROM '0'
 OR receipt_document->>'cleanupConfirmed' IS DISTINCT FROM 'true' THEN
  RAISE EXCEPTION 'Go patch execution or candidate identity is not qualified' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER go_patch_delivery_validate BEFORE INSERT ON go_patch_deliveries
 FOR EACH ROW EXECUTE FUNCTION enforce_go_patch_delivery();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM go_patch_deliveries) OR EXISTS(SELECT 1 FROM artifacts WHERE content_format='go_patch') THEN
  RAISE EXCEPTION 'cannot remove Go patch delivery history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TABLE go_patch_deliveries;
DROP FUNCTION enforce_go_patch_delivery();
ALTER TABLE artifacts DROP CONSTRAINT artifacts_go_patch_format_check;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_bytes_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_bytes_check CHECK(
 (artifact_kind='deliverable' AND bytes BETWEEN 1 AND 4096) OR
 (artifact_kind IN ('workspace_snapshot','operation_evidence') AND bytes BETWEEN 1 AND 8388608));
ALTER TABLE artifacts DROP COLUMN content_format;
