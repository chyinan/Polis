-- +goose Up
ALTER TABLE memory_dependencies
 ADD COLUMN bound_task_id text,
 ADD COLUMN worker_session_id text;
ALTER TABLE memory_dependencies
 ADD CONSTRAINT memory_dependencies_bound_task_fk
  FOREIGN KEY(company_id,bound_task_id) REFERENCES tasks(company_id,id),
 ADD CONSTRAINT memory_dependencies_worker_context_fk
  FOREIGN KEY(company_id,worker_session_id,bound_task_id) REFERENCES worker_sessions(company_id,id,task_id),
 ADD CONSTRAINT memory_dependencies_dependency_task_unique
  UNIQUE(company_id,dependency_id,bound_task_id),
 ADD CONSTRAINT memory_dependencies_worker_context_pair
  CHECK((bound_task_id IS NULL AND worker_session_id IS NULL) OR (bound_task_id IS NOT NULL AND worker_session_id IS NOT NULL));
-- The legacy unique constraint name is server-truncated. Locate it by its exact
-- column definition instead of depending on PostgreSQL's identifier truncation.
-- +goose StatementBegin
DO $$ DECLARE constraint_name text; BEGIN
 SELECT conname INTO constraint_name FROM pg_constraint
 WHERE conrelid='memory_dependencies'::regclass AND contype='u'
  AND pg_get_constraintdef(oid)='UNIQUE (company_id, record_id, record_revision, target_kind, target_id, target_revision)';
 IF constraint_name IS NULL THEN
  RAISE EXCEPTION 'memory dependency base-target uniqueness constraint was not found';
 END IF;
 EXECUTE format('ALTER TABLE memory_dependencies DROP CONSTRAINT %I', constraint_name);
END $$;
-- +goose StatementEnd
-- Preserve the original uniqueness rule for legacy, unbound rows. PostgreSQL
-- treats NULL values as distinct in regular UNIQUE constraints, so use two
-- partial indexes for the legacy and task-bound cases.
CREATE UNIQUE INDEX memory_dependencies_base_target_unique
 ON memory_dependencies(company_id,record_id,record_revision,target_kind,target_id,target_revision)
 WHERE bound_task_id IS NULL;
CREATE UNIQUE INDEX memory_dependencies_task_target_unique
 ON memory_dependencies(company_id,record_id,record_revision,target_kind,target_id,target_revision,bound_task_id)
 WHERE bound_task_id IS NOT NULL;
CREATE INDEX memory_dependencies_bound_task ON memory_dependencies(company_id,bound_task_id,dependency_id) WHERE bound_task_id IS NOT NULL;

CREATE TABLE memory_task_state_events (
 company_id text NOT NULL,
 company_seq bigint NOT NULL,
 task_id text NOT NULL,
 dependency_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('dirty','frozen','revalidated','cleared')),
 risk_level text NOT NULL CHECK(risk_level IN ('ordinary','high','critical')),
 cause_kind text NOT NULL CHECK(cause_kind IN ('memory_correction','revalidation','policy_change')),
 cause_id text NOT NULL,
 actor text NOT NULL,
 reason text NOT NULL CHECK(octet_length(reason) BETWEEN 1 AND 2048),
 PRIMARY KEY(company_id,company_seq),
 FOREIGN KEY(company_id,company_seq) REFERENCES events(company_id,company_seq),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,dependency_id,task_id) REFERENCES memory_dependencies(company_id,dependency_id,bound_task_id),
 FOREIGN KEY(company_id,actor) REFERENCES employees(company_id,id)
);
CREATE INDEX memory_task_state_latest ON memory_task_state_events(company_id,task_id,dependency_id,company_seq DESC);
CREATE TRIGGER memory_task_state_events_immutable
 BEFORE UPDATE OR DELETE ON memory_task_state_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER memory_task_state_events_no_truncate
 BEFORE TRUNCATE ON memory_task_state_events
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM memory_task_state_events) OR
    EXISTS(SELECT 1 FROM memory_dependencies WHERE bound_task_id IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot roll back memory task impact history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER memory_task_state_events_no_truncate ON memory_task_state_events;
DROP TRIGGER memory_task_state_events_immutable ON memory_task_state_events;
DROP INDEX memory_task_state_latest;
DROP TABLE memory_task_state_events;
DROP INDEX memory_dependencies_bound_task;
DROP INDEX memory_dependencies_task_target_unique;
DROP INDEX memory_dependencies_base_target_unique;
ALTER TABLE memory_dependencies
 ADD CONSTRAINT memory_dependencies_base_target_unique
 UNIQUE(company_id,record_id,record_revision,target_kind,target_id,target_revision);
ALTER TABLE memory_dependencies
 DROP CONSTRAINT memory_dependencies_worker_context_pair,
 DROP CONSTRAINT memory_dependencies_dependency_task_unique,
 DROP CONSTRAINT memory_dependencies_worker_context_fk,
 DROP CONSTRAINT memory_dependencies_bound_task_fk,
 DROP COLUMN worker_session_id,
 DROP COLUMN bound_task_id;
