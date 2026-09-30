-- +goose Up
-- Separate durable reservation from CAS I/O so retries do not publish duplicate inputs.
ALTER TABLE mission_inputs
 ADD COLUMN request_fingerprint text CHECK(request_fingerprint IS NULL OR request_fingerprint ~ '^[0-9a-f]{64}$'),
 ADD COLUMN updated_at timestamptz NOT NULL DEFAULT clock_timestamp();
ALTER TABLE mission_inputs DROP CONSTRAINT mission_inputs_state_check;
ALTER TABLE mission_inputs ADD CONSTRAINT mission_inputs_state_check
 CHECK(state IN ('uploading','stored','usable','partial','unsupported','rejected'));

-- +goose Down
ALTER TABLE mission_inputs DROP CONSTRAINT mission_inputs_state_check;
ALTER TABLE mission_inputs ADD CONSTRAINT mission_inputs_state_check
 CHECK(state IN ('usable','partial','unsupported'));
ALTER TABLE mission_inputs DROP COLUMN updated_at, DROP COLUMN request_fingerprint;
