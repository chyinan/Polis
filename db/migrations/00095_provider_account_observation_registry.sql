-- +goose Up
ALTER TABLE worker_provider_account_identity_snapshots
 ADD CONSTRAINT worker_provider_account_identity_snapshots_observation_key
 UNIQUE(company_id,worker_session_id,provider_class,account_fingerprint);

CREATE TABLE provider_account_registry (
 provider_class text NOT NULL CHECK(provider_class ~ '^[A-Za-z0-9_-]{1,80}$'),
 account_fingerprint text NOT NULL CHECK(account_fingerprint ~ '^[a-f0-9]{64}$'),
 first_seen_at timestamptz NOT NULL,
 PRIMARY KEY(provider_class,account_fingerprint)
);

CREATE TABLE provider_account_observations (
 company_id text NOT NULL,
 worker_session_id text NOT NULL,
 provider_class text NOT NULL,
 account_fingerprint text NOT NULL,
 PRIMARY KEY(company_id,worker_session_id),
 FOREIGN KEY(company_id,worker_session_id,provider_class,account_fingerprint)
  REFERENCES worker_provider_account_identity_snapshots(company_id,worker_session_id,provider_class,account_fingerprint),
 FOREIGN KEY(provider_class,account_fingerprint)
  REFERENCES provider_account_registry(provider_class,account_fingerprint)
);

INSERT INTO provider_account_registry(provider_class,account_fingerprint,first_seen_at)
SELECT provider_class,account_fingerprint,min(captured_at)
FROM worker_provider_account_identity_snapshots
WHERE identity_status='available'
GROUP BY provider_class,account_fingerprint;

INSERT INTO provider_account_observations(company_id,worker_session_id,provider_class,account_fingerprint)
SELECT company_id,worker_session_id,provider_class,account_fingerprint
FROM worker_provider_account_identity_snapshots
WHERE identity_status='available';

CREATE TRIGGER provider_account_registry_immutable
 BEFORE UPDATE OR DELETE ON provider_account_registry FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER provider_account_registry_no_truncate
 BEFORE TRUNCATE ON provider_account_registry FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER provider_account_observations_immutable
 BEFORE UPDATE OR DELETE ON provider_account_observations FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER provider_account_observations_no_truncate
 BEFORE TRUNCATE ON provider_account_observations FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM provider_account_observations) OR EXISTS(SELECT 1 FROM provider_account_registry) THEN
  RAISE EXCEPTION 'cannot remove observed provider account history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER provider_account_observations_no_truncate ON provider_account_observations;
DROP TRIGGER provider_account_observations_immutable ON provider_account_observations;
DROP TRIGGER provider_account_registry_no_truncate ON provider_account_registry;
DROP TRIGGER provider_account_registry_immutable ON provider_account_registry;
DROP TABLE provider_account_observations;
DROP TABLE provider_account_registry;
ALTER TABLE worker_provider_account_identity_snapshots
 DROP CONSTRAINT worker_provider_account_identity_snapshots_observation_key;
