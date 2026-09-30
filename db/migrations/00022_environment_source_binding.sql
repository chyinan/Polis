-- +goose Up
ALTER TABLE project_environment_revisions
 ADD COLUMN mission_id text,
 ADD COLUMN source_input_id text,
 ADD COLUMN source_input_revision bigint,
 ADD COLUMN project_root_relative text;
ALTER TABLE project_environment_revisions
 ADD CONSTRAINT project_environment_revisions_source_binding_complete
 CHECK((mission_id IS NULL AND source_input_id IS NULL AND source_input_revision IS NULL AND project_root_relative IS NULL)
    OR (mission_id IS NOT NULL AND source_input_id IS NOT NULL AND source_input_revision>0 AND project_root_relative IS NOT NULL AND octet_length(project_root_relative) BETWEEN 1 AND 1024));
ALTER TABLE project_environment_revisions
 ADD CONSTRAINT project_environment_revisions_mission_fk
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id);
ALTER TABLE project_environment_revisions
 ADD CONSTRAINT project_environment_revisions_source_input_fk
 FOREIGN KEY(company_id,source_input_id,source_input_revision) REFERENCES mission_inputs(company_id,input_id,revision);
CREATE INDEX project_environment_source_input ON project_environment_revisions(company_id,source_input_id,source_input_revision);
ALTER TABLE environment_preparation_events DROP CONSTRAINT environment_preparation_events_state_check;
ALTER TABLE environment_preparation_events
 ADD CONSTRAINT environment_preparation_events_state_check
 CHECK(state IN ('blocked_policy','blocked_source_unverified','blocked_unqualified','accepted','starting','running','ready','failed','cancelled','outcome_unknown'));

-- +goose Down
ALTER TABLE environment_preparation_events DROP CONSTRAINT environment_preparation_events_state_check;
ALTER TABLE environment_preparation_events
 ADD CONSTRAINT environment_preparation_events_state_check
 CHECK(state IN ('blocked_policy','blocked_unqualified','accepted','starting','running','ready','failed','cancelled','outcome_unknown'));
DROP INDEX project_environment_source_input;
ALTER TABLE project_environment_revisions DROP CONSTRAINT project_environment_revisions_source_input_fk;
ALTER TABLE project_environment_revisions DROP CONSTRAINT project_environment_revisions_mission_fk;
ALTER TABLE project_environment_revisions DROP CONSTRAINT project_environment_revisions_source_binding_complete;
ALTER TABLE project_environment_revisions DROP COLUMN project_root_relative;
ALTER TABLE project_environment_revisions DROP COLUMN source_input_revision;
ALTER TABLE project_environment_revisions DROP COLUMN source_input_id;
ALTER TABLE project_environment_revisions DROP COLUMN mission_id;
