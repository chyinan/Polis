-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION validate_domain_workflow_content_source_event() RETURNS trigger AS $$
DECLARE
 source_digest text;
 source_state text;
 source_media_type text;
 source_size bigint;
BEGIN
 IF NEW.state='revoked' THEN
  SELECT state INTO source_state FROM domain_workflow_content_source_events
  WHERE company_id=NEW.company_id AND input_id=NEW.input_id AND input_revision=NEW.input_revision
  ORDER BY event_seq DESC LIMIT 1;
  IF source_state IS DISTINCT FROM 'authorized' THEN
   RAISE EXCEPTION 'only a currently authorized content source can be revoked' USING ERRCODE='23514';
  END IF;
 END IF;
 IF NEW.state='authorized' THEN
  SELECT content_digest,state,media_type,byte_size INTO source_digest,source_state,source_media_type,source_size
  FROM mission_inputs WHERE company_id=NEW.company_id AND input_id=NEW.input_id AND revision=NEW.input_revision;
  IF NOT FOUND OR source_state<>'usable' OR source_digest<>NEW.source_sha256 OR source_size<1 OR source_size>1048576 OR
     NOT (source_media_type='application/json' OR source_media_type LIKE 'text/%') THEN
   RAISE EXCEPTION 'content source is not an exact usable bounded company MissionInput' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
