-- +goose Up
-- A materialization attempt is a durable, single-use fence around a qualified
-- resolver. State changes are events; unknown attempts are never replayed.
CREATE TABLE environment_dependency_change_materializations (
 company_id text NOT NULL,
 materialization_id text NOT NULL,
 proposal_id text NOT NULL,
 base_revision_id text NOT NULL,
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 request_sha256 text NOT NULL CHECK(request_sha256 ~ '^[a-f0-9]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,materialization_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,proposal_id) REFERENCES environment_dependency_change_proposals(company_id,proposal_id),
 FOREIGN KEY(company_id,base_revision_id) REFERENCES project_environment_revisions(company_id,revision_id)
);
CREATE INDEX environment_dependency_change_materializations_proposal
 ON environment_dependency_change_materializations(company_id,proposal_id,created_at DESC);

CREATE TABLE environment_dependency_change_materialization_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL,
 materialization_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('running','blocked','failed','outcome_unknown','materialized')),
 reason_code text NOT NULL CHECK(octet_length(reason_code)<=128),
 derived_input_id text,
 derived_input_revision bigint,
 successor_revision_id text,
 package_json_sha256 text,
 lockfile_sha256 text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 FOREIGN KEY(company_id,materialization_id) REFERENCES environment_dependency_change_materializations(company_id,materialization_id),
 FOREIGN KEY(company_id,derived_input_id,derived_input_revision) REFERENCES mission_inputs(company_id,input_id,revision),
 FOREIGN KEY(company_id,successor_revision_id) REFERENCES project_environment_revisions(company_id,revision_id),
 CHECK((state='materialized' AND derived_input_id IS NOT NULL AND derived_input_revision>0 AND successor_revision_id IS NOT NULL AND package_json_sha256 IS NOT NULL AND package_json_sha256 ~ '^[a-f0-9]{64}$' AND lockfile_sha256 IS NOT NULL AND lockfile_sha256 ~ '^[a-f0-9]{64}$')
    OR (state<>'materialized' AND derived_input_id IS NULL AND derived_input_revision IS NULL AND successor_revision_id IS NULL AND package_json_sha256 IS NULL AND lockfile_sha256 IS NULL))
);
CREATE INDEX environment_dependency_change_materialization_events_latest
 ON environment_dependency_change_materialization_events(company_id,materialization_id,event_seq DESC);
CREATE TRIGGER environment_dependency_change_materializations_immutable
 BEFORE UPDATE OR DELETE ON environment_dependency_change_materializations
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER environment_dependency_change_materializations_no_truncate
 BEFORE TRUNCATE ON environment_dependency_change_materializations
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER environment_dependency_change_materialization_events_immutable
 BEFORE UPDATE OR DELETE ON environment_dependency_change_materialization_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER environment_dependency_change_materialization_events_no_truncate
 BEFORE TRUNCATE ON environment_dependency_change_materialization_events
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM environment_dependency_change_materialization_events) OR EXISTS(SELECT 1 FROM environment_dependency_change_materializations) THEN
  RAISE EXCEPTION 'cannot discard dependency change materialization history';
 END IF;
END
$$;
DROP TRIGGER environment_dependency_change_materialization_events_immutable ON environment_dependency_change_materialization_events;
DROP TRIGGER environment_dependency_change_materialization_events_no_truncate ON environment_dependency_change_materialization_events;
DROP INDEX environment_dependency_change_materialization_events_latest;
DROP TABLE environment_dependency_change_materialization_events;
DROP TRIGGER environment_dependency_change_materializations_immutable ON environment_dependency_change_materializations;
DROP TRIGGER environment_dependency_change_materializations_no_truncate ON environment_dependency_change_materializations;
DROP INDEX environment_dependency_change_materializations_proposal;
DROP TABLE environment_dependency_change_materializations;
