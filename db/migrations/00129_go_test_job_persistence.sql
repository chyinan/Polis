-- +goose Up
ALTER TABLE job_runs DROP CONSTRAINT job_runs_kind_check;
ALTER TABLE job_runs ADD CONSTRAINT job_runs_kind_check
 CHECK(kind IN ('batch','service','controlled_input','go_test'));
ALTER TABLE job_runs ALTER COLUMN session_id DROP NOT NULL;
ALTER TABLE job_runs ALTER COLUMN environment_revision_id DROP NOT NULL;
ALTER TABLE job_runs ADD CONSTRAINT job_runs_execution_binding_check CHECK (
 (kind='go_test' AND session_id IS NULL AND environment_revision_id IS NULL AND service_id IS NULL AND handover_id IS NULL) OR
 (kind<>'go_test' AND session_id IS NOT NULL AND environment_revision_id IS NOT NULL)
);
ALTER TABLE job_runs ADD CONSTRAINT job_runs_company_job_kind_unique UNIQUE(company_id,job_id,kind);
ALTER TABLE job_runs ADD CONSTRAINT job_runs_company_job_task_unique UNIQUE(company_id,job_id,task_id);

ALTER TABLE job_run_events DROP CONSTRAINT job_run_events_state_check;
ALTER TABLE job_run_events ADD CONSTRAINT job_run_events_state_check
 CHECK(state IN ('accepted','blocked','starting','running','exited','failed','cancelled','outcome_unknown'));

ALTER TABLE worker_workspace_roots
 ADD COLUMN source_commit_oid text,
 ADD COLUMN source_input_manifest_sha256 text,
 ADD CONSTRAINT worker_workspace_roots_git_source_check CHECK (
  (source_commit_oid IS NULL AND source_input_manifest_sha256 IS NULL) OR
  (source_commit_oid IS NOT NULL AND source_input_manifest_sha256 IS NOT NULL AND
   source_commit_oid ~ '^([a-f0-9]{40}|[a-f0-9]{64})$' AND source_input_manifest_sha256 ~ '^[a-f0-9]{64}$')
 );
ALTER TABLE task_takeover_leases
 ADD COLUMN base_tree_source_commit_oid text,
 ADD COLUMN base_tree_input_manifest_sha256 text,
 ADD CONSTRAINT task_takeover_leases_git_source_check CHECK (
  (base_tree_source_commit_oid IS NULL AND base_tree_input_manifest_sha256 IS NULL) OR
  (base_tree_source_commit_oid IS NOT NULL AND base_tree_input_manifest_sha256 IS NOT NULL AND
   base_tree_source_commit_oid ~ '^([a-f0-9]{40}|[a-f0-9]{64})$' AND base_tree_input_manifest_sha256 ~ '^[a-f0-9]{64}$')
 );

CREATE TABLE go_test_job_bindings (
 company_id text NOT NULL,
 job_id text NOT NULL,
 job_kind text NOT NULL DEFAULT 'go_test' CHECK(job_kind='go_test'),
 mission_id text NOT NULL,
 task_id text NOT NULL,
 task_validation_binding_sha256 text NOT NULL CHECK(task_validation_binding_sha256 ~ '^[a-f0-9]{64}$'),
 base_commit_oid text NOT NULL CHECK(base_commit_oid ~ '^([a-f0-9]{40}|[a-f0-9]{64})$'),
 input_manifest_sha256 text NOT NULL CHECK(input_manifest_sha256 ~ '^[a-f0-9]{64}$'),
 base_snapshot_artifact_id text NOT NULL,
 base_snapshot_sha256 text NOT NULL CHECK(base_snapshot_sha256 ~ '^[a-f0-9]{64}$'),
 base_workspace_revision bigint NOT NULL CHECK(base_workspace_revision>0),
 base_files_sha256 text NOT NULL CHECK(base_files_sha256 ~ '^[a-f0-9]{64}$'),
 candidate_snapshot_artifact_id text NOT NULL,
 candidate_snapshot_sha256 text NOT NULL CHECK(candidate_snapshot_sha256 ~ '^[a-f0-9]{64}$'),
 candidate_workspace_revision bigint NOT NULL CHECK(candidate_workspace_revision>base_workspace_revision),
 candidate_files_sha256 text NOT NULL CHECK(candidate_files_sha256 ~ '^[a-f0-9]{64}$'),
 profile_sha256 text NOT NULL CHECK(profile_sha256 ~ '^[a-f0-9]{64}$'),
 runner_sha256 text NOT NULL CHECK(runner_sha256 ~ '^[a-f0-9]{64}$'),
 toolchain_sha256 text NOT NULL CHECK(toolchain_sha256 ~ '^[a-f0-9]{64}$'),
 command_sha256 text NOT NULL CHECK(command_sha256 ~ '^[a-f0-9]{64}$'),
 test_manifest_sha256 text NOT NULL CHECK(test_manifest_sha256 ~ '^[a-f0-9]{64}$'),
 resource_limits jsonb NOT NULL CHECK(jsonb_typeof(resource_limits)='object'),
 binding bytea NOT NULL CHECK(octet_length(binding) BETWEEN 1 AND 262144),
 binding_sha256 text NOT NULL CHECK(binding_sha256 ~ '^[a-f0-9]{64}$'),
 host_qualification_sha256 text CHECK(host_qualification_sha256 IS NULL OR host_qualification_sha256 ~ '^[a-f0-9]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,job_id),
 FOREIGN KEY(company_id,job_id,job_kind) REFERENCES job_runs(company_id,job_id,kind),
 FOREIGN KEY(company_id,job_id,task_id) REFERENCES job_runs(company_id,job_id,task_id),
 FOREIGN KEY(company_id,task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,task_id) REFERENCES task_validation_bindings(company_id,task_id),
 FOREIGN KEY(company_id,base_snapshot_artifact_id) REFERENCES artifacts(company_id,id),
 FOREIGN KEY(company_id,candidate_snapshot_artifact_id) REFERENCES artifacts(company_id,id),
 CHECK(base_snapshot_artifact_id<>candidate_snapshot_artifact_id)
);
CREATE INDEX go_test_job_bindings_task_created ON go_test_job_bindings(company_id,task_id,created_at DESC);
CREATE TRIGGER go_test_job_bindings_immutable
 BEFORE UPDATE OR DELETE ON go_test_job_bindings
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE go_test_job_results (
 company_id text NOT NULL,
 job_id text NOT NULL,
 outcome text NOT NULL CHECK(outcome IN ('passed','candidate_failed','dependency_unavailable','infrastructure_failed','resource_limit','timed_out','cancelled')),
 result bytea NOT NULL CHECK(octet_length(result) BETWEEN 1 AND 1048576),
 result_sha256 text NOT NULL CHECK(result_sha256 ~ '^[a-f0-9]{64}$'),
 runner_receipt bytea NOT NULL CHECK(octet_length(runner_receipt) BETWEEN 1 AND 1048576),
 runner_receipt_sha256 text NOT NULL CHECK(runner_receipt_sha256 ~ '^[a-f0-9]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,job_id),
 FOREIGN KEY(company_id,job_id) REFERENCES go_test_job_bindings(company_id,job_id)
);
CREATE TRIGGER go_test_job_results_immutable
 BEFORE UPDATE OR DELETE ON go_test_job_results
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM go_test_job_bindings) OR EXISTS (SELECT 1 FROM go_test_job_results) OR
    EXISTS (SELECT 1 FROM job_runs WHERE kind='go_test') OR EXISTS (SELECT 1 FROM job_run_events WHERE state='blocked') OR
    EXISTS (SELECT 1 FROM worker_workspace_roots WHERE source_commit_oid IS NOT NULL OR source_input_manifest_sha256 IS NOT NULL) OR
    EXISTS (SELECT 1 FROM task_takeover_leases WHERE base_tree_source_commit_oid IS NOT NULL OR base_tree_input_manifest_sha256 IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot remove Go JobRun persistence while records exist';
 END IF;
END;
$$;
-- +goose StatementEnd

DROP TABLE go_test_job_results;
DROP TABLE go_test_job_bindings;
ALTER TABLE task_takeover_leases DROP CONSTRAINT task_takeover_leases_git_source_check;
ALTER TABLE task_takeover_leases DROP COLUMN base_tree_source_commit_oid,DROP COLUMN base_tree_input_manifest_sha256;
ALTER TABLE worker_workspace_roots DROP CONSTRAINT worker_workspace_roots_git_source_check;
ALTER TABLE worker_workspace_roots DROP COLUMN source_commit_oid,DROP COLUMN source_input_manifest_sha256;
ALTER TABLE job_run_events DROP CONSTRAINT job_run_events_state_check;
ALTER TABLE job_run_events ADD CONSTRAINT job_run_events_state_check
 CHECK(state IN ('accepted','starting','running','exited','failed','cancelled','outcome_unknown'));
ALTER TABLE job_runs DROP CONSTRAINT job_runs_execution_binding_check;
ALTER TABLE job_runs DROP CONSTRAINT job_runs_company_job_task_unique;
ALTER TABLE job_runs DROP CONSTRAINT job_runs_company_job_kind_unique;
ALTER TABLE job_runs ALTER COLUMN session_id SET NOT NULL;
ALTER TABLE job_runs ALTER COLUMN environment_revision_id SET NOT NULL;
ALTER TABLE job_runs DROP CONSTRAINT job_runs_kind_check;
ALTER TABLE job_runs ADD CONSTRAINT job_runs_kind_check
 CHECK(kind IN ('batch','service','controlled_input'));
