-- +goose Up
ALTER TABLE artifacts DROP CONSTRAINT artifacts_state_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_state_check CHECK(state IN ('ready','missing','corrupt','revoked'));
ALTER TABLE artifacts ADD CONSTRAINT artifacts_revoked_kind_check CHECK(state!='revoked' OR artifact_kind='workspace_snapshot');

CREATE TABLE workspace_snapshot_revocations (
 company_id text NOT NULL,
 artifact_id text NOT NULL,
 workspace_id text NOT NULL,
 workspace_revision bigint NOT NULL CHECK(workspace_revision>0),
 artifact_digest text NOT NULL CHECK(artifact_digest ~ '^[a-f0-9]{64}$'),
 revoked_by text NOT NULL,
 worker_session_id text NOT NULL,
 worker_epoch bigint NOT NULL CHECK(worker_epoch>0),
 request_id text NOT NULL,
 reason text NOT NULL CHECK(octet_length(reason) BETWEEN 1 AND 512 AND btrim(reason)<>''),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,artifact_id),
 UNIQUE(company_id,revoked_by,request_id),
 FOREIGN KEY(company_id,artifact_id) REFERENCES artifacts(company_id,id),
 FOREIGN KEY(company_id,workspace_id) REFERENCES worker_workspace_roots(company_id,id),
 FOREIGN KEY(company_id,revoked_by) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,worker_session_id) REFERENCES worker_sessions(company_id,id)
);

-- +goose StatementBegin
CREATE FUNCTION validate_workspace_snapshot_revocation() RETURNS trigger AS $$
DECLARE
 artifact_task text;
 artifact_author text;
 artifact_digest text;
 artifact_workspace text;
 artifact_state text;
 artifact_verdict text;
 artifact_kind text;
 artifact_revision bigint;
 root_owner text;
 root_session text;
 root_epoch bigint;
 root_task text;
 session_task text;
 session_employee text;
 session_epoch bigint;
 session_state text;
BEGIN
 SELECT a.task_id,a.author,a.digest,a.workspace_id,a.state,a.verdict,a.artifact_kind,a.workspace_revision,
        r.owner,r.writer_session_id,r.writer_epoch,r.task_id
 INTO artifact_task,artifact_author,artifact_digest,artifact_workspace,artifact_state,artifact_verdict,artifact_kind,artifact_revision,
      root_owner,root_session,root_epoch,root_task
 FROM artifacts a JOIN worker_workspace_roots r ON r.company_id=a.company_id AND r.id=a.workspace_id
 WHERE a.company_id=NEW.company_id AND a.id=NEW.artifact_id
 FOR UPDATE OF a,r;
 IF NOT FOUND OR artifact_kind!='workspace_snapshot' OR artifact_state!='ready' OR artifact_verdict!='draft_not_accepted' OR
    artifact_task!=root_task OR artifact_workspace!=NEW.workspace_id OR artifact_revision!=NEW.workspace_revision OR
    artifact_digest!=NEW.artifact_digest OR root_owner!=NEW.revoked_by OR artifact_author!=NEW.revoked_by OR
    root_session!=NEW.worker_session_id OR root_epoch!=NEW.worker_epoch THEN
  RAISE EXCEPTION 'workspace snapshot revocation does not match its active source owner and immutable Artifact';
 END IF;
 SELECT task_id,employee_id,epoch,state INTO session_task,session_employee,session_epoch,session_state
 FROM worker_sessions WHERE company_id=NEW.company_id AND id=NEW.worker_session_id FOR SHARE;
 IF NOT FOUND OR session_state!='active' OR session_task!=root_task OR session_employee!=NEW.revoked_by OR session_epoch!=NEW.worker_epoch THEN
  RAISE EXCEPTION 'workspace snapshot revocation requires its current active owner WorkerSession';
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER workspace_snapshot_revocation_scope
 BEFORE INSERT ON workspace_snapshot_revocations
 FOR EACH ROW EXECUTE FUNCTION validate_workspace_snapshot_revocation();

-- The immutable receipt and Artifact read barrier commit atomically.
-- +goose StatementBegin
CREATE FUNCTION publish_workspace_snapshot_revocation() RETURNS trigger AS $$
BEGIN
 UPDATE artifacts SET state='revoked'
 WHERE company_id=NEW.company_id AND id=NEW.artifact_id AND artifact_kind='workspace_snapshot' AND state='ready';
 IF NOT FOUND THEN RAISE EXCEPTION 'workspace snapshot is no longer ready to revoke'; END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER workspace_snapshot_revocation_publish
 AFTER INSERT ON workspace_snapshot_revocations
 FOR EACH ROW EXECUTE FUNCTION publish_workspace_snapshot_revocation();
CREATE TRIGGER workspace_snapshot_revocations_immutable
 BEFORE UPDATE OR DELETE ON workspace_snapshot_revocations
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- Prevent a direct state edit from fabricating or reversing a revocation.
-- +goose StatementBegin
CREATE FUNCTION enforce_workspace_snapshot_revocation_state() RETURNS trigger AS $$
BEGIN
 IF OLD.state='revoked' THEN
  RAISE EXCEPTION 'revoked workspace snapshot state is immutable';
 END IF;
 IF NEW.state='revoked' AND (OLD.artifact_kind!='workspace_snapshot' OR NOT EXISTS (
  SELECT 1 FROM workspace_snapshot_revocations r WHERE r.company_id=OLD.company_id AND r.artifact_id=OLD.id AND
   r.workspace_id=OLD.workspace_id AND r.workspace_revision=OLD.workspace_revision AND r.artifact_digest=OLD.digest
 )) THEN
  RAISE EXCEPTION 'workspace snapshot revocation requires its immutable audit record';
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER workspace_snapshot_revocation_state_guard
 BEFORE UPDATE ON artifacts
 FOR EACH ROW EXECUTE FUNCTION enforce_workspace_snapshot_revocation_state();

-- +goose Down
-- A revoked Artifact cannot be represented by the previous state constraint;
-- retain the new schema until a separately reviewed restoration exists.
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM workspace_snapshot_revocations) THEN
  RAISE EXCEPTION 'cannot roll back workspace snapshot revocations with recorded history';
 END IF;
END;
$$;
-- +goose StatementEnd
DROP TRIGGER workspace_snapshot_revocation_state_guard ON artifacts;
DROP FUNCTION enforce_workspace_snapshot_revocation_state();
DROP TABLE workspace_snapshot_revocations;
DROP FUNCTION publish_workspace_snapshot_revocation();
DROP FUNCTION validate_workspace_snapshot_revocation();
ALTER TABLE artifacts DROP CONSTRAINT artifacts_revoked_kind_check;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_state_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_state_check CHECK(state IN ('ready','missing','corrupt'));
