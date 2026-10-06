-- +goose Up
-- BrowserRun is a control-plane record. This schema deliberately stores no
-- cookies, storageState, bearer handles, downloaded bytes or browser output.
CREATE TABLE browser_runs (
 company_id text NOT NULL,
 run_id text NOT NULL,
 mission_id text NOT NULL,
 task_id text NOT NULL,
 service_task_id text NOT NULL,
 session_id text NOT NULL,
 service_job_id text NOT NULL,
 service_generation integer NOT NULL CHECK(service_generation>0),
 target_origin text NOT NULL CHECK(target_origin ~ '^https://[^/?#]+$'),
 CHECK(target_origin=lower(target_origin) AND substring(target_origin from 9) !~ '[/#?@]'),
 plan_sha256 text NOT NULL CHECK(plan_sha256 ~ '^[a-f0-9]{64}$'),
 browser_build text NOT NULL CHECK(octet_length(btrim(browser_build)) BETWEEN 1 AND 128),
 execution_environment text NOT NULL CHECK(octet_length(btrim(execution_environment)) BETWEEN 1 AND 128),
 input_revision text NOT NULL,
 viewport_width integer NOT NULL CHECK(viewport_width BETWEEN 1 AND 4096),
 viewport_height integer NOT NULL CHECK(viewport_height BETWEEN 1 AND 4096),
 locale text NOT NULL CHECK(octet_length(btrim(locale)) BETWEEN 1 AND 128),
 timezone text NOT NULL CHECK(octet_length(btrim(timezone)) BETWEEN 1 AND 128),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,run_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,service_task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,session_id,task_id) REFERENCES worker_sessions(company_id,id,task_id),
 FOREIGN KEY(company_id,service_job_id,service_task_id) REFERENCES job_runs(company_id,job_id,task_id)
);
CREATE INDEX browser_runs_task_recent ON browser_runs(company_id,task_id,created_at DESC,run_id DESC);
CREATE TABLE browser_run_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL,
 run_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('blocked','requested','running','succeeded','failed','cancelled','unknown')),
 reason_code text NOT NULL CHECK(reason_code ~ '^[a-z0-9_:-]{1,128}$'),
 evidence_manifest_sha256 text CHECK(evidence_manifest_sha256 IS NULL OR evidence_manifest_sha256 ~ '^[a-f0-9]{64}$'),
 evidence jsonb NOT NULL DEFAULT '{}'::jsonb CHECK(jsonb_typeof(evidence)='object'),
 actor text NOT NULL CHECK(octet_length(btrim(actor)) BETWEEN 1 AND 80),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,run_id) REFERENCES browser_runs(company_id,run_id)
);
CREATE INDEX browser_run_events_latest ON browser_run_events(company_id,run_id,event_seq DESC);
CREATE TRIGGER browser_runs_immutable
 BEFORE UPDATE OR DELETE ON browser_runs
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER browser_runs_no_truncate
 BEFORE TRUNCATE ON browser_runs FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER browser_run_events_immutable
 BEFORE UPDATE OR DELETE ON browser_run_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER browser_run_events_no_truncate
 BEFORE TRUNCATE ON browser_run_events FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM browser_run_events) OR EXISTS(SELECT 1 FROM browser_runs) THEN
  RAISE EXCEPTION 'cannot discard browser run history';
 END IF;
END
$$;
DROP TRIGGER browser_run_events_no_truncate ON browser_run_events;
DROP TRIGGER browser_run_events_immutable ON browser_run_events;
DROP INDEX browser_run_events_latest;
DROP TABLE browser_run_events;
DROP TRIGGER browser_runs_no_truncate ON browser_runs;
DROP TRIGGER browser_runs_immutable ON browser_runs;
DROP INDEX browser_runs_task_recent;
DROP TABLE browser_runs;
