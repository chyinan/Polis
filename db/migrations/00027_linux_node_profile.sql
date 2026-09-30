-- +goose Up
ALTER TABLE project_environment_revisions
    DROP CONSTRAINT project_environment_revisions_profile_id_check,
    ADD CONSTRAINT project_environment_revisions_profile_id_check
        CHECK(profile_id IN ('windows-node-npm@1','linux-node-npm@1'));

ALTER TABLE environment_executor_qualification_events
    DROP CONSTRAINT environment_executor_qualification_events_profile_id_check,
    ADD CONSTRAINT environment_executor_qualification_events_profile_id_check
        CHECK(profile_id IN ('windows-node-npm@1','linux-node-npm@1'));

-- +goose Down
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM project_environment_revisions WHERE profile_id='linux-node-npm@1')
       OR EXISTS (SELECT 1 FROM environment_executor_qualification_events WHERE profile_id='linux-node-npm@1') THEN
        RAISE EXCEPTION 'cannot remove Linux Node profile rows during schema downgrade';
    END IF;
END $$;

ALTER TABLE environment_executor_qualification_events
    DROP CONSTRAINT environment_executor_qualification_events_profile_id_check,
    ADD CONSTRAINT environment_executor_qualification_events_profile_id_check
        CHECK(profile_id IN ('windows-node-npm@1'));

ALTER TABLE project_environment_revisions
    DROP CONSTRAINT project_environment_revisions_profile_id_check,
    ADD CONSTRAINT project_environment_revisions_profile_id_check
        CHECK(profile_id IN ('windows-node-npm@1'));
