-- +goose Up
CREATE TABLE migration_execution_evidence (
    version_id BIGINT PRIMARY KEY CHECK (version_id > 0),
    migration_sha256 TEXT NOT NULL CHECK (migration_sha256 ~ '^[a-f0-9]{64}$'),
    recorded_by_build_id TEXT NOT NULL CHECK (length(recorded_by_build_id) BETWEEN 1 AND 512),
    execution_result TEXT NOT NULL CHECK (execution_result IN ('applied_during_this_run','observed_preexisting')),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- +goose StatementBegin
CREATE FUNCTION guard_migration_execution_evidence_insert() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    table_owner_oid OID;
BEGIN
    SELECT relowner INTO table_owner_oid FROM pg_class WHERE oid=TG_RELID;
    IF NOT pg_has_role(current_user, table_owner_oid, 'USAGE') THEN
        RAISE EXCEPTION 'migration execution evidence can only be recorded by the table owner';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION reject_migration_execution_evidence_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'migration execution evidence is append-only';
END $$;
-- +goose StatementEnd

CREATE TRIGGER migration_execution_evidence_owner_insert
    BEFORE INSERT ON migration_execution_evidence
    FOR EACH ROW EXECUTE FUNCTION guard_migration_execution_evidence_insert();
CREATE TRIGGER migration_execution_evidence_no_update_delete
    BEFORE UPDATE OR DELETE ON migration_execution_evidence
    FOR EACH ROW EXECUTE FUNCTION reject_migration_execution_evidence_mutation();
CREATE TRIGGER migration_execution_evidence_no_truncate
    BEFORE TRUNCATE ON migration_execution_evidence
    FOR EACH STATEMENT EXECUTE FUNCTION reject_migration_execution_evidence_mutation();

-- +goose Down
DO $$ BEGIN
    RAISE EXCEPTION 'cannot roll back migration execution evidence; restore a verified pre-upgrade backup instead';
END $$;
