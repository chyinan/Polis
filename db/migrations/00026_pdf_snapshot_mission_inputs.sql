-- +goose Up
ALTER TABLE mission_inputs DROP CONSTRAINT mission_inputs_source_kind_check;
ALTER TABLE mission_inputs
 ADD CONSTRAINT mission_inputs_source_kind_check
 CHECK(source_kind IN ('upload','directory_snapshot','zip_snapshot','pdf_snapshot','git_snapshot'));

-- +goose Down
ALTER TABLE mission_inputs DROP CONSTRAINT mission_inputs_source_kind_check;
ALTER TABLE mission_inputs
 ADD CONSTRAINT mission_inputs_source_kind_check
 CHECK(source_kind IN ('upload','directory_snapshot','zip_snapshot','git_snapshot'));
