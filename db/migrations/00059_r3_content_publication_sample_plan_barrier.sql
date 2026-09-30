-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION validate_domain_workflow_content_publication_sample_plan() RETURNS trigger AS $$
DECLARE
 sample_plan_id text;
 sample_plan_revision bigint;
 sample_plan_sha256 text;
 current_source_state text;
 current_source_sha256 text;
 input_state text;
 input_sha256 text;
BEGIN
 SELECT sample->'plan'->>'inputId',(sample->'plan'->>'revision')::bigint,sample->'plan'->>'sha256'
 INTO sample_plan_id,sample_plan_revision,sample_plan_sha256
 FROM domain_workflow_content_reviews WHERE company_id=NEW.company_id AND review_id=NEW.review_id;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'content publication review is unavailable' USING ERRCODE='23514';
 END IF;
 SELECT state,source_sha256 INTO current_source_state,current_source_sha256
 FROM domain_workflow_content_source_events
 WHERE company_id=NEW.company_id AND input_id=sample_plan_id AND input_revision=sample_plan_revision
 ORDER BY event_seq DESC LIMIT 1;
 IF current_source_state IS DISTINCT FROM 'authorized' OR current_source_sha256 IS DISTINCT FROM sample_plan_sha256 THEN
  RAISE EXCEPTION 'content publication sample plan is no longer authorized' USING ERRCODE='23514';
 END IF;
 SELECT state,content_digest INTO input_state,input_sha256 FROM mission_inputs
 WHERE company_id=NEW.company_id AND input_id=sample_plan_id AND revision=sample_plan_revision;
 IF input_state IS DISTINCT FROM 'usable' OR input_sha256 IS DISTINCT FROM sample_plan_sha256 THEN
  RAISE EXCEPTION 'content publication sample plan input is stale or unusable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER domain_workflow_content_publication_sample_plan_valid
 BEFORE INSERT ON domain_workflow_content_publication_simulations
 FOR EACH ROW EXECUTE FUNCTION validate_domain_workflow_content_publication_sample_plan();
