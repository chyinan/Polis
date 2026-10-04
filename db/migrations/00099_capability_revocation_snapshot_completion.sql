-- +goose Up
-- A completion row distinguishes an exact empty revoke-time snapshot from a
-- legacy revocation for which only recorded-use rows are still available.
CREATE TABLE capability_revocation_snapshot_completions (
 company_id text NOT NULL REFERENCES companies(id),
 revocation_id text NOT NULL,
 scope text NOT NULL CHECK(scope IN ('capability','employee')),
 capability_kind text NOT NULL CHECK(capability_kind IN ('skill','mcp')),
 capability_id text NOT NULL,
 version_digest text NOT NULL CHECK(version_digest ~ '^[a-f0-9]{64}$'),
 employee_id text NOT NULL,
 session_count bigint NOT NULL CHECK(session_count>=0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,revocation_id),
 CHECK((scope='capability' AND employee_id='') OR (scope='employee' AND employee_id<>''))
);
CREATE TRIGGER capability_revocation_snapshot_completions_immutable
 BEFORE UPDATE OR DELETE ON capability_revocation_snapshot_completions
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER capability_revocation_snapshot_completions_no_truncate
 BEFORE TRUNCATE ON capability_revocation_snapshot_completions
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM capability_revocation_snapshot_completions) THEN
  RAISE EXCEPTION 'cannot roll back capability revocation snapshot completion receipts';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER capability_revocation_snapshot_completions_no_truncate ON capability_revocation_snapshot_completions;
DROP TRIGGER capability_revocation_snapshot_completions_immutable ON capability_revocation_snapshot_completions;
DROP TABLE capability_revocation_snapshot_completions;
