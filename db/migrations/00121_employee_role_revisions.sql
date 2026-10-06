-- +goose Up
-- Owner-confirmed semantic role revisions are durable context only. The
-- qualification remains unverified and does not authorize Worker admission.
-- +goose StatementBegin
CREATE FUNCTION fixed_team_coverage_snapshot_base64() RETURNS text
LANGUAGE SQL IMMUTABLE AS $$
 SELECT 'ewogICJkb2N1bWVudF92ZXJzaW9uIjogIjAuNC41IiwKICAiZGVzaWduX2tpbmQiOiAiZml4ZWRfdGVhbV9jb3ZlcmFnZV9kcmFmdCIsCiAgImV4ZWN1dGlvbl9lbmFibGVkIjogZmFsc2UsCiAgInJvbGVfY2hhbmdlc19hdF9ydW50aW1lIjogZmFsc2UsCiAgInRlbXBsYXRlX3JlcXVpcmVzX2h1bWFuX2NvbmZpcm1hdGlvbiI6IHRydWUsCiAgImVtcGxveWVlX2lkcyI6IFsKICAgICJlbXAtcGxhbm5pbmciLAogICAgImVtcC1iYWNrZW5kIiwKICAgICJlbXAtZnJvbnRlbmQiLAogICAgImVtcC1yZXZpZXciCiAgXSwKICAiY292ZXJhZ2UiOiBbCiAgICB7CiAgICAgICJ0YXNrX3R5cGUiOiAicGxhbm5pbmciLAogICAgICAib3duZXIiOiAiZW1wLXBsYW5uaW5nIiwKICAgICAgImVsaWdpYmxlX2luZGVwZW5kZW50X2NoZWNrZXJzIjogWwogICAgICAgICJlbXAtcmV2aWV3IgogICAgICBdLAogICAgICAiYWNjZXB0YW5jZV9wYXRoIjogIm93bmVyX3Byb3RlY3RlZF9nb2FsX2NvbnRyYWN0IiwKICAgICAgInF1YWxpZmljYXRpb24iOiAidW52ZXJpZmllZCIKICAgIH0sCiAgICB7CiAgICAgICJ0YXNrX3R5cGUiOiAiZnJvbnRlbmQiLAogICAgICAib3duZXIiOiAiZW1wLWZyb250ZW5kIiwKICAgICAgImVsaWdpYmxlX2luZGVwZW5kZW50X2NoZWNrZXJzIjogWwogICAgICAgICJlbXAtcmV2aWV3IgogICAgICBdLAogICAgICAiYWNjZXB0YW5jZV9wYXRoIjogInRydXN0ZWRfYmFzZWxpbmVfcGx1c19pbmRlcGVuZGVudF9yZXZpZXciLAogICAgICAicXVhbGlmaWNhdGlvbiI6ICJ1bnZlcmlmaWVkIgogICAgfSwKICAgIHsKICAgICAgInRhc2tfdHlwZSI6ICJiYWNrZW5kIiwKICAgICAgIm93bmVyIjogImVtcC1iYWNrZW5kIiwKICAgICAgImVsaWdpYmxlX2luZGVwZW5kZW50X2NoZWNrZXJzIjogWwogICAgICAgICJlbXAtcmV2aWV3IgogICAgICBdLAogICAgICAiYWNjZXB0YW5jZV9wYXRoIjogInRydXN0ZWRfYmFzZWxpbmVfcGx1c19pbmRlcGVuZGVudF9yZXZpZXciLAogICAgICAicXVhbGlmaWNhdGlvbiI6ICJ1bnZlcmlmaWVkIgogICAgfSwKICAgIHsKICAgICAgInRhc2tfdHlwZSI6ICJlbnZpcm9ubWVudF9wbGFuIiwKICAgICAgIm93bmVyIjogImVtcC1iYWNrZW5kIiwKICAgICAgImVsaWdpYmxlX2luZGVwZW5kZW50X2NoZWNrZXJzIjogWwogICAgICAgICJlbXAtcGxhbm5pbmciCiAgICAgIF0sCiAgICAgICJhY2NlcHRhbmNlX3BhdGgiOiAiY3VycmVudF9lbnZpcm9ubWVudF9wb2xpY3kiLAogICAgICAicXVhbGlmaWNhdGlvbiI6ICJ1bnZlcmlmaWVkIgogICAgfSwKICAgIHsKICAgICAgInRhc2tfdHlwZSI6ICJyZWdyZXNzaW9uX3Rlc3Rfb3ZlcmxheSIsCiAgICAgICJvd25lciI6ICJlbXAtcmV2aWV3IiwKICAgICAgImVsaWdpYmxlX2luZGVwZW5kZW50X2NoZWNrZXJzIjogWwogICAgICAgICJlbXAtYmFja2VuZCIsCiAgICAgICAgImVtcC1mcm9udGVuZCIKICAgICAgXSwKICAgICAgImFjY2VwdGFuY2VfcGF0aCI6ICJzZXBhcmF0ZV90ZXN0X3F1YWxpdHlfcmV2aWV3X25vdF9maW5hbF9wcm9kdWN0aW9uX2FjY2VwdGFuY2UiLAogICAgICAicXVhbGlmaWNhdGlvbiI6ICJ1bnZlcmlmaWVkIgogICAgfSwKICAgIHsKICAgICAgInRhc2tfdHlwZSI6ICJkZXNpZ25fY2hhbmdlIiwKICAgICAgIm93bmVyIjogImVtcC1wbGFubmluZyIsCiAgICAgICJlbGlnaWJsZV9pbmRlcGVuZGVudF9jaGVja2VycyI6IFsKICAgICAgICAiZW1wLXJldmlldyIKICAgICAgXSwKICAgICAgImFjY2VwdGFuY2VfcGF0aCI6ICJyaXNrX3BvbGljeV9vd25lcl9pZl9vdXRzaWRlX3Njb3BlIiwKICAgICAgInF1YWxpZmljYXRpb24iOiAidW52ZXJpZmllZCIKICAgIH0sCiAgICB7CiAgICAgICJ0YXNrX3R5cGUiOiAiZGVsaXZlcnlfYXNzZW1ibHkiLAogICAgICAib3duZXIiOiAiZW1wLXBsYW5uaW5nIiwKICAgICAgImVsaWdpYmxlX2luZGVwZW5kZW50X2NoZWNrZXJzIjogWwogICAgICAgICJlbXAtcmV2aWV3IgogICAgICBdLAogICAgICAiYWNjZXB0YW5jZV9wYXRoIjogImltbXV0YWJsZV9tYW5pZmVzdF9hbmRfZGV0ZXJtaW5pc3RpY19jaGVja3MiLAogICAgICAicXVhbGlmaWNhdGlvbiI6ICJ1bnZlcmlmaWVkIgogICAgfQogIF0sCiAgIm1pc3NpbmdfcGF0aCI6ICJzaG93X3JlcXVpcmVzX2h1bWFuX2JlZm9yZV9hZG1pc3Npb24iLAogICJjaGVja2VyX3BvbGljeSI6ICJub3QgYXV0aG9yIG9mIGV4YWN0IGNhbmRpZGF0ZTsgdGVzdC1vdmVybGF5IHJldmlldyBkb2VzIG5vdCBwZXJtaXQgcHJvZHVjdGlvbiBzZWxmLWFjY2VwdGFuY2UiLAogICJ0cnVzdGVkX2Jhc2VsaW5lX211dGFibGVfYnlfd29ya2VycyI6IGZhbHNlLAogICJndWFyYW50ZWVzX3NlbWFudGljX2luZGVwZW5kZW5jZSI6IGZhbHNlCn0K'
$$;
-- +goose StatementEnd
CREATE TABLE employee_role_revisions (
 company_id text NOT NULL,
 employee_id text NOT NULL,
 revision_sha256 text NOT NULL CHECK(revision_sha256 ~ '^[a-f0-9]{64}$'),
 confirmation_company_seq bigint NOT NULL,
 role_name text NOT NULL CHECK(octet_length(btrim(role_name)) BETWEEN 1 AND 200),
 task_types jsonb NOT NULL CHECK(jsonb_typeof(task_types)='array'),
 task_kinds jsonb NOT NULL CHECK(jsonb_typeof(task_kinds)='array'),
 owner_decision text NOT NULL CHECK(owner_decision='installation_owner_confirmed_fixed_team_mapping'),
 qualification text NOT NULL CHECK(qualification='unverified'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,employee_id,revision_sha256),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,confirmation_company_seq) REFERENCES events(company_id,company_seq)
);
-- Backfill every current owner confirmation so Schema121 does not leave
-- already-confirmed Companies without their per-employee semantic rows.
WITH raw_confirmations AS (
 SELECT company_id,company_seq,payload,
  convert_from(decode(payload->>'team_coverage_snapshot_base64','base64'),'UTF8')::jsonb AS snapshot
 FROM events WHERE kind='company.team_coverage.confirmed'
), latest_confirmation AS (
 SELECT DISTINCT ON(company_id) company_id,company_seq,payload,snapshot
 FROM raw_confirmations
 WHERE payload->>'template_sha256'='8dcd1c20d7b0db14e77b85ff5d4379b2cd8829f24bfd70383a7c2651dedab770'
  AND payload->>'team_coverage_snapshot_base64'=fixed_team_coverage_snapshot_base64()
  AND payload->>'decision'='installation_owner_confirmed_fixed_team_mapping'
  AND payload->>'qualification'='unverified'
  AND snapshot->>'document_version'='0.4.5'
  AND snapshot->>'design_kind'='fixed_team_coverage_draft'
  AND snapshot->>'execution_enabled'='false'
  AND snapshot->>'role_changes_at_runtime'='false'
  AND snapshot->>'template_requires_human_confirmation'='true'
  AND snapshot->>'missing_path'='show_requires_human_before_admission'
  AND snapshot->>'checker_policy'='not author of exact candidate; test-overlay review does not permit production self-acceptance'
  AND snapshot->>'trusted_baseline_mutable_by_workers'='false'
  AND snapshot->>'guarantees_semantic_independence'='false'
  AND snapshot->'employee_ids'='["emp-planning","emp-backend","emp-frontend","emp-review"]'::jsonb
  AND jsonb_array_length(snapshot->'coverage')=7
  AND NOT EXISTS (
   SELECT 1 FROM jsonb_array_elements(snapshot->'coverage') a
   WHERE NOT (
    (a->>'task_type'='planning' AND a->>'owner'='emp-planning' AND a->>'acceptance_path'='owner_protected_goal_contract' AND a->>'qualification'='unverified' AND a->'eligible_independent_checkers'='["emp-review"]'::jsonb) OR
    (a->>'task_type'='frontend' AND a->>'owner'='emp-frontend' AND a->>'acceptance_path'='trusted_baseline_plus_independent_review' AND a->>'qualification'='unverified' AND a->'eligible_independent_checkers'='["emp-review"]'::jsonb) OR
    (a->>'task_type'='backend' AND a->>'owner'='emp-backend' AND a->>'acceptance_path'='trusted_baseline_plus_independent_review' AND a->>'qualification'='unverified' AND a->'eligible_independent_checkers'='["emp-review"]'::jsonb) OR
    (a->>'task_type'='environment_plan' AND a->>'owner'='emp-backend' AND a->>'acceptance_path'='current_environment_policy' AND a->>'qualification'='unverified' AND a->'eligible_independent_checkers'='["emp-planning"]'::jsonb) OR
    (a->>'task_type'='regression_test_overlay' AND a->>'owner'='emp-review' AND a->>'acceptance_path'='separate_test_quality_review_not_final_production_acceptance' AND a->>'qualification'='unverified' AND a->'eligible_independent_checkers'='["emp-backend","emp-frontend"]'::jsonb) OR
    (a->>'task_type'='design_change' AND a->>'owner'='emp-planning' AND a->>'acceptance_path'='risk_policy_owner_if_outside_scope' AND a->>'qualification'='unverified' AND a->'eligible_independent_checkers'='["emp-review"]'::jsonb) OR
    (a->>'task_type'='delivery_assembly' AND a->>'owner'='emp-planning' AND a->>'acceptance_path'='immutable_manifest_and_deterministic_checks' AND a->>'qualification'='unverified' AND a->'eligible_independent_checkers'='["emp-review"]'::jsonb)
   )
  )
  AND (SELECT count(DISTINCT a.value->>'task_type') FROM jsonb_array_elements(snapshot->'coverage') a)=7
  AND snapshot->'coverage' = jsonb_build_array(
   jsonb_build_object('task_type','planning','owner','emp-planning','eligible_independent_checkers',jsonb_build_array('emp-review'),'acceptance_path','owner_protected_goal_contract','qualification','unverified'),
   jsonb_build_object('task_type','frontend','owner','emp-frontend','eligible_independent_checkers',jsonb_build_array('emp-review'),'acceptance_path','trusted_baseline_plus_independent_review','qualification','unverified'),
   jsonb_build_object('task_type','backend','owner','emp-backend','eligible_independent_checkers',jsonb_build_array('emp-review'),'acceptance_path','trusted_baseline_plus_independent_review','qualification','unverified'),
   jsonb_build_object('task_type','environment_plan','owner','emp-backend','eligible_independent_checkers',jsonb_build_array('emp-planning'),'acceptance_path','current_environment_policy','qualification','unverified'),
   jsonb_build_object('task_type','regression_test_overlay','owner','emp-review','eligible_independent_checkers',jsonb_build_array('emp-backend','emp-frontend'),'acceptance_path','separate_test_quality_review_not_final_production_acceptance','qualification','unverified'),
   jsonb_build_object('task_type','design_change','owner','emp-planning','eligible_independent_checkers',jsonb_build_array('emp-review'),'acceptance_path','risk_policy_owner_if_outside_scope','qualification','unverified'),
   jsonb_build_object('task_type','delivery_assembly','owner','emp-planning','eligible_independent_checkers',jsonb_build_array('emp-review'),'acceptance_path','immutable_manifest_and_deterministic_checks','qualification','unverified')
  )
 ORDER BY company_id,company_seq DESC
), assignments AS (
 SELECT c.company_id,c.company_seq,c.payload,e.id,e.role_name,a.value->>'task_type' AS task_type,a.ordinal
 FROM latest_confirmation c
 JOIN employees e ON e.company_id=c.company_id AND e.enabled
 CROSS JOIN LATERAL jsonb_array_elements(c.snapshot->'coverage') WITH ORDINALITY a(value,ordinal)
 WHERE a.value->>'owner'=e.id AND e.role_name=CASE e.id WHEN 'emp-planning' THEN 'planning' WHEN 'emp-backend' THEN 'backend' WHEN 'emp-frontend' THEN 'frontend' WHEN 'emp-review' THEN 'review' END
), grouped AS (
 SELECT company_id,company_seq,payload,id,role_name,
  jsonb_agg(to_jsonb(task_type) ORDER BY ordinal) AS task_types,
  jsonb_agg(to_jsonb(CASE task_type
   WHEN 'planning' THEN 'bootstrap_plan'
   WHEN 'design_change' THEN 'bootstrap_plan'
   WHEN 'frontend' THEN 'peer_frontend'
   WHEN 'backend' THEN 'compat'
   WHEN 'environment_plan' THEN 'compute'
   WHEN 'delivery_assembly' THEN 'compute'
   WHEN 'regression_test_overlay' THEN 'review'
   ELSE '' END) ORDER BY ordinal) AS task_kinds
 FROM assignments GROUP BY company_id,company_seq,payload,id,role_name
)
INSERT INTO employee_role_revisions(company_id,employee_id,revision_sha256,confirmation_company_seq,role_name,task_types,task_kinds,owner_decision,qualification)
SELECT company_id,id,payload->>'template_sha256',company_seq,role_name,task_types,task_kinds,payload->>'decision',payload->>'qualification'
FROM grouped
WHERE payload->>'template_sha256'='8dcd1c20d7b0db14e77b85ff5d4379b2cd8829f24bfd70383a7c2651dedab770'
 AND payload->>'decision'='installation_owner_confirmed_fixed_team_mapping' AND payload->>'qualification'='unverified'
ON CONFLICT(company_id,employee_id,revision_sha256) DO NOTHING;
DO $$
BEGIN
 IF EXISTS(
  SELECT 1 FROM (
   SELECT DISTINCT ON(company_id) company_id,payload->>'template_sha256' AS revision_sha256
   FROM events WHERE kind='company.team_coverage.confirmed'
   ORDER BY company_id,company_seq DESC
  ) confirmations
  WHERE (SELECT count(*) FROM employees e WHERE e.company_id=confirmations.company_id AND e.enabled)<>4
     OR (SELECT count(*) FROM employee_role_revisions r WHERE r.company_id=confirmations.company_id AND r.revision_sha256=confirmations.revision_sha256)<>4
 ) THEN
  RAISE EXCEPTION 'cannot backfill an incomplete fixed-team role revision';
 END IF;
END
$$;
CREATE INDEX employee_role_revisions_current ON employee_role_revisions(company_id,employee_id,created_at DESC,revision_sha256);
-- +goose StatementBegin
CREATE FUNCTION validate_employee_role_revision_provenance() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
 event_kind text;
 event_digest text;
 event_decision text;
 event_qualification text;
 snapshot_base64 text;
 snapshot jsonb;
BEGIN
 SELECT kind,payload->>'template_sha256',payload->>'decision',payload->>'qualification',payload->>'team_coverage_snapshot_base64',convert_from(decode(payload->>'team_coverage_snapshot_base64','base64'),'UTF8')::jsonb
 INTO event_kind,event_digest,event_decision,event_qualification,snapshot_base64,snapshot
 FROM events WHERE company_id=NEW.company_id AND company_seq=NEW.confirmation_company_seq;
 IF NEW.revision_sha256 IS DISTINCT FROM '8dcd1c20d7b0db14e77b85ff5d4379b2cd8829f24bfd70383a7c2651dedab770' OR event_kind IS DISTINCT FROM 'company.team_coverage.confirmed' OR event_digest IS DISTINCT FROM NEW.revision_sha256 OR event_digest IS DISTINCT FROM '8dcd1c20d7b0db14e77b85ff5d4379b2cd8829f24bfd70383a7c2651dedab770' OR event_decision IS DISTINCT FROM NEW.owner_decision OR event_qualification IS DISTINCT FROM NEW.qualification OR snapshot_base64 IS DISTINCT FROM fixed_team_coverage_snapshot_base64() OR snapshot IS NULL OR
    snapshot->'employee_ids' IS DISTINCT FROM '["emp-planning","emp-backend","emp-frontend","emp-review"]'::jsonb OR
    snapshot->>'document_version' IS DISTINCT FROM '0.4.5' OR snapshot->>'design_kind' IS DISTINCT FROM 'fixed_team_coverage_draft' OR snapshot->>'execution_enabled' IS DISTINCT FROM 'false' OR
    snapshot->>'role_changes_at_runtime' IS DISTINCT FROM 'false' OR snapshot->>'template_requires_human_confirmation' IS DISTINCT FROM 'true' OR
    snapshot->>'missing_path' IS DISTINCT FROM 'show_requires_human_before_admission' OR
    snapshot->>'checker_policy' IS DISTINCT FROM 'not author of exact candidate; test-overlay review does not permit production self-acceptance' OR
    snapshot->>'trusted_baseline_mutable_by_workers' IS DISTINCT FROM 'false' OR snapshot->>'guarantees_semantic_independence' IS DISTINCT FROM 'false' OR
    snapshot->'coverage' IS DISTINCT FROM jsonb_build_array(
      jsonb_build_object('task_type','planning','owner','emp-planning','eligible_independent_checkers',jsonb_build_array('emp-review'),'acceptance_path','owner_protected_goal_contract','qualification','unverified'),
      jsonb_build_object('task_type','frontend','owner','emp-frontend','eligible_independent_checkers',jsonb_build_array('emp-review'),'acceptance_path','trusted_baseline_plus_independent_review','qualification','unverified'),
      jsonb_build_object('task_type','backend','owner','emp-backend','eligible_independent_checkers',jsonb_build_array('emp-review'),'acceptance_path','trusted_baseline_plus_independent_review','qualification','unverified'),
      jsonb_build_object('task_type','environment_plan','owner','emp-backend','eligible_independent_checkers',jsonb_build_array('emp-planning'),'acceptance_path','current_environment_policy','qualification','unverified'),
      jsonb_build_object('task_type','regression_test_overlay','owner','emp-review','eligible_independent_checkers',jsonb_build_array('emp-backend','emp-frontend'),'acceptance_path','separate_test_quality_review_not_final_production_acceptance','qualification','unverified'),
      jsonb_build_object('task_type','design_change','owner','emp-planning','eligible_independent_checkers',jsonb_build_array('emp-review'),'acceptance_path','risk_policy_owner_if_outside_scope','qualification','unverified'),
      jsonb_build_object('task_type','delivery_assembly','owner','emp-planning','eligible_independent_checkers',jsonb_build_array('emp-review'),'acceptance_path','immutable_manifest_and_deterministic_checks','qualification','unverified')
    ) OR
    (NEW.employee_id IS NULL OR NEW.employee_id NOT IN ('emp-planning','emp-backend','emp-frontend','emp-review')) OR
    (NEW.employee_id='emp-planning' AND (NEW.role_name IS DISTINCT FROM 'planning' OR NEW.task_types IS DISTINCT FROM '["planning","design_change","delivery_assembly"]'::jsonb OR NEW.task_kinds IS DISTINCT FROM '["bootstrap_plan","bootstrap_plan","compute"]'::jsonb)) OR
    (NEW.employee_id='emp-backend' AND (NEW.role_name IS DISTINCT FROM 'backend' OR NEW.task_types IS DISTINCT FROM '["backend","environment_plan"]'::jsonb OR NEW.task_kinds IS DISTINCT FROM '["compat","compute"]'::jsonb)) OR
    (NEW.employee_id='emp-frontend' AND (NEW.role_name IS DISTINCT FROM 'frontend' OR NEW.task_types IS DISTINCT FROM '["frontend"]'::jsonb OR NEW.task_kinds IS DISTINCT FROM '["peer_frontend"]'::jsonb)) OR
    (NEW.employee_id='emp-review' AND (NEW.role_name IS DISTINCT FROM 'review' OR NEW.task_types IS DISTINCT FROM '["regression_test_overlay"]'::jsonb OR NEW.task_kinds IS DISTINCT FROM '["review"]'::jsonb)) THEN
  RAISE EXCEPTION 'employee role revision has invalid confirmation provenance' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER employee_role_revisions_provenance
 BEFORE INSERT ON employee_role_revisions
 FOR EACH ROW EXECUTE FUNCTION validate_employee_role_revision_provenance();
CREATE TRIGGER employee_role_revisions_immutable
 BEFORE UPDATE OR DELETE ON employee_role_revisions
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER employee_role_revisions_no_truncate
 BEFORE TRUNCATE ON employee_role_revisions FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM employee_role_revisions) THEN
  RAISE EXCEPTION 'cannot discard employee role revision history';
 END IF;
END
$$;
DROP TRIGGER employee_role_revisions_no_truncate ON employee_role_revisions;
DROP TRIGGER employee_role_revisions_immutable ON employee_role_revisions;
DROP TRIGGER employee_role_revisions_provenance ON employee_role_revisions;
DROP FUNCTION validate_employee_role_revision_provenance();
DROP FUNCTION fixed_team_coverage_snapshot_base64();
DROP INDEX employee_role_revisions_current;
DROP TABLE employee_role_revisions;
