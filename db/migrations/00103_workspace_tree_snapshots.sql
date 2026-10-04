-- +goose Up
ALTER TABLE artifacts ADD COLUMN artifact_kind text NOT NULL DEFAULT 'deliverable';
ALTER TABLE artifacts ADD COLUMN workspace_id text;
ALTER TABLE artifacts ADD COLUMN workspace_revision bigint;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_company_id_task_id_key;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_bytes_check;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_verdict_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_kind_check CHECK (artifact_kind IN ('deliverable','workspace_snapshot'));
ALTER TABLE artifacts ADD CONSTRAINT artifacts_bytes_check CHECK (
 (artifact_kind='deliverable' AND bytes BETWEEN 1 AND 4096) OR
 (artifact_kind='workspace_snapshot' AND bytes BETWEEN 1 AND 8388608));
ALTER TABLE artifacts ADD CONSTRAINT artifacts_verdict_check CHECK (
 verdict IN ('candidate','passed','failed','invalidated','draft_not_accepted'));
ALTER TABLE artifacts ADD CONSTRAINT artifacts_workspace_snapshot_binding_check CHECK (
 (artifact_kind='deliverable' AND workspace_id IS NULL AND workspace_revision IS NULL AND verdict!='draft_not_accepted') OR
 (artifact_kind='workspace_snapshot' AND workspace_id IS NOT NULL AND workspace_revision IS NOT NULL AND workspace_revision>0 AND verdict='draft_not_accepted'));
CREATE UNIQUE INDEX one_deliverable_artifact_per_task ON artifacts(company_id,task_id) WHERE artifact_kind='deliverable';

ALTER TABLE artifact_staging ADD COLUMN artifact_kind text NOT NULL DEFAULT 'deliverable';
ALTER TABLE artifact_staging ADD COLUMN workspace_id text;
ALTER TABLE artifact_staging ADD COLUMN workspace_revision bigint;
ALTER TABLE artifact_staging DROP CONSTRAINT artifact_staging_company_id_task_id_key;
ALTER TABLE artifact_staging ADD CONSTRAINT artifact_staging_kind_check CHECK (
 (artifact_kind='deliverable' AND workspace_id IS NULL AND workspace_revision IS NULL) OR
 (artifact_kind='workspace_snapshot' AND workspace_id IS NOT NULL AND workspace_revision IS NOT NULL AND workspace_revision>0));
CREATE UNIQUE INDEX one_deliverable_stage_per_task ON artifact_staging(company_id,task_id) WHERE artifact_kind='deliverable';

CREATE TABLE worker_workspace_roots (
 company_id text NOT NULL,
 id text NOT NULL,
 task_id text NOT NULL,
 mission_id text NOT NULL,
 owner text NOT NULL,
 read_write_class text NOT NULL CHECK(read_write_class='task_private'),
 revision bigint NOT NULL CHECK(revision>0),
 writer_session_id text NOT NULL,
 writer_epoch bigint NOT NULL CHECK(writer_epoch>0),
 PRIMARY KEY(company_id,id),
 UNIQUE(company_id,task_id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,owner) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,writer_session_id) REFERENCES worker_sessions(company_id,id)
);
CREATE TABLE worker_workspace_files (
 company_id text NOT NULL,
 workspace_id text NOT NULL,
 relative_path text NOT NULL CHECK(octet_length(relative_path) BETWEEN 1 AND 1024),
 digest text NOT NULL CHECK(digest ~ '^[a-f0-9]{64}$'),
 bytes bigint NOT NULL CHECK(bytes BETWEEN 1 AND 2097152),
 file_revision bigint NOT NULL CHECK(file_revision>0),
 source_revision bigint NOT NULL CHECK(source_revision>0),
 content_type text NOT NULL CHECK(content_type='text/utf-8'),
 PRIMARY KEY(company_id,workspace_id,relative_path),
 FOREIGN KEY(company_id,workspace_id) REFERENCES worker_workspace_roots(company_id,id)
);
CREATE INDEX worker_workspace_files_page ON worker_workspace_files(company_id,workspace_id,relative_path);

CREATE TABLE workspace_snapshot_files (
 company_id text NOT NULL,
 artifact_id text NOT NULL,
 workspace_id text NOT NULL,
 relative_path text NOT NULL,
 digest text NOT NULL CHECK(digest ~ '^[a-f0-9]{64}$'),
 bytes bigint NOT NULL CHECK(bytes BETWEEN 1 AND 2097152),
 file_revision bigint NOT NULL CHECK(file_revision>0),
 PRIMARY KEY(company_id,artifact_id,relative_path),
 FOREIGN KEY(company_id,artifact_id) REFERENCES artifacts(company_id,id),
 FOREIGN KEY(company_id,workspace_id) REFERENCES worker_workspace_roots(company_id,id)
);

-- +goose Down
DROP TABLE workspace_snapshot_files,worker_workspace_files,worker_workspace_roots;
DROP INDEX one_deliverable_stage_per_task;
ALTER TABLE artifact_staging DROP CONSTRAINT artifact_staging_kind_check;
ALTER TABLE artifact_staging DROP COLUMN workspace_revision,DROP COLUMN workspace_id,DROP COLUMN artifact_kind;
ALTER TABLE artifact_staging ADD CONSTRAINT artifact_staging_company_id_task_id_key UNIQUE(company_id,task_id);
DROP INDEX one_deliverable_artifact_per_task;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_workspace_snapshot_binding_check;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_verdict_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_verdict_check CHECK(verdict IN ('candidate','passed','failed','invalidated'));
ALTER TABLE artifacts DROP CONSTRAINT artifacts_bytes_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_bytes_check CHECK(bytes BETWEEN 1 AND 4096);
ALTER TABLE artifacts DROP CONSTRAINT artifacts_kind_check;
ALTER TABLE artifacts DROP COLUMN workspace_revision,DROP COLUMN workspace_id,DROP COLUMN artifact_kind;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_company_id_task_id_key UNIQUE(company_id,task_id);
