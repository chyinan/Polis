-- +goose Up
ALTER TABLE installation_owner_auth_events
 DROP CONSTRAINT installation_owner_auth_events_event_type_check;
ALTER TABLE installation_owner_auth_events
 ADD CONSTRAINT installation_owner_auth_events_event_type_check
 CHECK(event_type IN ('bootstrap_issued','bootstrap_failed','owner_initialized','login_failed','login_succeeded','session_revoked'));

CREATE TABLE installation_owner_sessions (
 token_sha256 text PRIMARY KEY CHECK(token_sha256 ~ '^[a-f0-9]{64}$'),
 csrf_sha256 text NOT NULL CHECK(csrf_sha256 ~ '^[a-f0-9]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 expires_at timestamptz NOT NULL,
 revoked_at timestamptz,
 CHECK(expires_at > created_at),
 CHECK(revoked_at IS NULL OR revoked_at >= created_at)
);
CREATE INDEX installation_owner_sessions_active_expiry
 ON installation_owner_sessions(expires_at) WHERE revoked_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION protect_installation_owner_session() RETURNS trigger AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  RAISE EXCEPTION 'installation owner session history is immutable' USING ERRCODE='23514';
 END IF;
 IF (NEW.token_sha256,NEW.csrf_sha256,NEW.created_at,NEW.expires_at) IS DISTINCT FROM
    (OLD.token_sha256,OLD.csrf_sha256,OLD.created_at,OLD.expires_at) OR
    OLD.revoked_at IS NOT NULL OR NEW.revoked_at IS NULL THEN
  RAISE EXCEPTION 'installation owner session history is immutable except for one revocation' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER installation_owner_sessions_revoke_once
 BEFORE UPDATE OR DELETE ON installation_owner_sessions FOR EACH ROW
 EXECUTE FUNCTION protect_installation_owner_session();
CREATE TRIGGER installation_owner_sessions_no_truncate
 BEFORE TRUNCATE ON installation_owner_sessions FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE installation_owner_login_attempts (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 window_started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 failure_count integer NOT NULL DEFAULT 0 CHECK(failure_count BETWEEN 0 AND 5),
 blocked_until timestamptz
);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM installation_owner_sessions) OR
    EXISTS(SELECT 1 FROM installation_owner_auth_events WHERE event_type IN ('login_failed','login_succeeded','session_revoked')) THEN
  RAISE EXCEPTION 'cannot remove installation owner session history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TABLE installation_owner_login_attempts;
DROP TRIGGER installation_owner_sessions_no_truncate ON installation_owner_sessions;
DROP TRIGGER installation_owner_sessions_revoke_once ON installation_owner_sessions;
DROP FUNCTION protect_installation_owner_session();
DROP INDEX installation_owner_sessions_active_expiry;
DROP TABLE installation_owner_sessions;
ALTER TABLE installation_owner_auth_events
 DROP CONSTRAINT installation_owner_auth_events_event_type_check;
ALTER TABLE installation_owner_auth_events
 ADD CONSTRAINT installation_owner_auth_events_event_type_check
 CHECK(event_type IN ('bootstrap_issued','bootstrap_failed','owner_initialized'));
