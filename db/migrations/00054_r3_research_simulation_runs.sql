-- +goose Up
CREATE TABLE domain_workflow_research_simulation_runs (
 company_id text NOT NULL,
 run_id text NOT NULL,
 profile_id text NOT NULL CHECK(profile_id='research-simulation-reference'),
 profile_revision text NOT NULL CHECK(profile_revision='research-simulation@1'),
 protocol_revision bigint NOT NULL CHECK(protocol_revision>0),
 dataset_input_id text NOT NULL,
 dataset_input_revision bigint NOT NULL CHECK(dataset_input_revision>0),
 dataset_sha256 text NOT NULL CHECK(dataset_sha256 ~ '^[a-f0-9]{64}$'),
 method_input_id text NOT NULL,
 method_input_revision bigint NOT NULL CHECK(method_input_revision>0),
 method_sha256 text NOT NULL CHECK(method_sha256 ~ '^[a-f0-9]{64}$'),
 seed text NOT NULL CHECK(seed ~ '^(0|[1-9][0-9]{0,19})$' AND seed::numeric<=18446744073709551615),
 control_definition text NOT NULL CHECK(char_length(control_definition) BETWEEN 1 AND 512),
 risk_unit text NOT NULL CHECK(risk_unit='sample_draw'),
 risk_budget_units bigint NOT NULL CHECK(risk_budget_units BETWEEN 1 AND 5120000),
 risk_consumed_units bigint NOT NULL CHECK(risk_consumed_units>0 AND risk_consumed_units<=risk_budget_units),
 output_sha256 text NOT NULL CHECK(output_sha256 ~ '^[a-f0-9]{64}$'),
 output_json jsonb NOT NULL CHECK(
  jsonb_typeof(output_json)='object'
  AND COALESCE(output_json->>'schemaVersion'='polis-research-simulation-output@1',false)
  AND COALESCE(output_json->>'algorithm'='bootstrap-mean-difference@1',false)
  AND COALESCE((output_json->>'protocolRevision')::bigint=protocol_revision,false)
  AND COALESCE(output_json->>'datasetSha256'=dataset_sha256,false)
  AND COALESCE(output_json->>'methodSha256'=method_sha256,false)
  AND COALESCE(output_json->>'seed'=seed,false)
  AND COALESCE(output_json->>'controlDefinition'=control_definition,false)
  AND COALESCE((output_json->>'riskConsumedUnits')::bigint=risk_consumed_units,false)
  AND COALESCE(output_json->>'riskUnit'=risk_unit,false)
 ),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,run_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 FOREIGN KEY(company_id,dataset_input_id,dataset_input_revision) REFERENCES mission_inputs(company_id,input_id,revision),
 FOREIGN KEY(company_id,method_input_id,method_input_revision) REFERENCES mission_inputs(company_id,input_id,revision)
);

CREATE INDEX domain_workflow_research_simulation_runs_recent
 ON domain_workflow_research_simulation_runs(company_id,created_at DESC,run_id DESC);

-- +goose StatementBegin
CREATE FUNCTION validate_domain_workflow_research_simulation_sources() RETURNS trigger AS $$
DECLARE
 dataset_digest text;
 dataset_state text;
 dataset_media_type text;
 dataset_size bigint;
 method_digest text;
 method_state text;
 method_media_type text;
 method_size bigint;
BEGIN
 SELECT content_digest,state,media_type,byte_size INTO dataset_digest,dataset_state,dataset_media_type,dataset_size
 FROM mission_inputs WHERE company_id=NEW.company_id AND input_id=NEW.dataset_input_id AND revision=NEW.dataset_input_revision;
 IF NOT FOUND OR dataset_state<>'usable' OR dataset_digest<>NEW.dataset_sha256 OR dataset_size>1048576 OR
    NOT (dataset_media_type='application/json' OR dataset_media_type LIKE 'text/%') THEN
  RAISE EXCEPTION 'research simulation dataset source is not the exact usable bounded company input' USING ERRCODE='23514';
 END IF;
 SELECT content_digest,state,media_type,byte_size INTO method_digest,method_state,method_media_type,method_size
 FROM mission_inputs WHERE company_id=NEW.company_id AND input_id=NEW.method_input_id AND revision=NEW.method_input_revision;
 IF NOT FOUND OR method_state<>'usable' OR method_digest<>NEW.method_sha256 OR method_size>16384 OR
    NOT (method_media_type='application/json' OR method_media_type LIKE 'text/%') THEN
  RAISE EXCEPTION 'research simulation method source is not the exact usable bounded company input' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER domain_workflow_research_simulation_sources_valid
 BEFORE INSERT ON domain_workflow_research_simulation_runs
 FOR EACH ROW EXECUTE FUNCTION validate_domain_workflow_research_simulation_sources();

CREATE TRIGGER domain_workflow_research_simulation_runs_immutable
 BEFORE UPDATE OR DELETE ON domain_workflow_research_simulation_runs
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
