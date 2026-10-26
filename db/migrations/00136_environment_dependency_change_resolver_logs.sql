-- +goose Up
ALTER TABLE environment_dependency_change_materialization_events
 ADD COLUMN resolver_logs_sha256 text CHECK(resolver_logs_sha256 IS NULL OR resolver_logs_sha256 ~ '^[a-f0-9]{64}$');

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM environment_dependency_change_materialization_events WHERE resolver_logs_sha256 IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot discard dependency resolver log references';
 END IF;
END
$$;
ALTER TABLE environment_dependency_change_materialization_events DROP COLUMN resolver_logs_sha256;
