-- +goose Up
ALTER TABLE domain_workflow_research_simulation_runs
 ADD CONSTRAINT domain_workflow_research_simulation_runs_risk_equation_check
 CHECK(COALESCE(
  jsonb_typeof(output_json->'iterations')='number' AND
  (output_json->>'iterations')::numeric BETWEEN 1 AND 10000 AND
  jsonb_typeof(output_json->'sampleSize')='number' AND
  (output_json->>'sampleSize')::numeric BETWEEN 1 AND 256 AND
  jsonb_typeof(output_json->'riskConsumedUnits')='number' AND
  (output_json->>'riskConsumedUnits')::numeric=risk_consumed_units::numeric AND
  risk_consumed_units::numeric=2::numeric*(output_json->>'iterations')::numeric*(output_json->>'sampleSize')::numeric,
  false));
