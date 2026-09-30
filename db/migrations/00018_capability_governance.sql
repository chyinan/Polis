-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION enforce_skill_revision_immutable() RETURNS trigger AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'skill revision history is immutable'; END IF;
 IF ROW(OLD.company_id,OLD.id,OLD.publisher_scope,OLD.package_id,OLD.revision,OLD.display_name,OLD.source_ref,OLD.content_digest,OLD.manifest,OLD.created_at)
    IS DISTINCT FROM
    ROW(NEW.company_id,NEW.id,NEW.publisher_scope,NEW.package_id,NEW.revision,NEW.display_name,NEW.source_ref,NEW.content_digest,NEW.manifest,NEW.created_at) THEN
  RAISE EXCEPTION 'skill revision definition is immutable';
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER skill_revision_definition_immutable BEFORE UPDATE OR DELETE ON skill_revisions
 FOR EACH ROW EXECUTE FUNCTION enforce_skill_revision_immutable();

-- +goose StatementBegin
CREATE FUNCTION enforce_mcp_server_definition_immutable() RETURNS trigger AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'MCP server definition history is immutable'; END IF;
 IF ROW(OLD.company_id,OLD.id,OLD.name,OLD.transport,OLD.endpoint,OLD.command,OLD.args,OLD.descriptor_digest,OLD.created_at)
    IS DISTINCT FROM
    ROW(NEW.company_id,NEW.id,NEW.name,NEW.transport,NEW.endpoint,NEW.command,NEW.args,NEW.descriptor_digest,NEW.created_at) THEN
  RAISE EXCEPTION 'MCP server definition is immutable';
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER mcp_server_definition_immutable BEFORE UPDATE OR DELETE ON mcp_server_definitions
 FOR EACH ROW EXECUTE FUNCTION enforce_mcp_server_definition_immutable();

CREATE TABLE capability_qualification_records (
 company_id text NOT NULL,
 qualification_id text NOT NULL,
 capability_kind text NOT NULL CHECK(capability_kind IN ('skill','mcp')),
 capability_id text NOT NULL,
 version_digest text NOT NULL CHECK(version_digest ~ '^[a-f0-9]{64}$'),
 profile text NOT NULL CHECK(profile IN ('read_only_skill@1','stdio_mcp@1')),
 status text NOT NULL CHECK(status IN ('metadata_verified','needs_external_qualification','failed')),
 evidence_digest text NOT NULL CHECK(evidence_digest ~ '^[a-f0-9]{64}$'),
 evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,qualification_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);
CREATE INDEX capability_qualification_current ON capability_qualification_records(company_id,capability_kind,capability_id,created_at DESC);
CREATE TRIGGER capability_qualification_records_immutable
 BEFORE UPDATE OR DELETE ON capability_qualification_records
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE capability_decisions (
 company_id text NOT NULL,
 decision_id text NOT NULL,
 capability_kind text NOT NULL CHECK(capability_kind IN ('skill','mcp')),
 capability_id text NOT NULL,
 version_digest text NOT NULL CHECK(version_digest ~ '^[a-f0-9]{64}$'),
 qualification_id text NOT NULL,
 decision text NOT NULL CHECK(decision IN ('approved','revoked')),
 rationale text NOT NULL CHECK(octet_length(rationale) BETWEEN 1 AND 512),
 actor text NOT NULL DEFAULT 'local-owner',
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,decision_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 FOREIGN KEY(company_id,qualification_id) REFERENCES capability_qualification_records(company_id,qualification_id)
);
CREATE INDEX capability_decisions_current ON capability_decisions(company_id,capability_kind,capability_id,created_at DESC);
CREATE TRIGGER capability_decisions_immutable
 BEFORE UPDATE OR DELETE ON capability_decisions
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE employee_capability_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL UNIQUE,
 event_id text NOT NULL,
 employee_id text NOT NULL,
 capability_kind text NOT NULL CHECK(capability_kind IN ('skill','mcp')),
 capability_id text NOT NULL,
 version_digest text NOT NULL CHECK(version_digest ~ '^[a-f0-9]{64}$'),
 qualification_id text NOT NULL,
 event text NOT NULL CHECK(event IN ('bound','revoked')),
 reason text NOT NULL CHECK(octet_length(reason) <= 512),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,qualification_id) REFERENCES capability_qualification_records(company_id,qualification_id)
);
CREATE INDEX employee_capability_events_current ON employee_capability_events(company_id,employee_id,capability_kind,capability_id,created_at DESC);
CREATE TRIGGER employee_capability_events_immutable
 BEFORE UPDATE OR DELETE ON employee_capability_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DROP TRIGGER employee_capability_events_immutable ON employee_capability_events;
DROP INDEX employee_capability_events_current;
DROP TABLE employee_capability_events;
DROP TRIGGER capability_decisions_immutable ON capability_decisions;
DROP INDEX capability_decisions_current;
DROP TABLE capability_decisions;
DROP TRIGGER capability_qualification_records_immutable ON capability_qualification_records;
DROP INDEX capability_qualification_current;
DROP TABLE capability_qualification_records;
DROP TRIGGER mcp_server_definition_immutable ON mcp_server_definitions;
DROP FUNCTION enforce_mcp_server_definition_immutable();
DROP TRIGGER skill_revision_definition_immutable ON skill_revisions;
DROP FUNCTION enforce_skill_revision_immutable();
