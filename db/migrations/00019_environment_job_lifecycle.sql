-- +goose Up
CREATE TABLE project_environment_revisions (
 company_id text NOT NULL,
 revision_id text NOT NULL,
 profile_id text NOT NULL CHECK(profile_id IN ('windows-node-npm@1')),
 source_revision_sha256 text NOT NULL CHECK(source_revision_sha256 ~ '^[a-f0-9]{64}$'),
 package_json_sha256 text NOT NULL CHECK(package_json_sha256 ~ '^[a-f0-9]{64}$'),
 lockfile_sha256 text NOT NULL CHECK(lockfile_sha256 ~ '^[a-f0-9]{64}$'),
 policy_sha256 text NOT NULL CHECK(policy_sha256 ~ '^[a-f0-9]{64}$'),
 toolchain_sha256 text NOT NULL CHECK(toolchain_sha256 ~ '^[a-f0-9]{64}$'),
 created_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,revision_id),
 UNIQUE(company_id,profile_id,source_revision_sha256,lockfile_sha256,policy_sha256,toolchain_sha256),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);
CREATE TRIGGER project_environment_revisions_immutable
 BEFORE UPDATE OR DELETE ON project_environment_revisions
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE environment_policy_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL UNIQUE,
 event_id text NOT NULL,
 revision_id text NOT NULL,
 policy_sha256 text NOT NULL CHECK(policy_sha256 ~ '^[a-f0-9]{64}$'),
 decision text NOT NULL CHECK(decision IN ('approved','revoked')),
 rationale text NOT NULL CHECK(octet_length(rationale) BETWEEN 1 AND 512),
 actor text NOT NULL,
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,revision_id) REFERENCES project_environment_revisions(company_id,revision_id)
);
CREATE INDEX environment_policy_events_current ON environment_policy_events(company_id,revision_id,event_seq DESC);
CREATE TRIGGER environment_policy_events_immutable
 BEFORE UPDATE OR DELETE ON environment_policy_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE environment_executor_qualification_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL UNIQUE,
 event_id text NOT NULL,
 profile_id text NOT NULL CHECK(profile_id IN ('windows-node-npm@1')),
 executor_fingerprint_sha256 text NOT NULL CHECK(executor_fingerprint_sha256 ~ '^[a-f0-9]{64}$'),
 isolation_profile text NOT NULL,
 decision text NOT NULL CHECK(decision IN ('qualified','revoked')),
 evidence_sha256 text NOT NULL CHECK(evidence_sha256 ~ '^[a-f0-9]{64}$'),
 qualified_until timestamptz,
 actor text NOT NULL,
 rationale text NOT NULL CHECK(octet_length(rationale) BETWEEN 1 AND 512),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);
CREATE INDEX environment_executor_qualification_current ON environment_executor_qualification_events(company_id,profile_id,event_seq DESC);
CREATE TRIGGER environment_executor_qualification_events_immutable
 BEFORE UPDATE OR DELETE ON environment_executor_qualification_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE environment_preparation_runs (
 company_id text NOT NULL,
 run_id text NOT NULL,
 revision_id text NOT NULL,
 request_id text NOT NULL,
 created_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,run_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,revision_id) REFERENCES project_environment_revisions(company_id,revision_id)
);
CREATE TRIGGER environment_preparation_runs_immutable
 BEFORE UPDATE OR DELETE ON environment_preparation_runs
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE environment_preparation_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL UNIQUE,
 event_id text NOT NULL,
 run_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('blocked_policy','blocked_unqualified','accepted','starting','running','ready','failed','cancelled','outcome_unknown')),
 reason_code text NOT NULL,
 evidence_sha256 text CHECK(evidence_sha256 IS NULL OR evidence_sha256 ~ '^[a-f0-9]{64}$'),
 log_sha256 text CHECK(log_sha256 IS NULL OR log_sha256 ~ '^[a-f0-9]{64}$'),
 log_bytes integer NOT NULL DEFAULT 0 CHECK(log_bytes BETWEEN 0 AND 65536),
 log_truncated boolean NOT NULL DEFAULT false,
 log_gap boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 FOREIGN KEY(company_id,run_id) REFERENCES environment_preparation_runs(company_id,run_id)
);
CREATE INDEX environment_preparation_events_latest ON environment_preparation_events(company_id,run_id,event_seq DESC);
CREATE TRIGGER environment_preparation_events_immutable
 BEFORE UPDATE OR DELETE ON environment_preparation_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE job_runs (
 company_id text NOT NULL,
 job_id text NOT NULL,
 task_id text NOT NULL,
 session_id text NOT NULL,
 environment_revision_id text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('batch','service','controlled_input')),
 source_revision_sha256 text NOT NULL CHECK(source_revision_sha256 ~ '^[a-f0-9]{64}$'),
 argv_sha256 text NOT NULL CHECK(argv_sha256 ~ '^[a-f0-9]{64}$'),
 working_directory_sha256 text NOT NULL CHECK(working_directory_sha256 ~ '^[a-f0-9]{64}$'),
 network_policy_sha256 text NOT NULL CHECK(network_policy_sha256 ~ '^[a-f0-9]{64}$'),
 environment_allowlist jsonb NOT NULL CHECK(jsonb_typeof(environment_allowlist)='array'),
 timeout_ms integer NOT NULL CHECK(timeout_ms BETWEEN 1 AND 3600000),
 output_limit_bytes integer NOT NULL CHECK(output_limit_bytes BETWEEN 1 AND 1048576),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,job_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,session_id) REFERENCES worker_sessions(company_id,id),
 FOREIGN KEY(company_id,environment_revision_id) REFERENCES project_environment_revisions(company_id,revision_id)
);
CREATE INDEX job_runs_task_created ON job_runs(company_id,task_id,created_at DESC);
CREATE TRIGGER job_runs_immutable
 BEFORE UPDATE OR DELETE ON job_runs
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE job_run_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL UNIQUE,
 event_id text NOT NULL,
 job_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('accepted','starting','running','exited','failed','cancelled','outcome_unknown')),
 readiness text NOT NULL CHECK(readiness IN ('not_applicable','not_ready','ready','unhealthy')),
 exit_code integer,
 reason_code text NOT NULL,
 stdout_offset bigint NOT NULL DEFAULT 0 CHECK(stdout_offset>=0),
 stderr_offset bigint NOT NULL DEFAULT 0 CHECK(stderr_offset>=0),
 stdout_bytes integer NOT NULL DEFAULT 0 CHECK(stdout_bytes BETWEEN 0 AND 1048576),
 stderr_bytes integer NOT NULL DEFAULT 0 CHECK(stderr_bytes BETWEEN 0 AND 1048576),
 logs_truncated boolean NOT NULL DEFAULT false,
 log_gap boolean NOT NULL DEFAULT false,
 log_manifest_sha256 text CHECK(log_manifest_sha256 IS NULL OR log_manifest_sha256 ~ '^[a-f0-9]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 FOREIGN KEY(company_id,job_id) REFERENCES job_runs(company_id,job_id)
);
CREATE INDEX job_run_events_latest ON job_run_events(company_id,job_id,event_seq DESC);
CREATE TRIGGER job_run_events_immutable
 BEFORE UPDATE OR DELETE ON job_run_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE service_endpoint_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL UNIQUE,
 event_id text NOT NULL,
 job_id text NOT NULL,
 generation integer NOT NULL CHECK(generation>0),
 bind_address text NOT NULL CHECK(bind_address IN ('127.0.0.1','::1','localhost')),
 port integer NOT NULL CHECK(port BETWEEN 1 AND 65535),
 readiness text NOT NULL CHECK(readiness IN ('not_ready','ready','unhealthy','revoked')),
 source_revision_sha256 text NOT NULL CHECK(source_revision_sha256 ~ '^[a-f0-9]{64}$'),
 healthcheck_sha256 text NOT NULL CHECK(healthcheck_sha256 ~ '^[a-f0-9]{64}$'),
 lease_expires_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 FOREIGN KEY(company_id,job_id) REFERENCES job_runs(company_id,job_id)
);
CREATE INDEX service_endpoint_events_current ON service_endpoint_events(company_id,job_id,generation DESC,event_seq DESC);
CREATE TRIGGER service_endpoint_events_immutable
 BEFORE UPDATE OR DELETE ON service_endpoint_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DROP TRIGGER service_endpoint_events_immutable ON service_endpoint_events;
DROP INDEX service_endpoint_events_current;
DROP TABLE service_endpoint_events;
DROP TRIGGER job_run_events_immutable ON job_run_events;
DROP INDEX job_run_events_latest;
DROP TABLE job_run_events;
DROP TRIGGER job_runs_immutable ON job_runs;
DROP INDEX job_runs_task_created;
DROP TABLE job_runs;
DROP TRIGGER environment_preparation_events_immutable ON environment_preparation_events;
DROP INDEX environment_preparation_events_latest;
DROP TABLE environment_preparation_events;
DROP TRIGGER environment_preparation_runs_immutable ON environment_preparation_runs;
DROP TABLE environment_preparation_runs;
DROP TRIGGER environment_executor_qualification_events_immutable ON environment_executor_qualification_events;
DROP INDEX environment_executor_qualification_current;
DROP TABLE environment_executor_qualification_events;
DROP TRIGGER environment_policy_events_immutable ON environment_policy_events;
DROP INDEX environment_policy_events_current;
DROP TABLE environment_policy_events;
DROP TRIGGER project_environment_revisions_immutable ON project_environment_revisions;
DROP TABLE project_environment_revisions;
