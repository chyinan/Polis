-- +goose Up
CREATE TABLE worker_provider_account_identity_snapshots (
 company_id text NOT NULL,
 worker_session_id text NOT NULL,
 snapshot_schema_version text NOT NULL CHECK(snapshot_schema_version='provider-account-identity@1'),
 provider_class text NOT NULL CHECK(provider_class ~ '^[A-Za-z0-9_-]{1,80}$'),
 identity_status text NOT NULL CHECK(identity_status IN ('available','unavailable','unsupported')),
 account_fingerprint text,
 reason_code text,
 captured_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,worker_session_id),
 FOREIGN KEY(company_id,worker_session_id) REFERENCES worker_sessions(company_id,id),
 CHECK((identity_status='available' AND account_fingerprint IS NOT NULL AND account_fingerprint ~ '^[a-f0-9]{64}$' AND reason_code IS NULL) OR
       (identity_status IN ('unavailable','unsupported') AND account_fingerprint IS NULL AND reason_code IS NOT NULL AND reason_code ~ '^[A-Za-z0-9_-]{1,80}$'))
);

-- A session binds its opaque account locator once, before provider execution.
-- It is not a validated billing scope or a financial budget key.
-- +goose StatementBegin
CREATE FUNCTION validate_worker_provider_account_identity_snapshot() RETURNS trigger AS $$
DECLARE session_state text;
BEGIN
 SELECT state INTO session_state FROM worker_sessions
 WHERE company_id=NEW.company_id AND id=NEW.worker_session_id FOR UPDATE;
 IF NOT FOUND OR session_state<>'restoring' THEN
  RAISE EXCEPTION 'provider account identity snapshot must bind while WorkerSession is restoring' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER worker_provider_account_identity_snapshots_validate
 BEFORE INSERT ON worker_provider_account_identity_snapshots FOR EACH ROW
 EXECUTE FUNCTION validate_worker_provider_account_identity_snapshot();
CREATE TRIGGER worker_provider_account_identity_snapshots_immutable
 BEFORE UPDATE OR DELETE ON worker_provider_account_identity_snapshots FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER worker_provider_account_identity_snapshots_no_truncate
 BEFORE TRUNCATE ON worker_provider_account_identity_snapshots FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM worker_provider_account_identity_snapshots) THEN
  RAISE EXCEPTION 'cannot remove persisted Worker provider account identity snapshots';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER worker_provider_account_identity_snapshots_no_truncate ON worker_provider_account_identity_snapshots;
DROP TRIGGER worker_provider_account_identity_snapshots_immutable ON worker_provider_account_identity_snapshots;
DROP TRIGGER worker_provider_account_identity_snapshots_validate ON worker_provider_account_identity_snapshots;
DROP FUNCTION validate_worker_provider_account_identity_snapshot();
DROP TABLE worker_provider_account_identity_snapshots;
