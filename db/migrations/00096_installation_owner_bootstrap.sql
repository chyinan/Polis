-- +goose Up
CREATE TABLE installation_owner (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 password_scheme text NOT NULL CHECK(password_scheme='argon2id-v1'),
 password_hash text NOT NULL CHECK(password_hash ~ '^\$argon2id\$v=19\$m=65536,t=3,p=4\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$'),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision > 0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE installation_owner_bootstrap (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 token_sha256 text NOT NULL CHECK(token_sha256 ~ '^[a-f0-9]{64}$'),
 revision bigint NOT NULL CHECK(revision > 0),
 issued_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL,
 consumed_at timestamptz,
 CHECK(expires_at > issued_at),
 CHECK(consumed_at IS NULL OR consumed_at >= issued_at)
);

CREATE TABLE installation_owner_bootstrap_attempts (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 window_started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 failure_count integer NOT NULL DEFAULT 0 CHECK(failure_count BETWEEN 0 AND 5),
 blocked_until timestamptz
);

CREATE TABLE installation_owner_auth_events (
 event_id bigserial PRIMARY KEY,
 event_type text NOT NULL CHECK(event_type IN ('bootstrap_issued','bootstrap_failed','owner_initialized')),
 bootstrap_revision bigint,
 subject_sha256 text CHECK(subject_sha256 IS NULL OR subject_sha256 ~ '^[a-f0-9]{64}$'),
 occurred_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TRIGGER installation_owner_auth_events_immutable
 BEFORE UPDATE OR DELETE ON installation_owner_auth_events FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER installation_owner_auth_events_no_truncate
 BEFORE TRUNCATE ON installation_owner_auth_events FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM installation_owner) OR EXISTS(SELECT 1 FROM installation_owner_bootstrap) OR EXISTS(SELECT 1 FROM installation_owner_auth_events) THEN
  RAISE EXCEPTION 'cannot remove installation owner credential or bootstrap history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER installation_owner_auth_events_no_truncate ON installation_owner_auth_events;
DROP TRIGGER installation_owner_auth_events_immutable ON installation_owner_auth_events;
DROP TABLE installation_owner_auth_events;
DROP TABLE installation_owner_bootstrap_attempts;
DROP TABLE installation_owner_bootstrap;
DROP TABLE installation_owner;
