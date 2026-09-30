-- +goose Up
-- Persist the idempotency fence before local MCP code is started. A reserved
-- request whose process result cannot be recorded is never replayed.
CREATE TABLE mcp_runtime_observation_intents (
 company_id text NOT NULL,
 request_id text NOT NULL,
 capability_id text NOT NULL,
 capability_qualification_id text NOT NULL,
 package_revision_id text NOT NULL,
 package_manifest_sha256 text NOT NULL CHECK(package_manifest_sha256 ~ '^[a-f0-9]{64}$'),
 fingerprint text NOT NULL CHECK(fingerprint ~ '^[a-f0-9]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,request_id),
 FOREIGN KEY(company_id,capability_id) REFERENCES mcp_server_definitions(company_id,id),
 FOREIGN KEY(company_id,capability_qualification_id) REFERENCES capability_qualification_records(company_id,qualification_id),
 FOREIGN KEY(company_id,package_revision_id) REFERENCES mcp_server_package_revisions(company_id,package_revision_id)
);
CREATE TRIGGER mcp_runtime_observation_intents_immutable
 BEFORE UPDATE OR DELETE ON mcp_runtime_observation_intents
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER mcp_runtime_observation_intents_no_truncate
 BEFORE TRUNCATE ON mcp_runtime_observation_intents
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE mcp_runtime_observation_intent_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL UNIQUE,
 event_id text NOT NULL,
 observation_request_id text NOT NULL,
 status text NOT NULL CHECK(status IN ('reserved','completed','outcome_unknown')),
 package_revision_id text NOT NULL,
 package_manifest_sha256 text NOT NULL CHECK(package_manifest_sha256 ~ '^[a-f0-9]{64}$'),
 runtime_qualification_id text,
 rationale text NOT NULL CHECK(octet_length(rationale)<=512),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,observation_request_id) REFERENCES mcp_runtime_observation_intents(company_id,request_id),
 FOREIGN KEY(company_id,package_revision_id) REFERENCES mcp_server_package_revisions(company_id,package_revision_id),
 FOREIGN KEY(company_id,runtime_qualification_id) REFERENCES mcp_runtime_qualification_records(company_id,runtime_qualification_id),
 CHECK((status='completed' AND runtime_qualification_id IS NOT NULL) OR (status<>'completed' AND runtime_qualification_id IS NULL))
);
CREATE INDEX mcp_runtime_observation_intent_events_current
 ON mcp_runtime_observation_intent_events(company_id,observation_request_id,event_seq DESC);

-- +goose StatementBegin
CREATE FUNCTION enforce_mcp_runtime_observation_intent_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 intent RECORD;
 prior_status text;
 latest_package_id text;
 latest_manifest_digest text;
 descriptor_digest text;
 descriptor_status text;
 metadata_status text;
 metadata_profile text;
 metadata_digest text;
 metadata_decision text;
 runtime_source_id text;
 runtime_source_digest text;
BEGIN
 SELECT capability_id,capability_qualification_id,package_revision_id,package_manifest_sha256
 INTO intent FROM mcp_runtime_observation_intents
 WHERE company_id=NEW.company_id AND request_id=NEW.observation_request_id FOR SHARE;
 IF NOT FOUND OR NEW.package_revision_id IS DISTINCT FROM intent.package_revision_id
    OR NEW.package_manifest_sha256 IS DISTINCT FROM intent.package_manifest_sha256 THEN
  RAISE EXCEPTION 'MCP observation event differs from its immutable intent' USING ERRCODE='23514';
 END IF;
 SELECT status INTO prior_status FROM mcp_runtime_observation_intent_events
 WHERE company_id=NEW.company_id AND observation_request_id=NEW.observation_request_id
 ORDER BY event_seq DESC LIMIT 1;

 IF NEW.status='reserved' THEN
  IF prior_status IS NOT NULL THEN
   RAISE EXCEPTION 'MCP observation request was already reserved' USING ERRCODE='23514';
  END IF;
  SELECT d.descriptor_digest,d.status INTO descriptor_digest,descriptor_status
  FROM mcp_server_definitions d WHERE d.company_id=NEW.company_id AND d.id=intent.capability_id AND d.transport='stdio' FOR SHARE;
  SELECT q.status,q.profile,q.version_digest INTO metadata_status,metadata_profile,metadata_digest
  FROM capability_qualification_records q
  WHERE q.company_id=NEW.company_id AND q.qualification_id=intent.capability_qualification_id AND q.capability_kind='mcp' AND q.capability_id=intent.capability_id;
  SELECT decision INTO metadata_decision FROM capability_decisions
  WHERE company_id=NEW.company_id AND capability_kind='mcp' AND capability_id=intent.capability_id
    AND version_digest=descriptor_digest AND qualification_id=intent.capability_qualification_id
  ORDER BY created_at DESC,decision_id DESC LIMIT 1;
  SELECT package_revision_id,manifest_digest INTO latest_package_id,latest_manifest_digest
  FROM mcp_server_package_revisions WHERE company_id=NEW.company_id AND server_id=intent.capability_id
  ORDER BY created_at DESC,package_revision_id LIMIT 1 FOR SHARE;
  IF descriptor_digest IS NULL OR descriptor_status IS DISTINCT FROM 'approved'
     OR metadata_status IS DISTINCT FROM 'metadata_verified' OR metadata_profile IS DISTINCT FROM 'stdio_mcp@1'
     OR metadata_digest IS DISTINCT FROM descriptor_digest OR metadata_decision IS DISTINCT FROM 'approved'
     OR latest_package_id IS DISTINCT FROM intent.package_revision_id
     OR latest_manifest_digest IS DISTINCT FROM intent.package_manifest_sha256 THEN
   RAISE EXCEPTION 'MCP observation requires the current approved descriptor and latest imported package' USING ERRCODE='23514';
  END IF;
 ELSIF NEW.status='completed' THEN
  IF prior_status IS DISTINCT FROM 'reserved' THEN
   RAISE EXCEPTION 'MCP observation completion requires a reserved request' USING ERRCODE='23514';
  END IF;
  SELECT process_spec->>'SourcePackageRevisionID',process_spec->>'SourcePackageManifestSHA256'
  INTO runtime_source_id,runtime_source_digest FROM mcp_runtime_qualification_records
  WHERE company_id=NEW.company_id AND runtime_qualification_id=NEW.runtime_qualification_id
    AND capability_id=intent.capability_id AND capability_qualification_id=intent.capability_qualification_id;
  IF runtime_source_id IS DISTINCT FROM intent.package_revision_id OR runtime_source_digest IS DISTINCT FROM intent.package_manifest_sha256 THEN
   RAISE EXCEPTION 'MCP runtime qualification does not bind the reserved source package' USING ERRCODE='23514';
  END IF;
 ELSIF NEW.status='outcome_unknown' THEN
  IF prior_status IS DISTINCT FROM 'reserved' THEN
   RAISE EXCEPTION 'MCP unknown outcome requires a reserved request' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER mcp_runtime_observation_intent_events_validate
 BEFORE INSERT ON mcp_runtime_observation_intent_events
 FOR EACH ROW EXECUTE FUNCTION enforce_mcp_runtime_observation_intent_event();
CREATE TRIGGER mcp_runtime_observation_intent_events_immutable
 BEFORE UPDATE OR DELETE ON mcp_runtime_observation_intent_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER mcp_runtime_observation_intent_events_no_truncate
 BEFORE TRUNCATE ON mcp_runtime_observation_intent_events
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM mcp_runtime_observation_intent_events) OR EXISTS(SELECT 1 FROM mcp_runtime_observation_intents) THEN
  RAISE EXCEPTION 'cannot roll back MCP runtime observation intent evidence';
 END IF;
END $$;
DROP TRIGGER mcp_runtime_observation_intent_events_no_truncate ON mcp_runtime_observation_intent_events;
DROP TRIGGER mcp_runtime_observation_intent_events_immutable ON mcp_runtime_observation_intent_events;
DROP TRIGGER mcp_runtime_observation_intent_events_validate ON mcp_runtime_observation_intent_events;
DROP FUNCTION enforce_mcp_runtime_observation_intent_event();
DROP INDEX mcp_runtime_observation_intent_events_current;
DROP TABLE mcp_runtime_observation_intent_events;
DROP TRIGGER mcp_runtime_observation_intents_no_truncate ON mcp_runtime_observation_intents;
DROP TRIGGER mcp_runtime_observation_intents_immutable ON mcp_runtime_observation_intents;
DROP TABLE mcp_runtime_observation_intents;
