-- +goose Up
ALTER TABLE capability_qualification_records
 DROP CONSTRAINT capability_qualification_records_profile_check;
ALTER TABLE capability_qualification_records
 ADD CONSTRAINT capability_qualification_records_profile_check
 CHECK(profile IN ('read_only_skill@1','stdio_mcp@1','streamable_http_mcp_2026_07_28@1'));

-- +goose Down
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM capability_qualification_records WHERE profile='streamable_http_mcp_2026_07_28@1') THEN
  RAISE EXCEPTION 'cannot remove Streamable HTTP qualifications';
 END IF;
END $$;
ALTER TABLE capability_qualification_records
 DROP CONSTRAINT capability_qualification_records_profile_check;
ALTER TABLE capability_qualification_records
 ADD CONSTRAINT capability_qualification_records_profile_check
 CHECK(profile IN ('read_only_skill@1','stdio_mcp@1'));
