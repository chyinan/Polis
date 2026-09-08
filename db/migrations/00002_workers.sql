-- +goose Up
ALTER TABLE missions DROP CONSTRAINT missions_contract_check;
ALTER TABLE missions ADD CHECK(contract IN ('r0-arithmetic@1','signed-zero@1'));
ALTER TABLE artifacts DROP CONSTRAINT artifacts_contract_check;
ALTER TABLE artifacts ADD CHECK(contract IN ('r0-arithmetic@1','signed-zero@1'));
ALTER TABLE tasks DROP CONSTRAINT tasks_kind_check;
ALTER TABLE tasks ADD CHECK(kind IN ('bootstrap_plan','compute','compat','review'));
CREATE TABLE worker_sessions (
 company_id text NOT NULL,
 id text NOT NULL,
 employee_id text NOT NULL,
 task_id text NOT NULL,
 generation bigint NOT NULL,
 epoch bigint NOT NULL,
 incarnation text NOT NULL,
 profile text NOT NULL,
 state text NOT NULL CHECK(state IN ('restoring','validating','activation_pending_environment','active','stopping','stopped','reconcile_required')),
 capability_digest text,
 process_pid bigint,
 stop_receipt text,
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 CHECK(state!='stopped' OR stop_receipt IS NOT NULL)
);
CREATE UNIQUE INDEX one_live_worker ON worker_sessions(company_id,employee_id) WHERE state!='stopped';
CREATE TABLE worker_workspaces (
 company_id text NOT NULL,
 task_id text NOT NULL,
 digest text NOT NULL CHECK(digest ~ '^[a-f0-9]{64}$'),
 revision bigint NOT NULL DEFAULT 1,
 PRIMARY KEY(company_id,task_id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id)
);
CREATE TABLE worker_checks (
 company_id text NOT NULL,
 id text NOT NULL,
 session_id text NOT NULL,
 digest text NOT NULL,
 phase text NOT NULL CHECK(phase IN ('single','full')),
 passed boolean NOT NULL,
 report jsonb NOT NULL,
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id,session_id) REFERENCES worker_sessions(company_id,id)
);
CREATE TABLE worker_checkpoints (
 company_id text NOT NULL,
 id text NOT NULL,
 session_id text NOT NULL,
 digest text NOT NULL,
 data jsonb NOT NULL,
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id,session_id) REFERENCES worker_sessions(company_id,id)
);
CREATE TABLE worker_observations (
 company_id text NOT NULL,
 id text NOT NULL,
 session_id text NOT NULL,
 reason text NOT NULL,
 data jsonb NOT NULL,
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id,session_id) REFERENCES worker_sessions(company_id,id)
);
-- +goose Down
DROP TABLE worker_observations,worker_checkpoints,worker_checks,worker_workspaces,worker_sessions;
-- R0.1 records cannot be silently converted into arithmetic tasks. Downgrade
-- requires a clean R0 database; these constraints intentionally reject live R0.1 data.
ALTER TABLE tasks DROP CONSTRAINT tasks_kind_check;
ALTER TABLE tasks ADD CHECK(kind IN ('bootstrap_plan','compute'));
ALTER TABLE missions DROP CONSTRAINT missions_contract_check;
ALTER TABLE missions ADD CHECK(contract='r0-arithmetic@1');
ALTER TABLE artifacts DROP CONSTRAINT artifacts_contract_check;
ALTER TABLE artifacts ADD CHECK(contract='r0-arithmetic@1');
