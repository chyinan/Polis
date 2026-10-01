-- +goose Up
ALTER TABLE memory_dependencies DROP CONSTRAINT memory_dependencies_worker_context_pair;
ALTER TABLE memory_dependencies
 ADD CONSTRAINT memory_dependencies_worker_context_pair
 CHECK((bound_task_id IS NULL AND worker_session_id IS NULL) OR bound_task_id IS NOT NULL);
ALTER TABLE memory_dependency_invalidation_events DROP CONSTRAINT memory_dependency_invalidation_events_cause_kind_check;
ALTER TABLE memory_dependency_invalidation_events
 ADD CONSTRAINT memory_dependency_invalidation_events_cause_kind_check
 CHECK(cause_kind IN ('memory_correction','source_revoked','policy_change','revalidation'));

-- Operator revalidation is recorded by the trusted local control plane. It is
-- distinct from employee-authored correction review and can therefore use the
-- local-owner audit identity rather than impersonating an employee.
-- +goose StatementBegin
DO $$ DECLARE constraint_name text; BEGIN
 SELECT conname INTO constraint_name FROM pg_constraint
 WHERE conrelid='memory_dependency_invalidation_events'::regclass AND contype='f'
  AND pg_get_constraintdef(oid)='FOREIGN KEY (company_id, actor) REFERENCES employees(company_id, id)';
 IF constraint_name IS NULL THEN RAISE EXCEPTION 'memory dependency invalidation actor constraint was not found'; END IF;
 EXECUTE format('ALTER TABLE memory_dependency_invalidation_events DROP CONSTRAINT %I', constraint_name);
 SELECT conname INTO constraint_name FROM pg_constraint
 WHERE conrelid='memory_task_state_events'::regclass AND contype='f'
  AND pg_get_constraintdef(oid)='FOREIGN KEY (company_id, actor) REFERENCES employees(company_id, id)';
 IF constraint_name IS NULL THEN RAISE EXCEPTION 'memory task impact actor constraint was not found'; END IF;
 EXECUTE format('ALTER TABLE memory_task_state_events DROP CONSTRAINT %I', constraint_name);
END $$;
-- +goose StatementEnd

CREATE TABLE memory_task_revalidation_events (
 company_id text NOT NULL,
 company_seq bigint NOT NULL,
 revalidation_id text NOT NULL,
 task_id text NOT NULL,
 task_generation bigint NOT NULL CHECK(task_generation>=0),
 task_plan_sha256 text NOT NULL CHECK(task_plan_sha256 ~ '^[0-9a-f]{64}$'),
 dependency_id text NOT NULL,
 replacement_dependency_id text NOT NULL,
 correction_id text NOT NULL,
 previous_record_id text NOT NULL,
 previous_revision bigint NOT NULL CHECK(previous_revision>0),
 replacement_revision bigint NOT NULL CHECK(replacement_revision>0),
 target_kind text NOT NULL CHECK(target_kind IN ('mission_input','artifact','contract_revision','task_revision')),
 target_id text NOT NULL,
 target_revision bigint NOT NULL CHECK(target_revision>0),
 target_sha256 text NOT NULL CHECK(target_sha256 ~ '^[0-9a-f]{64}$'),
 stopped_session_id text NOT NULL,
 workspace_digest text NOT NULL CHECK(workspace_digest ~ '^[0-9a-f]{64}$'),
 workspace_revision bigint NOT NULL CHECK(workspace_revision>0),
 reviewed_context_sha256 text NOT NULL CHECK(reviewed_context_sha256 ~ '^[0-9a-f]{64}$'),
 actor text NOT NULL CHECK(actor='local-owner'),
 reason text NOT NULL CHECK(octet_length(reason) BETWEEN 1 AND 2048),
 PRIMARY KEY(company_id,company_seq),
 UNIQUE(company_id,revalidation_id),
 UNIQUE(company_id,dependency_id,correction_id),
 FOREIGN KEY(company_id,company_seq) REFERENCES events(company_id,company_seq),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,dependency_id,task_id) REFERENCES memory_dependencies(company_id,dependency_id,bound_task_id),
 FOREIGN KEY(company_id,replacement_dependency_id,task_id) REFERENCES memory_dependencies(company_id,dependency_id,bound_task_id),
 FOREIGN KEY(company_id,correction_id) REFERENCES memory_correction_requests(company_id,correction_id),
 FOREIGN KEY(company_id,previous_record_id,previous_revision) REFERENCES memory_record_revisions(company_id,record_id,revision),
 FOREIGN KEY(company_id,previous_record_id,replacement_revision) REFERENCES memory_record_revisions(company_id,record_id,revision),
 FOREIGN KEY(company_id,stopped_session_id,task_id) REFERENCES worker_sessions(company_id,id,task_id)
);
CREATE INDEX memory_task_revalidation_task_latest ON memory_task_revalidation_events(company_id,task_id,company_seq DESC);
CREATE TRIGGER memory_task_revalidation_events_immutable
 BEFORE UPDATE OR DELETE ON memory_task_revalidation_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER memory_task_revalidation_events_no_truncate
 BEFORE TRUNCATE ON memory_task_revalidation_events
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM memory_task_revalidation_events) OR
    EXISTS(SELECT 1 FROM memory_dependencies WHERE bound_task_id IS NOT NULL AND worker_session_id IS NULL) OR
    EXISTS(SELECT 1 FROM memory_dependency_invalidation_events WHERE cause_kind='revalidation') THEN
  RAISE EXCEPTION 'cannot roll back memory revalidation history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER memory_task_revalidation_events_no_truncate ON memory_task_revalidation_events;
DROP TRIGGER memory_task_revalidation_events_immutable ON memory_task_revalidation_events;
DROP INDEX memory_task_revalidation_task_latest;
DROP TABLE memory_task_revalidation_events;
ALTER TABLE memory_task_state_events
 ADD CONSTRAINT memory_task_state_events_company_id_actor_fkey FOREIGN KEY(company_id,actor) REFERENCES employees(company_id,id);
ALTER TABLE memory_dependency_invalidation_events
 ADD CONSTRAINT memory_dependency_invalidation_events_company_id_actor_fkey FOREIGN KEY(company_id,actor) REFERENCES employees(company_id,id),
 DROP CONSTRAINT memory_dependency_invalidation_events_cause_kind_check,
 ADD CONSTRAINT memory_dependency_invalidation_events_cause_kind_check
 CHECK(cause_kind IN ('memory_correction','source_revoked','policy_change'));
ALTER TABLE memory_dependencies DROP CONSTRAINT memory_dependencies_worker_context_pair;
ALTER TABLE memory_dependencies
 ADD CONSTRAINT memory_dependencies_worker_context_pair
 CHECK((bound_task_id IS NULL AND worker_session_id IS NULL) OR (bound_task_id IS NOT NULL AND worker_session_id IS NOT NULL));
