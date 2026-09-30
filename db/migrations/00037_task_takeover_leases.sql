-- +goose Up
CREATE TABLE task_takeover_leases (
 company_id text NOT NULL,
 lease_id text NOT NULL CHECK(octet_length(lease_id) BETWEEN 1 AND 80),
 mission_id text NOT NULL,
 task_id text NOT NULL,
 client_request_id text NOT NULL CHECK(octet_length(client_request_id) BETWEEN 1 AND 80),
 base_requirements_sha256 text NOT NULL CHECK(base_requirements_sha256 ~ '^[a-f0-9]{64}$'),
 base_workspace_digest text NOT NULL CHECK(base_workspace_digest ~ '^[a-f0-9]{64}$'),
 base_workspace_revision bigint NOT NULL CHECK(base_workspace_revision > 0),
 created_by text NOT NULL CHECK(octet_length(created_by) BETWEEN 1 AND 80),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,lease_id),
 UNIQUE(company_id,client_request_id),
 UNIQUE(company_id,lease_id,task_id,mission_id),
 FOREIGN KEY(company_id,task_id,mission_id) REFERENCES tasks(company_id,id,mission_id)
);
CREATE INDEX task_takeover_leases_mission_created ON task_takeover_leases(company_id,mission_id,created_at DESC,lease_id);
CREATE TRIGGER task_takeover_leases_immutable
 BEFORE UPDATE OR DELETE ON task_takeover_leases
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE task_takeover_active_slots (
 company_id text NOT NULL,
 task_id text NOT NULL,
 lease_id text NOT NULL,
 mission_id text NOT NULL,
 PRIMARY KEY(company_id,task_id),
 UNIQUE(company_id,lease_id),
 FOREIGN KEY(company_id,task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,lease_id,task_id,mission_id) REFERENCES task_takeover_leases(company_id,lease_id,task_id,mission_id)
);
CREATE TRIGGER task_takeover_active_slots_immutable_update
 BEFORE UPDATE ON task_takeover_active_slots
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE task_takeover_lease_events (
 company_id text NOT NULL,
 event_id text NOT NULL,
 lease_id text NOT NULL,
 event_seq bigserial NOT NULL UNIQUE,
 state text NOT NULL CHECK(state IN ('granted','returned','released')),
 snapshot_input_id text,
 snapshot_revision bigint,
 snapshot_digest text CHECK(snapshot_digest IS NULL OR snapshot_digest ~ '^[a-f0-9]{64}$'),
 snapshot_bytes bigint CHECK(snapshot_bytes IS NULL OR snapshot_bytes BETWEEN 1 AND 4096),
 diff_summary jsonb NOT NULL DEFAULT '{}'::jsonb CHECK(jsonb_typeof(diff_summary)='object'),
 human_effort_seconds integer CHECK(human_effort_seconds IS NULL OR human_effort_seconds BETWEEN 1 AND 86400),
 reason_code text NOT NULL CHECK(octet_length(reason_code) <= 120),
 actor text NOT NULL CHECK(octet_length(actor) BETWEEN 1 AND 80),
 command_request_id text NOT NULL CHECK(octet_length(command_request_id) BETWEEN 1 AND 80),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 UNIQUE(company_id,command_request_id),
 FOREIGN KEY(company_id,lease_id) REFERENCES task_takeover_leases(company_id,lease_id),
 FOREIGN KEY(company_id,snapshot_input_id,snapshot_revision) REFERENCES mission_inputs(company_id,input_id,revision),
 CHECK((state='returned') = (snapshot_input_id IS NOT NULL AND snapshot_revision IS NOT NULL AND snapshot_digest IS NOT NULL AND snapshot_bytes IS NOT NULL)),
 CHECK((state='returned') OR human_effort_seconds IS NULL)
);
CREATE INDEX task_takeover_lease_events_latest ON task_takeover_lease_events(company_id,lease_id,event_seq DESC);
CREATE INDEX task_takeover_lease_events_snapshot ON task_takeover_lease_events(company_id,snapshot_input_id,snapshot_revision) WHERE state='returned';
CREATE TRIGGER task_takeover_lease_events_immutable
 BEFORE UPDATE OR DELETE ON task_takeover_lease_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM task_takeover_leases)
    OR EXISTS (SELECT 1 FROM task_takeover_lease_events)
    OR EXISTS (SELECT 1 FROM task_takeover_active_slots) THEN
  RAISE EXCEPTION 'cannot roll back human takeover leases or provenance';
 END IF;
END
$$;
-- +goose StatementEnd
DROP TRIGGER task_takeover_lease_events_immutable ON task_takeover_lease_events;
DROP INDEX task_takeover_lease_events_snapshot;
DROP INDEX task_takeover_lease_events_latest;
DROP TABLE task_takeover_lease_events;
DROP TRIGGER task_takeover_active_slots_immutable_update ON task_takeover_active_slots;
DROP TABLE task_takeover_active_slots;
DROP TRIGGER task_takeover_leases_immutable ON task_takeover_leases;
DROP INDEX task_takeover_leases_mission_created;
DROP TABLE task_takeover_leases;
