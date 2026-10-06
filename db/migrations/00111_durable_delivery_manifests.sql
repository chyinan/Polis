-- +goose Up
-- A manifest revision pins one Company/Mission/Task/Artifact tuple. Later
-- changes are represented by another immutable revision, never an update.
-- The extra unique key permits the composite FK to prove that the Artifact
-- belongs to the exact Task named by the manifest.
ALTER TABLE artifacts
 ADD CONSTRAINT artifacts_company_id_task_id_unique UNIQUE(company_id,id,task_id);

CREATE TABLE delivery_manifest_revisions (
 company_id text NOT NULL,
 delivery_id text NOT NULL CHECK(delivery_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 revision bigint NOT NULL CHECK(revision > 0),
 mission_id text NOT NULL,
 task_id text NOT NULL,
 artifact_id text NOT NULL,
 state text NOT NULL DEFAULT 'assembling'
  CHECK(state IN ('assembling','ready','invalidated','withdrawn')),
 manifest jsonb NOT NULL
  CHECK(jsonb_typeof(manifest)='object' AND octet_length(manifest::text)<=1048576),
 manifest_sha256 text NOT NULL CHECK(manifest_sha256 ~ '^[a-f0-9]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,delivery_id,revision),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,artifact_id,task_id) REFERENCES artifacts(company_id,id,task_id),
 CHECK(manifest ? 'state' AND manifest->>'state'=state),
 CHECK(COALESCE(
  manifest->>'schemaVersion'='polis-durable-delivery-manifest@1' AND
  manifest->>'companyId'=company_id AND
  manifest->>'deliveryId'=delivery_id AND
  manifest->>'revision'=revision::text AND
  manifest->>'missionId'=mission_id AND
  manifest->>'taskId'=task_id AND
  manifest->>'artifactId'=artifact_id,
  false
 ))
);
CREATE INDEX delivery_manifest_revisions_mission
 ON delivery_manifest_revisions(company_id,mission_id,delivery_id,revision DESC);

CREATE TABLE delivery_user_dispositions (
 company_id text NOT NULL,
 delivery_id text NOT NULL,
 manifest_revision bigint NOT NULL CHECK(manifest_revision > 0),
 revision bigint NOT NULL CHECK(revision > 0),
 state text NOT NULL DEFAULT 'not_requested'
  CHECK(state IN ('not_requested','awaiting_feedback','accepted','changes_requested')),
 actor text NOT NULL CHECK(octet_length(btrim(actor)) BETWEEN 1 AND 80),
 reason text NOT NULL CHECK(octet_length(btrim(reason)) BETWEEN 1 AND 4096),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 feedback_deadline timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,delivery_id,manifest_revision,revision),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,delivery_id,manifest_revision)
  REFERENCES delivery_manifest_revisions(company_id,delivery_id,revision),
 CHECK(state<>'awaiting_feedback' OR feedback_deadline IS NOT NULL)
);
CREATE INDEX delivery_user_dispositions_latest
 ON delivery_user_dispositions(company_id,delivery_id,manifest_revision,revision DESC);

CREATE TRIGGER delivery_manifest_revisions_immutable
 BEFORE UPDATE OR DELETE ON delivery_manifest_revisions FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER delivery_manifest_revisions_no_truncate
 BEFORE TRUNCATE ON delivery_manifest_revisions FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER delivery_user_dispositions_immutable
 BEFORE UPDATE OR DELETE ON delivery_user_dispositions FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER delivery_user_dispositions_no_truncate
 BEFORE TRUNCATE ON delivery_user_dispositions FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM delivery_user_dispositions)
    OR EXISTS(SELECT 1 FROM delivery_manifest_revisions) THEN
  RAISE EXCEPTION 'cannot discard durable delivery manifests or user dispositions';
 END IF;
END
$$;
-- +goose StatementEnd
DROP TRIGGER delivery_user_dispositions_no_truncate ON delivery_user_dispositions;
DROP TRIGGER delivery_user_dispositions_immutable ON delivery_user_dispositions;
DROP INDEX delivery_user_dispositions_latest;
DROP TABLE delivery_user_dispositions;
DROP TRIGGER delivery_manifest_revisions_no_truncate ON delivery_manifest_revisions;
DROP TRIGGER delivery_manifest_revisions_immutable ON delivery_manifest_revisions;
DROP INDEX delivery_manifest_revisions_mission;
DROP TABLE delivery_manifest_revisions;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_company_id_task_id_unique;
