-- +goose Up
ALTER TABLE worker_sessions ADD COLUMN execution_mode text NOT NULL DEFAULT 'provider'
 CHECK(execution_mode IN ('provider','project_job'));
ALTER TABLE worker_sessions ADD CONSTRAINT worker_sessions_task_identity UNIQUE(company_id,id,task_id);
ALTER TABLE job_runs ADD CONSTRAINT job_runs_handover_source_identity UNIQUE(company_id,job_id,task_id,session_id,environment_revision_id);
ALTER TABLE project_environment_revisions ADD CONSTRAINT project_environment_revisions_handover_identity UNIQUE(company_id,revision_id,mission_id,profile_id);
ALTER TABLE task_input_manifests ADD CONSTRAINT task_input_manifests_mission_identity UNIQUE(company_id,task_id,mission_id);

CREATE TABLE cross_backend_handovers (
 company_id text NOT NULL,
 handover_id text NOT NULL,
 mission_id text NOT NULL,
 task_id text NOT NULL,
 source_job_id text NOT NULL,
 source_session_id text NOT NULL,
 source_runtime_incarnation text NOT NULL CHECK(octet_length(source_runtime_incarnation) BETWEEN 1 AND 128),
 source_environment_revision_id text NOT NULL,
 source_profile_id text NOT NULL CHECK(source_profile_id IN ('windows-node-npm@1','linux-node-npm@1')),
 target_environment_revision_id text NOT NULL,
 target_profile_id text NOT NULL CHECK(target_profile_id IN ('windows-node-npm@1','linux-node-npm@1')),
 project_source_sha256 text NOT NULL CHECK(project_source_sha256 ~ '^[a-f0-9]{64}$'),
 package_json_sha256 text NOT NULL CHECK(package_json_sha256 ~ '^[a-f0-9]{64}$'),
 lockfile_sha256 text NOT NULL CHECK(lockfile_sha256 ~ '^[a-f0-9]{64}$'),
 workspace_digest text NOT NULL CHECK(workspace_digest ~ '^[a-f0-9]{64}$'),
 workspace_revision bigint NOT NULL CHECK(workspace_revision>0),
 task_input_manifest_sha256 text NOT NULL CHECK(task_input_manifest_sha256 ~ '^[a-f0-9]{64}$'),
 target_policy_sha256 text NOT NULL CHECK(target_policy_sha256 ~ '^[a-f0-9]{64}$'),
 target_toolchain_sha256 text NOT NULL CHECK(target_toolchain_sha256 ~ '^[a-f0-9]{64}$'),
 request_id text NOT NULL,
 record_sha256 text NOT NULL CHECK(record_sha256 ~ '^[a-f0-9]{64}$'),
 created_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,handover_id),
 UNIQUE(company_id,request_id),
 UNIQUE(company_id,source_job_id),
 UNIQUE(company_id,handover_id,task_id,target_environment_revision_id),
 FOREIGN KEY(company_id,task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,source_job_id,task_id,source_session_id,source_environment_revision_id) REFERENCES job_runs(company_id,job_id,task_id,session_id,environment_revision_id),
 FOREIGN KEY(company_id,source_session_id,task_id) REFERENCES worker_sessions(company_id,id,task_id),
 FOREIGN KEY(company_id,source_environment_revision_id,mission_id,source_profile_id) REFERENCES project_environment_revisions(company_id,revision_id,mission_id,profile_id),
 FOREIGN KEY(company_id,target_environment_revision_id,mission_id,target_profile_id) REFERENCES project_environment_revisions(company_id,revision_id,mission_id,profile_id),
 FOREIGN KEY(company_id,task_id,mission_id) REFERENCES task_input_manifests(company_id,task_id,mission_id),
 CHECK((source_profile_id='windows-node-npm@1' AND target_profile_id='linux-node-npm@1') OR (source_profile_id='linux-node-npm@1' AND target_profile_id='windows-node-npm@1'))
);
CREATE INDEX cross_backend_handovers_task_created ON cross_backend_handovers(company_id,task_id,created_at DESC,handover_id);
CREATE TRIGGER cross_backend_handovers_immutable
 BEFORE UPDATE OR DELETE ON cross_backend_handovers
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

ALTER TABLE job_runs ADD COLUMN handover_id text;
ALTER TABLE job_runs ADD CONSTRAINT job_runs_cross_backend_handover_fk
 FOREIGN KEY(company_id,handover_id,task_id,environment_revision_id)
 REFERENCES cross_backend_handovers(company_id,handover_id,task_id,target_environment_revision_id);
CREATE UNIQUE INDEX job_runs_handover_once ON job_runs(company_id,handover_id) WHERE handover_id IS NOT NULL;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM cross_backend_handovers)
    OR EXISTS (SELECT 1 FROM job_runs WHERE handover_id IS NOT NULL)
    OR EXISTS (SELECT 1 FROM worker_sessions WHERE execution_mode='project_job') THEN
  RAISE EXCEPTION 'cannot roll back persisted cross-backend handovers or project-job sessions';
 END IF;
END
$$;
-- +goose StatementEnd
DROP INDEX job_runs_handover_once;
ALTER TABLE job_runs DROP CONSTRAINT job_runs_cross_backend_handover_fk;
ALTER TABLE job_runs DROP COLUMN handover_id;
DROP TRIGGER cross_backend_handovers_immutable ON cross_backend_handovers;
DROP INDEX cross_backend_handovers_task_created;
DROP TABLE cross_backend_handovers;
ALTER TABLE task_input_manifests DROP CONSTRAINT task_input_manifests_mission_identity;
ALTER TABLE project_environment_revisions DROP CONSTRAINT project_environment_revisions_handover_identity;
ALTER TABLE job_runs DROP CONSTRAINT job_runs_handover_source_identity;
ALTER TABLE worker_sessions DROP CONSTRAINT worker_sessions_task_identity;
ALTER TABLE worker_sessions DROP COLUMN execution_mode;
