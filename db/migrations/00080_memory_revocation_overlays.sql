-- +goose Up
-- A small database mirror accelerates reads; the append-only overlay files
-- outside each recovery generation remain authoritative across old restores.
CREATE TABLE memory_record_revocation_overlays (
 company_id text NOT NULL,
 record_id text NOT NULL,
 operation_id text NOT NULL,
 content_sha256 text NOT NULL CHECK(content_sha256 ~ '^[0-9a-f]{64}$'),
 source_revision bigint NOT NULL CHECK(source_revision>0),
 reason_code text NOT NULL CHECK(reason_code IN ('incorrect','sensitive','requested','other')),
 created_at timestamptz NOT NULL,
 PRIMARY KEY(company_id,record_id),
 UNIQUE(company_id,operation_id),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);
CREATE TRIGGER memory_record_revocation_overlays_immutable
 BEFORE UPDATE OR DELETE ON memory_record_revocation_overlays
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER memory_record_revocation_overlays_no_truncate
 BEFORE TRUNCATE ON memory_record_revocation_overlays
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

ALTER TABLE memory_task_state_events DROP CONSTRAINT memory_task_state_events_cause_kind_check;
ALTER TABLE memory_task_state_events ADD CONSTRAINT memory_task_state_events_cause_kind_check
 CHECK(cause_kind IN ('memory_correction','revalidation','policy_change','source_revoked'));

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 RAISE EXCEPTION 'cannot roll back memory revocation overlays; authoritative tombstones may remain outside this database generation';
END $$;
-- +goose StatementEnd
