-- +goose Up
ALTER TABLE missions ADD COLUMN acceptance_contract jsonb;
ALTER TABLE missions ADD CONSTRAINT missions_acceptance_contract_shape CHECK (
 acceptance_contract IS NULL OR CASE
  WHEN jsonb_typeof(acceptance_contract) = 'object'
   AND acceptance_contract->>'revision' = 'text-acceptance@1'
   AND jsonb_typeof(acceptance_contract->'required_text') = 'array'
  THEN jsonb_array_length(acceptance_contract->'required_text') BETWEEN 1 AND 8
  ELSE FALSE
 END
);

ALTER TABLE tasks ADD CONSTRAINT tasks_company_task_mission_unique UNIQUE(company_id,id,mission_id);

CREATE TABLE task_validation_bindings (
 company_id text NOT NULL,
 task_id text NOT NULL,
 mission_id text NOT NULL,
 acceptance_revision text NOT NULL CHECK(acceptance_revision!=''),
 runner_kind text NOT NULL CHECK(runner_kind!=''),
 runner_revision text NOT NULL CHECK(runner_revision!=''),
 configuration_digest text NOT NULL CHECK(configuration_digest ~ '^[a-f0-9]{64}$'),
 contract jsonb NOT NULL CHECK(
  jsonb_typeof(contract) = 'object'
  AND contract->>'revision' = acceptance_revision
  AND jsonb_typeof(contract->'required_text') = 'array'
 ),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,task_id),
 FOREIGN KEY(company_id,task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id)
);

-- +goose StatementBegin
CREATE FUNCTION reject_task_validation_binding_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'task_validation_bindings are immutable';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER task_validation_bindings_immutable
 BEFORE UPDATE OR DELETE ON task_validation_bindings
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

ALTER TABLE worker_checks DROP CONSTRAINT worker_checks_phase_check;
ALTER TABLE worker_checks ADD CONSTRAINT worker_checks_phase_check CHECK(phase IN ('single','full','product'));

CREATE TABLE task_validation_artifact_qualifications (
 company_id text NOT NULL,
 task_id text NOT NULL,
 artifact_id text NOT NULL,
 checkpoint_id text NOT NULL,
 check_id text NOT NULL,
 session_id text NOT NULL,
 validation_binding_digest text NOT NULL CHECK(validation_binding_digest ~ '^[a-f0-9]{64}$'),
 workspace_digest text NOT NULL CHECK(workspace_digest ~ '^[a-f0-9]{64}$'),
 workspace_revision bigint NOT NULL CHECK(workspace_revision>0),
 runner_revision text NOT NULL CHECK(runner_revision!=''),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,task_id),
 UNIQUE(company_id,artifact_id),
 FOREIGN KEY(company_id,task_id) REFERENCES task_validation_bindings(company_id,task_id),
 FOREIGN KEY(company_id,artifact_id) REFERENCES artifacts(company_id,id),
 FOREIGN KEY(company_id,checkpoint_id) REFERENCES worker_checkpoints(company_id,id),
 FOREIGN KEY(company_id,check_id) REFERENCES worker_checks(company_id,id),
 FOREIGN KEY(company_id,session_id) REFERENCES worker_sessions(company_id,id)
);
CREATE TRIGGER task_validation_artifact_qualifications_immutable
 BEFORE UPDATE OR DELETE ON task_validation_artifact_qualifications
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DROP TRIGGER task_validation_artifact_qualifications_immutable ON task_validation_artifact_qualifications;
DROP TABLE task_validation_artifact_qualifications;
DROP TRIGGER task_validation_bindings_immutable ON task_validation_bindings;
DROP FUNCTION reject_task_validation_binding_mutation();
DROP TABLE task_validation_bindings;
ALTER TABLE worker_checks DROP CONSTRAINT worker_checks_phase_check;
ALTER TABLE worker_checks ADD CONSTRAINT worker_checks_phase_check CHECK(phase IN ('single','full'));
ALTER TABLE tasks DROP CONSTRAINT tasks_company_task_mission_unique;
ALTER TABLE missions DROP CONSTRAINT missions_acceptance_contract_shape;
ALTER TABLE missions DROP COLUMN acceptance_contract;
