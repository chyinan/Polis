-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION signal_employee_schedule_for_ready_task() RETURNS trigger AS $$
DECLARE
 mission_state text;
BEGIN
 IF NEW.state <> 'ready' THEN
  RETURN NEW;
 END IF;
 IF TG_OP = 'UPDATE' THEN
  IF OLD.state = 'ready' THEN
   RETURN NEW;
  END IF;
 END IF;
 SELECT state FROM missions
  WHERE company_id=NEW.company_id AND id=NEW.mission_id
 INTO mission_state;
 IF mission_state NOT IN ('active','paused') THEN
  RETURN NEW;
 END IF;

 INSERT INTO employee_schedules(company_id,employee_id,state)
 VALUES(NEW.company_id,NEW.owner,'sleeping')
 ON CONFLICT(company_id,employee_id) DO NOTHING;
 UPDATE employee_schedules
 SET work_generation=work_generation+1,
     state=CASE
      WHEN mission_state='paused' THEN 'paused'
      WHEN state IN ('paused','waiting_quota','working','admitted') THEN state
      ELSE 'wake_pending' END,
     pause_reason=CASE WHEN mission_state='paused' THEN 'mission_paused' ELSE pause_reason END,
     updated_at=now()
 WHERE company_id=NEW.company_id AND employee_id=NEW.owner;
 PERFORM pg_notify('polis_employee_wake',NEW.owner);
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER tasks_ready_employee_schedule_wake
 AFTER INSERT OR UPDATE ON tasks
 FOR EACH ROW EXECUTE FUNCTION signal_employee_schedule_for_ready_task();

UPDATE employee_schedules s
SET work_generation=s.work_generation+1,
    state=CASE
     WHEN ready.mission_state='paused' THEN 'paused'
     WHEN s.state IN ('paused','waiting_quota','working','admitted') THEN s.state
     ELSE 'wake_pending' END,
    pause_reason=CASE WHEN ready.mission_state='paused' THEN 'mission_paused' ELSE s.pause_reason END,
    updated_at=now()
FROM (
 SELECT DISTINCT t.company_id,t.owner,m.state AS mission_state
 FROM tasks t JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
 WHERE t.state='ready' AND m.state IN ('active','paused')
) ready
WHERE s.company_id=ready.company_id AND s.employee_id=ready.owner;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM employee_schedules WHERE work_generation>0 OR state<>'sleeping' OR checked_generation>0) THEN
  RAISE EXCEPTION 'cannot roll back ready-task wake history';
 END IF;
END
$$;
-- +goose StatementEnd
DROP TRIGGER tasks_ready_employee_schedule_wake ON tasks;
DROP FUNCTION signal_employee_schedule_for_ready_task();
