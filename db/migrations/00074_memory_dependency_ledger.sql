-- +goose Up
-- Memory records and dependencies are append-only. Their latest state is
-- derived from company-sequenced events; the source and target revisions stay
-- pinned even after later corrections.
CREATE TABLE memory_records (
 company_id text NOT NULL,
 record_id text NOT NULL,
 record_kind text NOT NULL CHECK(record_kind IN ('role_profile','verified_fact','decision','hypothesis','responsibility','temporary_context')),
 scope_kind text NOT NULL CHECK(scope_kind IN ('company','mission','employee')),
 mission_id text,
 employee_id text,
 sensitivity text NOT NULL CHECK(sensitivity IN ('public','internal','confidential','restricted')),
 created_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,record_id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,created_by) REFERENCES employees(company_id,id),
 CHECK((scope_kind='company' AND mission_id IS NULL AND employee_id IS NULL) OR
       (scope_kind='mission' AND mission_id IS NOT NULL AND employee_id IS NULL) OR
       (scope_kind='employee' AND employee_id IS NOT NULL))
);
CREATE INDEX memory_records_scope ON memory_records(company_id,scope_kind,mission_id,employee_id,record_id);
CREATE TRIGGER memory_records_immutable
 BEFORE UPDATE OR DELETE ON memory_records
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER memory_records_no_truncate
 BEFORE TRUNCATE ON memory_records
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE memory_record_revisions (
 company_id text NOT NULL,
 record_id text NOT NULL,
 revision bigint NOT NULL CHECK(revision>0),
 content text NOT NULL CHECK(octet_length(content) BETWEEN 1 AND 32768),
 content_sha256 text NOT NULL CHECK(content_sha256 ~ '^[0-9a-f]{64}$'),
 source_kind text CHECK(source_kind IN ('mission_input','artifact')),
 source_id text,
 source_revision bigint,
 source_sha256 text,
 observed_at timestamptz NOT NULL,
 created_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,record_id,revision),
 FOREIGN KEY(company_id,record_id) REFERENCES memory_records(company_id,record_id),
 FOREIGN KEY(company_id,created_by) REFERENCES employees(company_id,id),
 CHECK((source_kind IS NULL AND source_id IS NULL AND source_revision IS NULL AND source_sha256 IS NULL) OR
       (source_kind IS NOT NULL AND source_id IS NOT NULL AND source_revision IS NOT NULL AND source_revision>0 AND
        source_sha256 IS NOT NULL AND source_sha256 ~ '^[0-9a-f]{64}$'))
);
CREATE INDEX memory_record_revisions_source ON memory_record_revisions(company_id,source_kind,source_id,source_revision);
CREATE TRIGGER memory_record_revisions_immutable
 BEFORE UPDATE OR DELETE ON memory_record_revisions
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER memory_record_revisions_no_truncate
 BEFORE TRUNCATE ON memory_record_revisions
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE memory_revision_state_events (
 company_id text NOT NULL,
 company_seq bigint NOT NULL,
 record_id text NOT NULL,
 revision bigint NOT NULL,
 state text NOT NULL CHECK(state IN ('proposed','verified','disputed','superseded','expired')),
 actor text NOT NULL,
 reason text NOT NULL CHECK(octet_length(reason) BETWEEN 1 AND 2048),
 PRIMARY KEY(company_id,company_seq),
 FOREIGN KEY(company_id,company_seq) REFERENCES events(company_id,company_seq),
 FOREIGN KEY(company_id,record_id,revision) REFERENCES memory_record_revisions(company_id,record_id,revision),
 FOREIGN KEY(company_id,actor) REFERENCES employees(company_id,id)
);
CREATE INDEX memory_revision_state_latest ON memory_revision_state_events(company_id,record_id,revision,company_seq DESC);
CREATE TRIGGER memory_revision_state_events_immutable
 BEFORE UPDATE OR DELETE ON memory_revision_state_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER memory_revision_state_events_no_truncate
 BEFORE TRUNCATE ON memory_revision_state_events
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE memory_dependencies (
 company_id text NOT NULL,
 dependency_id text NOT NULL,
 record_id text NOT NULL,
 record_revision bigint NOT NULL,
 target_kind text NOT NULL CHECK(target_kind IN ('mission_input','artifact','contract_revision','task_revision')),
 target_id text NOT NULL,
 target_revision bigint NOT NULL CHECK(target_revision>0),
 target_sha256 text NOT NULL CHECK(target_sha256 ~ '^[0-9a-f]{64}$'),
 risk_level text NOT NULL CHECK(risk_level IN ('ordinary','high','critical')),
 company_seq bigint NOT NULL,
 created_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,dependency_id),
 UNIQUE(company_id,record_id,record_revision,target_kind,target_id,target_revision),
 FOREIGN KEY(company_id,record_id,record_revision) REFERENCES memory_record_revisions(company_id,record_id,revision),
 FOREIGN KEY(company_id,company_seq) REFERENCES events(company_id,company_seq),
 FOREIGN KEY(company_id,created_by) REFERENCES employees(company_id,id)
);
CREATE INDEX memory_dependencies_target ON memory_dependencies(company_id,target_kind,target_id,target_revision,dependency_id);
CREATE TRIGGER memory_dependencies_immutable
 BEFORE UPDATE OR DELETE ON memory_dependencies
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER memory_dependencies_no_truncate
 BEFORE TRUNCATE ON memory_dependencies
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE memory_dependency_invalidation_events (
 company_id text NOT NULL,
 company_seq bigint NOT NULL,
 dependency_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('needs_revalidation','revalidated','frozen','cleared')),
 cause_kind text NOT NULL CHECK(cause_kind IN ('memory_correction','source_revoked','policy_change')),
 cause_id text NOT NULL,
 actor text NOT NULL,
 reason text NOT NULL CHECK(octet_length(reason) BETWEEN 1 AND 2048),
 PRIMARY KEY(company_id,company_seq),
 FOREIGN KEY(company_id,company_seq) REFERENCES events(company_id,company_seq),
 FOREIGN KEY(company_id,dependency_id) REFERENCES memory_dependencies(company_id,dependency_id),
 FOREIGN KEY(company_id,actor) REFERENCES employees(company_id,id)
);
CREATE INDEX memory_dependency_invalidations_latest ON memory_dependency_invalidation_events(company_id,dependency_id,company_seq DESC);
CREATE TRIGGER memory_dependency_invalidation_events_immutable
 BEFORE UPDATE OR DELETE ON memory_dependency_invalidation_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER memory_dependency_invalidation_events_no_truncate
 BEFORE TRUNCATE ON memory_dependency_invalidation_events
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM memory_records) OR EXISTS(SELECT 1 FROM memory_record_revisions) OR
    EXISTS(SELECT 1 FROM memory_revision_state_events) OR EXISTS(SELECT 1 FROM memory_dependencies) OR
    EXISTS(SELECT 1 FROM memory_dependency_invalidation_events) THEN
  RAISE EXCEPTION 'cannot roll back memory dependency history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER memory_dependency_invalidation_events_no_truncate ON memory_dependency_invalidation_events;
DROP TRIGGER memory_dependency_invalidation_events_immutable ON memory_dependency_invalidation_events;
DROP INDEX memory_dependency_invalidations_latest;
DROP TABLE memory_dependency_invalidation_events;
DROP TRIGGER memory_dependencies_no_truncate ON memory_dependencies;
DROP TRIGGER memory_dependencies_immutable ON memory_dependencies;
DROP INDEX memory_dependencies_target;
DROP TABLE memory_dependencies;
DROP TRIGGER memory_revision_state_events_no_truncate ON memory_revision_state_events;
DROP TRIGGER memory_revision_state_events_immutable ON memory_revision_state_events;
DROP INDEX memory_revision_state_latest;
DROP TABLE memory_revision_state_events;
DROP TRIGGER memory_record_revisions_no_truncate ON memory_record_revisions;
DROP TRIGGER memory_record_revisions_immutable ON memory_record_revisions;
DROP INDEX memory_record_revisions_source;
DROP TABLE memory_record_revisions;
DROP TRIGGER memory_records_no_truncate ON memory_records;
DROP TRIGGER memory_records_immutable ON memory_records;
DROP INDEX memory_records_scope;
DROP TABLE memory_records;
