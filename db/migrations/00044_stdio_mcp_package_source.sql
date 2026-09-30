-- +goose Up
CREATE TABLE mcp_server_package_revisions (
 company_id text NOT NULL,
 package_revision_id text NOT NULL,
 server_id text NOT NULL,
 revision text NOT NULL CHECK(octet_length(revision) BETWEEN 1 AND 64),
 manifest_digest text NOT NULL CHECK(manifest_digest ~ '^[a-f0-9]{64}$'),
 manifest jsonb NOT NULL CHECK(
  jsonb_typeof(manifest)='object' AND
  manifest->>'schemaVersion'='polis-controlled-stdio-mcp@1' AND
  jsonb_typeof(manifest->'args')='array' AND
  jsonb_typeof(manifest->'files')='array' AND
  jsonb_array_length(manifest->'files') BETWEEN 1 AND 63
 ),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,package_revision_id),
 UNIQUE(company_id,server_id,revision),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,server_id) REFERENCES mcp_server_definitions(company_id,id)
);
CREATE INDEX mcp_server_package_revisions_lookup ON mcp_server_package_revisions(company_id,server_id,created_at DESC);
CREATE INDEX mcp_server_package_revisions_digest ON mcp_server_package_revisions(company_id,server_id,manifest_digest);

-- +goose StatementBegin
CREATE FUNCTION enforce_mcp_server_package_definition_match() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 server_name text;
 server_transport text;
 server_command text;
 server_args jsonb;
BEGIN
 SELECT name,transport,COALESCE(command,''),args
 INTO server_name,server_transport,server_command,server_args
 FROM mcp_server_definitions
 WHERE company_id=NEW.company_id AND id=NEW.server_id
 FOR SHARE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'MCP server package has no registered definition';
 END IF;
 IF server_transport<>'stdio' OR server_name<>NEW.manifest->>'name' OR
    server_command<>NEW.manifest->>'command' OR server_args<>NEW.manifest->'args' THEN
  RAISE EXCEPTION 'MCP server package differs from its registered stdio definition';
 END IF;
 RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER mcp_server_package_definition_match
 BEFORE INSERT ON mcp_server_package_revisions
 FOR EACH ROW EXECUTE FUNCTION enforce_mcp_server_package_definition_match();

CREATE TRIGGER mcp_server_package_revisions_immutable
 BEFORE UPDATE OR DELETE ON mcp_server_package_revisions
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
-- +goose StatementBegin
CREATE FUNCTION reject_mcp_server_package_revisions_truncate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'mcp server package history is append only';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER mcp_server_package_revisions_no_truncate
 BEFORE TRUNCATE ON mcp_server_package_revisions
 FOR EACH STATEMENT EXECUTE FUNCTION reject_mcp_server_package_revisions_truncate();

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM mcp_server_package_revisions) THEN
  RAISE EXCEPTION 'cannot downgrade while MCP package source history exists';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER mcp_server_package_revisions_no_truncate ON mcp_server_package_revisions;
DROP TRIGGER mcp_server_package_revisions_immutable ON mcp_server_package_revisions;
DROP TRIGGER mcp_server_package_definition_match ON mcp_server_package_revisions;
DROP FUNCTION reject_mcp_server_package_revisions_truncate();
DROP FUNCTION enforce_mcp_server_package_definition_match();
DROP INDEX mcp_server_package_revisions_digest;
DROP INDEX mcp_server_package_revisions_lookup;
DROP TABLE mcp_server_package_revisions;
