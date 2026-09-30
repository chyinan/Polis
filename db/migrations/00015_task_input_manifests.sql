-- +goose Up
CREATE TABLE task_input_manifests (
 company_id text NOT NULL,
 task_id text NOT NULL,
 mission_id text NOT NULL,
 schema_version text NOT NULL CHECK(schema_version='polis-model-input-manifest@1'),
 manifest_digest text NOT NULL CHECK(manifest_digest ~ '^[a-f0-9]{64}$'),
 delivery_status text NOT NULL CHECK(delivery_status='not_delivered'),
 manifest jsonb NOT NULL CHECK(
  jsonb_typeof(manifest)='object'
  AND manifest->>'schemaVersion'=schema_version
  AND manifest->>'companyId'=company_id
  AND manifest->>'missionId'=mission_id
  AND manifest->>'taskId'=task_id
  AND manifest->>'deliveryStatus'=delivery_status
  AND jsonb_typeof(manifest->'candidateInputs')='array'
  AND jsonb_typeof(manifest->'excludedInputs')='array'
 ),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,task_id),
 FOREIGN KEY(company_id,task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id)
);
CREATE INDEX task_input_manifests_mission ON task_input_manifests(company_id,mission_id,created_at DESC);
CREATE TRIGGER task_input_manifests_immutable
 BEFORE UPDATE OR DELETE ON task_input_manifests
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DROP TRIGGER task_input_manifests_immutable ON task_input_manifests;
DROP INDEX task_input_manifests_mission;
DROP TABLE task_input_manifests;
