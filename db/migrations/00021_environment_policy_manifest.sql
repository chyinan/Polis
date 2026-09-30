-- +goose Up
ALTER TABLE project_environment_revisions
 ADD COLUMN policy_manifest jsonb;
ALTER TABLE project_environment_revisions
 ADD CONSTRAINT project_environment_revisions_policy_manifest_object
 CHECK(policy_manifest IS NULL OR jsonb_typeof(policy_manifest)='object');

-- +goose Down
ALTER TABLE project_environment_revisions DROP CONSTRAINT project_environment_revisions_policy_manifest_object;
ALTER TABLE project_environment_revisions DROP COLUMN policy_manifest;
