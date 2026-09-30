-- +goose Up
CREATE TABLE mission_change_requests (
 company_id text NOT NULL,
 change_request_id text NOT NULL CHECK(octet_length(change_request_id) BETWEEN 1 AND 80),
 mission_id text NOT NULL,
 client_request_id text NOT NULL CHECK(octet_length(client_request_id) BETWEEN 1 AND 80),
 base_requirements_sha256 text NOT NULL CHECK(base_requirements_sha256 ~ '^[a-f0-9]{64}$'),
 change_summary text NOT NULL CHECK(octet_length(btrim(change_summary)) BETWEEN 1 AND 4096),
 proposed_title text NOT NULL CHECK(octet_length(btrim(proposed_title)) BETWEEN 1 AND 200),
 proposed_goal text NOT NULL CHECK(octet_length(btrim(proposed_goal)) BETWEEN 1 AND 4096),
 proposed_acceptance_contract jsonb,
 block_previous_results boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,change_request_id),
 UNIQUE(company_id,client_request_id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 CHECK(proposed_acceptance_contract IS NULL OR jsonb_typeof(proposed_acceptance_contract)='object')
);
CREATE INDEX mission_change_requests_mission_created ON mission_change_requests(company_id,mission_id,created_at DESC,change_request_id);
CREATE TRIGGER mission_change_requests_immutable
 BEFORE UPDATE OR DELETE ON mission_change_requests
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE mission_change_request_events (
 company_id text NOT NULL,
 event_id text NOT NULL,
 change_request_id text NOT NULL,
 event_seq bigserial NOT NULL UNIQUE,
 state text NOT NULL CHECK(state IN ('received','queued','considered','applied','declined','superseded')),
 impact_revision bigint,
 successor_mission_id text,
 details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK(jsonb_typeof(details)='object'),
 reason_code text NOT NULL CHECK(octet_length(reason_code) <= 120),
 actor text NOT NULL CHECK(octet_length(actor) BETWEEN 1 AND 80),
 command_request_id text NOT NULL CHECK(octet_length(command_request_id) BETWEEN 1 AND 80),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 UNIQUE(company_id,command_request_id),
 FOREIGN KEY(company_id,change_request_id) REFERENCES mission_change_requests(company_id,change_request_id),
 FOREIGN KEY(company_id,successor_mission_id) REFERENCES missions(company_id,id),
 CHECK((state='applied') = (successor_mission_id IS NOT NULL))
);
CREATE INDEX mission_change_request_events_latest ON mission_change_request_events(company_id,change_request_id,event_seq DESC);
CREATE TRIGGER mission_change_request_events_immutable
 BEFORE UPDATE OR DELETE ON mission_change_request_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE mission_change_request_impacts (
 company_id text NOT NULL,
 change_request_id text NOT NULL,
 revision bigint NOT NULL CHECK(revision > 0),
 impact_sha256 text NOT NULL CHECK(impact_sha256 ~ '^[a-f0-9]{64}$'),
 impact jsonb NOT NULL CHECK(jsonb_typeof(impact)='object'),
 created_by text NOT NULL CHECK(octet_length(created_by) BETWEEN 1 AND 80),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,change_request_id,revision),
 FOREIGN KEY(company_id,change_request_id) REFERENCES mission_change_requests(company_id,change_request_id)
);
CREATE TRIGGER mission_change_request_impacts_immutable
 BEFORE UPDATE OR DELETE ON mission_change_request_impacts
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE mission_change_request_output_blocks (
 company_id text NOT NULL,
 change_request_id text NOT NULL,
 artifact_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,change_request_id,artifact_id),
 FOREIGN KEY(company_id,change_request_id) REFERENCES mission_change_requests(company_id,change_request_id),
 FOREIGN KEY(company_id,artifact_id) REFERENCES artifacts(company_id,id)
);
CREATE INDEX mission_change_request_output_blocks_artifact ON mission_change_request_output_blocks(company_id,artifact_id);
CREATE TRIGGER mission_change_request_output_blocks_immutable
 BEFORE UPDATE OR DELETE ON mission_change_request_output_blocks
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM mission_change_requests)
    OR EXISTS (SELECT 1 FROM mission_change_request_events)
    OR EXISTS (SELECT 1 FROM mission_change_request_impacts)
    OR EXISTS (SELECT 1 FROM mission_change_request_output_blocks) THEN
  RAISE EXCEPTION 'cannot roll back mission change history without losing requests, decisions or output blocks';
 END IF;
END
$$;
-- +goose StatementEnd
DROP TRIGGER mission_change_request_output_blocks_immutable ON mission_change_request_output_blocks;
DROP INDEX mission_change_request_output_blocks_artifact;
DROP TABLE mission_change_request_output_blocks;
DROP TRIGGER mission_change_request_impacts_immutable ON mission_change_request_impacts;
DROP TABLE mission_change_request_impacts;
DROP TRIGGER mission_change_request_events_immutable ON mission_change_request_events;
DROP INDEX mission_change_request_events_latest;
DROP TABLE mission_change_request_events;
DROP TRIGGER mission_change_requests_immutable ON mission_change_requests;
DROP INDEX mission_change_requests_mission_created;
DROP TABLE mission_change_requests;
