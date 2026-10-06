-- +goose Up
-- Semantic TaskRevision rows bind an explicit task contract to the exact
-- owner-confirmed RoleRevision. They are context only: unverified rows never
-- grant Worker or Provider execution.
CREATE TABLE task_semantic_revisions (
 company_id text NOT NULL,
 task_id text NOT NULL,
 revision bigint NOT NULL CHECK(revision>0),
 binding_sha256 text NOT NULL CHECK(binding_sha256 ~ '^[a-f0-9]{64}$'),
 role_revision_sha256 text NOT NULL CHECK(role_revision_sha256 ~ '^[a-f0-9]{64}$'),
 task_type text NOT NULL CHECK(task_type IN ('planning','frontend','backend','environment_plan','regression_test_overlay','design_change','delivery_assembly')),
 task_kind text NOT NULL CHECK(task_kind IN ('bootstrap_plan','compute','compat','review','peer_backend','peer_frontend','peer_review')),
 owner_employee_id text NOT NULL,
 qualification text NOT NULL CHECK(qualification='unverified'),
 requires_human boolean NOT NULL CHECK(requires_human),
 reason_code text NOT NULL CHECK(reason_code='task_revision_unqualified'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,task_id,revision),
 UNIQUE(company_id,task_id,binding_sha256),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,owner_employee_id,role_revision_sha256) REFERENCES employee_role_revisions(company_id,employee_id,revision_sha256)
);
CREATE INDEX task_semantic_revisions_current ON task_semantic_revisions(company_id,task_id,revision DESC);
-- Database writers must preserve the same Task and RoleRevision scope as the
-- Kernel writer; the foreign keys alone do not enforce owner/kind alignment.
-- +goose StatementBegin
CREATE FUNCTION validate_task_semantic_revision_provenance() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
 task_owner text;
 task_kind text;
 role_task_types jsonb;
 role_task_kinds jsonb;
 mapping_valid boolean;
BEGIN
 SELECT owner,kind INTO task_owner,task_kind
 FROM tasks WHERE company_id=NEW.company_id AND id=NEW.task_id;
 IF task_owner IS NULL OR task_owner IS DISTINCT FROM NEW.owner_employee_id OR task_kind IS DISTINCT FROM NEW.task_kind THEN
  RAISE EXCEPTION 'task semantic revision is outside the Task owner/kind scope' USING ERRCODE='23514';
 END IF;
 SELECT task_types,task_kinds INTO role_task_types,role_task_kinds
 FROM employee_role_revisions
 WHERE company_id=NEW.company_id AND employee_id=NEW.owner_employee_id AND revision_sha256=NEW.role_revision_sha256;
 SELECT EXISTS(
  SELECT 1
  FROM jsonb_array_elements_text(role_task_types) WITH ORDINALITY t(value,ordinal)
  JOIN jsonb_array_elements_text(role_task_kinds) WITH ORDINALITY k(value,ordinal) ON k.ordinal=t.ordinal
  WHERE t.value=NEW.task_type AND k.value=NEW.task_kind
 ) INTO mapping_valid;
 IF NOT mapping_valid THEN
  RAISE EXCEPTION 'task semantic revision does not match RoleRevision mapping' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER task_semantic_revisions_provenance
 BEFORE INSERT ON task_semantic_revisions
 FOR EACH ROW EXECUTE FUNCTION validate_task_semantic_revision_provenance();
CREATE TRIGGER task_semantic_revisions_immutable
 BEFORE UPDATE OR DELETE ON task_semantic_revisions
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER task_semantic_revisions_no_truncate
 BEFORE TRUNCATE ON task_semantic_revisions FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM task_semantic_revisions) THEN
  RAISE EXCEPTION 'cannot discard task semantic revision history';
 END IF;
END
$$;
DROP TRIGGER task_semantic_revisions_no_truncate ON task_semantic_revisions;
DROP TRIGGER task_semantic_revisions_immutable ON task_semantic_revisions;
DROP TRIGGER task_semantic_revisions_provenance ON task_semantic_revisions;
DROP FUNCTION validate_task_semantic_revision_provenance();
DROP INDEX task_semantic_revisions_current;
DROP TABLE task_semantic_revisions;
