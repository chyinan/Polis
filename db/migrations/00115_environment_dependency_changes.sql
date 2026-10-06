-- +goose Up
-- DependencyChange proposals are immutable owner-reviewed intent. Approval
-- never materializes a lockfile or runs a package manager; a qualified
-- executor must later produce a new immutable environment revision.
ALTER TABLE project_environment_revisions
 ADD CONSTRAINT project_environment_revisions_company_revision_mission_unique UNIQUE(company_id,revision_id,mission_id);

CREATE TABLE environment_dependency_change_proposals (
 company_id text NOT NULL,
 proposal_id text NOT NULL,
 mission_id text NOT NULL,
 base_revision_id text NOT NULL,
 proposal jsonb NOT NULL CHECK(jsonb_typeof(proposal)='object' AND octet_length(proposal::text)<=65536),
 proposal_sha256 text NOT NULL CHECK(proposal_sha256 ~ '^[a-f0-9]{64}$'),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,proposal_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,base_revision_id,mission_id) REFERENCES project_environment_revisions(company_id,revision_id,mission_id)
);
CREATE INDEX environment_dependency_change_proposals_mission
 ON environment_dependency_change_proposals(company_id,mission_id,created_at DESC);
CREATE TABLE environment_dependency_change_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL,
 proposal_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('proposed','approved','rejected','blocked')),
 actor text NOT NULL CHECK(octet_length(btrim(actor)) BETWEEN 1 AND 80),
 rationale text NOT NULL CHECK(octet_length(btrim(rationale)) BETWEEN 1 AND 4096),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,proposal_id) REFERENCES environment_dependency_change_proposals(company_id,proposal_id)
);
CREATE INDEX environment_dependency_change_events_latest
 ON environment_dependency_change_events(company_id,proposal_id,event_seq DESC);
CREATE TRIGGER environment_dependency_change_proposals_immutable
 BEFORE UPDATE OR DELETE ON environment_dependency_change_proposals
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER environment_dependency_change_proposals_no_truncate
 BEFORE TRUNCATE ON environment_dependency_change_proposals
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER environment_dependency_change_events_immutable
 BEFORE UPDATE OR DELETE ON environment_dependency_change_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER environment_dependency_change_events_no_truncate
 BEFORE TRUNCATE ON environment_dependency_change_events
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM environment_dependency_change_events) OR EXISTS(SELECT 1 FROM environment_dependency_change_proposals) THEN
  RAISE EXCEPTION 'cannot discard dependency change history';
 END IF;
END
$$;
DROP TRIGGER environment_dependency_change_events_immutable ON environment_dependency_change_events;
DROP TRIGGER environment_dependency_change_events_no_truncate ON environment_dependency_change_events;
DROP INDEX environment_dependency_change_events_latest;
DROP TABLE environment_dependency_change_events;
DROP TRIGGER environment_dependency_change_proposals_immutable ON environment_dependency_change_proposals;
DROP TRIGGER environment_dependency_change_proposals_no_truncate ON environment_dependency_change_proposals;
DROP INDEX environment_dependency_change_proposals_mission;
DROP TABLE environment_dependency_change_proposals;
ALTER TABLE project_environment_revisions DROP CONSTRAINT project_environment_revisions_company_revision_mission_unique;
