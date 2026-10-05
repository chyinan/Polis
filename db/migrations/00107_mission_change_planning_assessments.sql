-- +goose Up
CREATE TABLE mission_change_request_planning_assessments (
 company_id text NOT NULL,
 change_request_id text NOT NULL,
 assessment_id text NOT NULL CHECK(octet_length(assessment_id) BETWEEN 1 AND 80),
 revision bigint NOT NULL CHECK(revision > 0),
 analysis_basis_sha256 text NOT NULL CHECK(analysis_basis_sha256 ~ '^[a-f0-9]{64}$'),
 assessment_sha256 text NOT NULL CHECK(assessment_sha256 ~ '^[a-f0-9]{64}$'),
 assessment jsonb NOT NULL CHECK(jsonb_typeof(assessment)='object' AND octet_length(assessment::text) <= 32768),
 worker_session_id text NOT NULL,
 worker_task_id text NOT NULL,
 worker_epoch bigint NOT NULL CHECK(worker_epoch > 0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,change_request_id,revision),
 UNIQUE(company_id,assessment_id),
 FOREIGN KEY(company_id,change_request_id) REFERENCES mission_change_requests(company_id,change_request_id),
 FOREIGN KEY(company_id,worker_session_id) REFERENCES worker_sessions(company_id,id),
 FOREIGN KEY(company_id,worker_task_id) REFERENCES tasks(company_id,id)
);
CREATE INDEX mission_change_planning_assessments_latest
 ON mission_change_request_planning_assessments(company_id,change_request_id,revision DESC);

-- The database is the authority for the Planning identity and active session.
-- This trigger protects the receipt even if a future caller bypasses Kernel.
-- +goose StatementBegin
CREATE FUNCTION validate_mission_change_planning_assessment() RETURNS trigger AS $$
DECLARE
 request_mission text;
 request_state text;
 session_employee text;
 session_task text;
 session_epoch bigint;
 session_state text;
 execution_mode text;
 employee_epoch bigint;
 task_owner text;
 task_mission text;
 task_state text;
 mission_state text;
BEGIN
 SELECT r.mission_id,e.state INTO request_mission,request_state
 FROM mission_change_requests r
 JOIN LATERAL (
  SELECT state FROM mission_change_request_events
  WHERE company_id=r.company_id AND change_request_id=r.change_request_id
  ORDER BY event_seq DESC LIMIT 1
 ) e ON true
 WHERE r.company_id=NEW.company_id AND r.change_request_id=NEW.change_request_id
 FOR UPDATE OF r;
 IF NOT FOUND OR request_state NOT IN ('received','queued') THEN
  RAISE EXCEPTION 'Planning assessment requires an open formal change request' USING ERRCODE='23514';
 END IF;

 SELECT s.employee_id,s.task_id,s.epoch,s.state,s.execution_mode,e.epoch,t.owner,t.mission_id,t.state,m.state
 INTO session_employee,session_task,session_epoch,session_state,execution_mode,employee_epoch,task_owner,task_mission,task_state,mission_state
 FROM worker_sessions s
 JOIN employees e ON e.company_id=s.company_id AND e.id=s.employee_id
 JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
 JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
 WHERE s.company_id=NEW.company_id AND s.id=NEW.worker_session_id
 FOR UPDATE OF s,e,t,m;
 IF NOT FOUND OR session_employee<>'emp-planning' OR task_owner<>'emp-planning'
    OR session_task<>NEW.worker_task_id OR task_mission<>request_mission OR session_epoch<>NEW.worker_epoch
    OR employee_epoch<>NEW.worker_epoch OR session_state<>'active' OR execution_mode<>'provider'
    OR task_state<>'working' OR mission_state<>'active' THEN
  RAISE EXCEPTION 'Planning assessment must be bound to the current active Planning WorkerSession in the request Mission' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER mission_change_planning_assessments_validate
 BEFORE INSERT ON mission_change_request_planning_assessments FOR EACH ROW
 EXECUTE FUNCTION validate_mission_change_planning_assessment();
CREATE TRIGGER mission_change_planning_assessments_immutable
 BEFORE UPDATE OR DELETE ON mission_change_request_planning_assessments FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER mission_change_planning_assessments_no_truncate
 BEFORE TRUNCATE ON mission_change_request_planning_assessments FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM mission_change_request_planning_assessments) THEN
  RAISE EXCEPTION 'cannot remove persisted Planning impact assessments';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER mission_change_planning_assessments_no_truncate ON mission_change_request_planning_assessments;
DROP TRIGGER mission_change_planning_assessments_immutable ON mission_change_request_planning_assessments;
DROP TRIGGER mission_change_planning_assessments_validate ON mission_change_request_planning_assessments;
DROP FUNCTION validate_mission_change_planning_assessment();
DROP INDEX mission_change_planning_assessments_latest;
DROP TABLE mission_change_request_planning_assessments;
