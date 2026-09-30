-- +goose Up
-- Route transient wake hints to the exact durable schedule scope.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION signal_employee_schedule_for_ready_task() RETURNS trigger AS $$
DECLARE
 mission_state text;
BEGIN
 IF NEW.state <> 'ready' THEN
  RETURN NEW;
 END IF;
 IF TG_OP = 'UPDATE' AND OLD.state = 'ready' THEN
  RETURN NEW;
 END IF;
 SELECT state FROM missions WHERE company_id=NEW.company_id AND id=NEW.mission_id INTO mission_state;
 IF mission_state NOT IN ('active','paused') THEN
  RETURN NEW;
 END IF;

 INSERT INTO employee_schedules(company_id,employee_id,state)
 VALUES(NEW.company_id,NEW.owner,'sleeping') ON CONFLICT(company_id,employee_id) DO NOTHING;
 UPDATE employee_schedules
 SET work_generation=work_generation+1,
     state=CASE WHEN mission_state='paused' THEN 'paused'
       WHEN state IN ('paused','waiting_quota','working','admitted') THEN state
       ELSE 'wake_pending' END,
     pause_reason=CASE WHEN mission_state='paused' THEN 'mission_paused' ELSE pause_reason END,
     updated_at=now()
 WHERE company_id=NEW.company_id AND employee_id=NEW.owner;
 PERFORM pg_notify('polis_employee_wake',NEW.company_id || ':' || NEW.owner);
 RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION signal_employee_schedule_for_routine_occurrence() RETURNS trigger AS $$
DECLARE
 mission_state text;
 employee text;
BEGIN
 SELECT r.employee_id,m.state INTO employee,mission_state
 FROM routines r JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
 WHERE r.company_id=NEW.company_id AND r.id=NEW.routine_id;
 IF mission_state NOT IN ('active','paused') THEN
  RETURN NEW;
 END IF;

 INSERT INTO employee_schedules(company_id,employee_id,state)
 VALUES(NEW.company_id,employee,'sleeping') ON CONFLICT(company_id,employee_id) DO NOTHING;
 UPDATE employee_schedules
 SET work_generation=work_generation+1,
     state=CASE WHEN mission_state='paused' THEN 'paused'
       WHEN state IN ('paused','waiting_quota','working','admitted') THEN state
       ELSE 'wake_pending' END,
     pause_reason=CASE WHEN mission_state='paused' THEN 'mission_paused' ELSE pause_reason END,
     updated_at=now()
 WHERE company_id=NEW.company_id AND employee_id=employee;
 PERFORM pg_notify('polis_employee_wake',NEW.company_id || ':' || employee);
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION signal_employee_schedule_for_ready_task() RETURNS trigger AS $$
DECLARE
 mission_state text;
BEGIN
 IF NEW.state <> 'ready' THEN
  RETURN NEW;
 END IF;
 IF TG_OP='UPDATE' AND OLD.state='ready' THEN RETURN NEW; END IF;
 SELECT state FROM missions WHERE company_id=NEW.company_id AND id=NEW.mission_id INTO mission_state;
 IF mission_state NOT IN ('active','paused') THEN RETURN NEW; END IF;
 INSERT INTO employee_schedules(company_id,employee_id,state)
 VALUES(NEW.company_id,NEW.owner,'sleeping') ON CONFLICT(company_id,employee_id) DO NOTHING;
 UPDATE employee_schedules
 SET work_generation=work_generation+1,
     state=CASE WHEN mission_state='paused' THEN 'paused'
       WHEN state IN ('paused','waiting_quota','working','admitted') THEN state ELSE 'wake_pending' END,
     pause_reason=CASE WHEN mission_state='paused' THEN 'mission_paused' ELSE pause_reason END,
     updated_at=now()
 WHERE company_id=NEW.company_id AND employee_id=NEW.owner;
 PERFORM pg_notify('polis_employee_wake',NEW.owner);
 RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION signal_employee_schedule_for_routine_occurrence() RETURNS trigger AS $$
DECLARE
 mission_state text;
 employee text;
BEGIN
 SELECT r.employee_id,m.state INTO employee,mission_state
 FROM routines r JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
 WHERE r.company_id=NEW.company_id AND r.id=NEW.routine_id;
 IF mission_state NOT IN ('active','paused') THEN RETURN NEW; END IF;
 INSERT INTO employee_schedules(company_id,employee_id,state)
 VALUES(NEW.company_id,employee,'sleeping') ON CONFLICT(company_id,employee_id) DO NOTHING;
 UPDATE employee_schedules
 SET work_generation=work_generation+1,
     state=CASE WHEN mission_state='paused' THEN 'paused'
       WHEN state IN ('paused','waiting_quota','working','admitted') THEN state ELSE 'wake_pending' END,
     pause_reason=CASE WHEN mission_state='paused' THEN 'mission_paused' ELSE pause_reason END,
     updated_at=now()
 WHERE company_id=NEW.company_id AND employee_id=employee;
 PERFORM pg_notify('polis_employee_wake',employee);
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
