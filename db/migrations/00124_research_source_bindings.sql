-- +goose Up
-- Research source registrations are explicit, Mission-scoped metadata. They
-- do not grant network access; ResearchOperation still remains unavailable
-- until a separately qualified backend consumes the binding.
CREATE TABLE research_source_bindings (
 company_id text NOT NULL,
 source_id text NOT NULL CHECK(source_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 mission_id text NOT NULL,
 origin text NOT NULL CHECK(origin ~ '^https://[^/?#@[:space:]]+$'),
 profile_revision text NOT NULL CHECK(profile_revision='research-source@1'),
 identity_sha256 text NOT NULL CHECK(identity_sha256 ~ '^[a-f0-9]{64}$'),
 data_sha256 text NOT NULL CHECK(data_sha256 ~ '^[a-f0-9]{64}$'),
 registration_sha256 text NOT NULL CHECK(registration_sha256 ~ '^[a-f0-9]{64}$'),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,source_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id)
);
CREATE INDEX research_source_bindings_mission ON research_source_bindings(company_id,mission_id,source_id);
CREATE TABLE research_source_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL,
 source_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('authorized','revoked')),
 rationale text NOT NULL CHECK(octet_length(btrim(rationale)) BETWEEN 1 AND 512),
 actor text NOT NULL CHECK(actor='local-owner'),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,source_id) REFERENCES research_source_bindings(company_id,source_id)
);
CREATE INDEX research_source_events_latest ON research_source_events(company_id,source_id,event_seq DESC);
-- +goose StatementBegin
CREATE FUNCTION validate_research_source_event() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
 current_state text;
BEGIN
 SELECT state INTO current_state
 FROM research_source_events
 WHERE company_id=NEW.company_id AND source_id=NEW.source_id
 ORDER BY event_seq DESC LIMIT 1;
 IF current_state IS NULL AND NEW.state<>'authorized' THEN
  RAISE EXCEPTION 'research source must be authorized before it can be revoked' USING ERRCODE='23514';
 END IF;
 IF current_state='authorized' AND NEW.state<>'revoked' THEN
  RAISE EXCEPTION 'research source authorization is not idempotently repeatable' USING ERRCODE='23514';
 END IF;
 IF current_state='revoked' THEN
  RAISE EXCEPTION 'revoked research source cannot be reauthorized' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER research_source_events_provenance
 BEFORE INSERT ON research_source_events
 FOR EACH ROW EXECUTE FUNCTION validate_research_source_event();
CREATE TRIGGER research_source_bindings_immutable
 BEFORE UPDATE OR DELETE ON research_source_bindings
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER research_source_bindings_no_truncate
 BEFORE TRUNCATE ON research_source_bindings FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER research_source_events_immutable
 BEFORE UPDATE OR DELETE ON research_source_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER research_source_events_no_truncate
 BEFORE TRUNCATE ON research_source_events FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
ALTER TABLE research_operations ADD COLUMN source_id text;
ALTER TABLE research_operations
 ADD CONSTRAINT research_operations_source_fk
 FOREIGN KEY(company_id,source_id) REFERENCES research_source_bindings(company_id,source_id);
CREATE INDEX research_operations_source_recent ON research_operations(company_id,source_id,created_at DESC,operation_id DESC);

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM research_operation_events) OR EXISTS(SELECT 1 FROM research_operations WHERE source_id IS NOT NULL) OR EXISTS(SELECT 1 FROM research_source_events) OR EXISTS(SELECT 1 FROM research_source_bindings) THEN
  RAISE EXCEPTION 'cannot discard research source and operation history';
 END IF;
END
$$;
DROP INDEX research_operations_source_recent;
ALTER TABLE research_operations DROP CONSTRAINT research_operations_source_fk;
ALTER TABLE research_operations DROP COLUMN source_id;
DROP TRIGGER research_source_events_no_truncate ON research_source_events;
DROP TRIGGER research_source_events_immutable ON research_source_events;
DROP TRIGGER research_source_bindings_no_truncate ON research_source_bindings;
DROP TRIGGER research_source_bindings_immutable ON research_source_bindings;
DROP TRIGGER research_source_events_provenance ON research_source_events;
DROP FUNCTION validate_research_source_event();
DROP INDEX research_source_events_latest;
DROP TABLE research_source_events;
DROP INDEX research_source_bindings_mission;
DROP TABLE research_source_bindings;
