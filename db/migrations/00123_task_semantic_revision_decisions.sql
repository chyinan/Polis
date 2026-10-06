-- +goose Up
-- Owner decisions are append-only and do not change the unverified execution
-- qualification stored on task_semantic_revisions.
ALTER TABLE task_semantic_revisions
 ADD CONSTRAINT task_semantic_revisions_identity_unique
 UNIQUE(company_id,task_id,revision,binding_sha256);
CREATE TABLE task_semantic_revision_decisions (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL,
 task_id text NOT NULL,
 revision bigint NOT NULL,
 binding_sha256 text NOT NULL CHECK(binding_sha256 ~ '^[a-f0-9]{64}$'),
 decision text NOT NULL CHECK(decision IN ('approved','rejected','revoked')),
 rationale text NOT NULL CHECK(octet_length(btrim(rationale)) BETWEEN 1 AND 512),
 actor text NOT NULL CHECK(actor='local-owner'),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,task_id,revision,binding_sha256) REFERENCES task_semantic_revisions(company_id,task_id,revision,binding_sha256)
);
CREATE INDEX task_semantic_revision_decisions_latest ON task_semantic_revision_decisions(company_id,task_id,revision,event_seq DESC);
-- +goose StatementBegin
CREATE FUNCTION validate_task_semantic_revision_decision() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
 current_decision text;
BEGIN
 PERFORM 1 FROM task_semantic_revisions
 WHERE company_id=NEW.company_id AND task_id=NEW.task_id AND revision=NEW.revision AND binding_sha256=NEW.binding_sha256
 FOR UPDATE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'task semantic revision decision target is missing' USING ERRCODE='23503';
 END IF;
 SELECT decision INTO current_decision
 FROM task_semantic_revision_decisions
 WHERE company_id=NEW.company_id AND task_id=NEW.task_id AND revision=NEW.revision AND binding_sha256=NEW.binding_sha256
 ORDER BY event_seq DESC LIMIT 1;
 IF (current_decision IS NULL AND NEW.decision NOT IN ('approved','rejected')) OR
    (current_decision='approved' AND NEW.decision<>'revoked') OR
    (current_decision IS NOT NULL AND current_decision<>'approved' AND NEW.decision IS NOT NULL) THEN
  RAISE EXCEPTION 'task semantic revision decision is not a valid monotonic transition' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER task_semantic_revision_decisions_provenance
 BEFORE INSERT ON task_semantic_revision_decisions
 FOR EACH ROW EXECUTE FUNCTION validate_task_semantic_revision_decision();
CREATE TRIGGER task_semantic_revision_decisions_immutable
 BEFORE UPDATE OR DELETE ON task_semantic_revision_decisions
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER task_semantic_revision_decisions_no_truncate
 BEFORE TRUNCATE ON task_semantic_revision_decisions FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM task_semantic_revision_decisions) THEN
  RAISE EXCEPTION 'cannot discard task semantic revision decisions';
 END IF;
END
$$;
DROP TRIGGER task_semantic_revision_decisions_no_truncate ON task_semantic_revision_decisions;
DROP TRIGGER task_semantic_revision_decisions_immutable ON task_semantic_revision_decisions;
DROP TRIGGER task_semantic_revision_decisions_provenance ON task_semantic_revision_decisions;
DROP FUNCTION validate_task_semantic_revision_decision();
DROP INDEX task_semantic_revision_decisions_latest;
DROP TABLE task_semantic_revision_decisions;
ALTER TABLE task_semantic_revisions DROP CONSTRAINT task_semantic_revisions_identity_unique;
