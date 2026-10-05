-- +goose Up
-- A considered request remains open until it is applied or declined. If its
-- impact basis changes before application, Planning can refresh the receipt
-- after the Mission is resumed and its active session is re-established.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION validate_mission_change_planning_assessment() RETURNS trigger AS $$
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
 IF NOT FOUND OR request_state NOT IN ('received','queued','considered') THEN
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

-- +goose Down
-- Restore the previous guard. Existing assessment rows remain immutable.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION validate_mission_change_planning_assessment() RETURNS trigger AS $$
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
