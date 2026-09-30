-- +goose Up
ALTER TABLE job_runs ADD COLUMN service_id text;

-- Older service runs did not persist the selected definition. Preserve that
-- limitation explicitly instead of inventing an attribution from the port.
UPDATE job_runs SET service_id='legacy:unattributed' WHERE kind='service';

ALTER TABLE job_runs ADD CONSTRAINT job_runs_service_id_consistency CHECK (
 (kind='service' AND service_id IS NOT NULL AND (service_id='legacy:unattributed' OR service_id ~ '^[a-z][a-z0-9-]{0,63}$')) OR
 (kind<>'service' AND service_id IS NULL)
);

-- +goose Down
ALTER TABLE job_runs DROP CONSTRAINT job_runs_service_id_consistency;
ALTER TABLE job_runs DROP COLUMN service_id;
