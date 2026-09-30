-- +goose Up
CREATE TABLE routines (
 company_id text NOT NULL,
 id text NOT NULL CHECK(id ~ '^[a-zA-Z0-9_-]{1,80}$'),
 mission_id text NOT NULL,
 employee_id text NOT NULL,
 timezone text NOT NULL CHECK(octet_length(timezone) BETWEEN 1 AND 128 AND timezone <> 'Local'),
 local_time text NOT NULL CHECK(local_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
 next_logical_day date NOT NULL,
 catch_up_policy text NOT NULL CHECK(catch_up_policy IN ('skip','coalesce_latest','catch_up')),
 max_catch_up integer NOT NULL CHECK(max_catch_up BETWEEN 1 AND 10),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id)
);

CREATE TABLE routine_materializations (
 company_id text NOT NULL,
 routine_id text NOT NULL,
 request_id text NOT NULL CHECK(octet_length(request_id) BETWEEN 1 AND 80),
 planned_at timestamptz NOT NULL,
 next_logical_day date NOT NULL,
 occurrence_count integer NOT NULL CHECK(occurrence_count BETWEEN 0 AND 10),
 skipped_from date,
 skipped_through date,
 PRIMARY KEY(company_id,routine_id,request_id),
 CHECK((skipped_from IS NULL) = (skipped_through IS NULL)),
 CHECK(skipped_from IS NULL OR skipped_from <= skipped_through),
 FOREIGN KEY(company_id,routine_id) REFERENCES routines(company_id,id)
);

CREATE TABLE routine_occurrences (
 company_id text NOT NULL,
 routine_id text NOT NULL,
 occurrence_key text NOT NULL CHECK(octet_length(occurrence_key) BETWEEN 1 AND 128),
 logical_day date NOT NULL,
 scheduled_at timestamptz NOT NULL,
 coalesced_from date,
 coalesced_through date,
 materialization_request_id text NOT NULL,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','completed','cancelled')),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(company_id,routine_id,occurrence_key),
 UNIQUE(company_id,routine_id,logical_day),
 CHECK((coalesced_from IS NULL) = (coalesced_through IS NULL)),
 CHECK(coalesced_from IS NULL OR coalesced_from <= coalesced_through),
 FOREIGN KEY(company_id,routine_id) REFERENCES routines(company_id,id),
 FOREIGN KEY(company_id,routine_id,materialization_request_id)
  REFERENCES routine_materializations(company_id,routine_id,request_id)
);

CREATE INDEX routine_occurrences_pending ON routine_occurrences(company_id,routine_id,logical_day)
 WHERE state='pending';
CREATE INDEX routines_employee ON routines(company_id,employee_id,next_logical_day);

-- A newly materialized occurrence is durable work. Paused Missions retain it
-- without reopening execution; resume will find the same pending row.
-- +goose StatementBegin
CREATE FUNCTION signal_employee_schedule_for_routine_occurrence() RETURNS trigger AS $$
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
 VALUES(NEW.company_id,employee,'sleeping')
 ON CONFLICT(company_id,employee_id) DO NOTHING;
 UPDATE employee_schedules
 SET work_generation=work_generation+1,
     state=CASE
      WHEN mission_state='paused' THEN 'paused'
      WHEN state IN ('paused','waiting_quota','working','admitted') THEN state
      ELSE 'wake_pending' END,
     pause_reason=CASE WHEN mission_state='paused' THEN 'mission_paused' ELSE pause_reason END,
     updated_at=now()
 WHERE company_id=NEW.company_id AND employee_id=employee;
 PERFORM pg_notify('polis_employee_wake',employee);
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER routine_occurrence_employee_schedule_wake
 AFTER INSERT ON routine_occurrences
 FOR EACH ROW WHEN (NEW.state='pending')
 EXECUTE FUNCTION signal_employee_schedule_for_routine_occurrence();

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM routine_occurrences) OR EXISTS (SELECT 1 FROM routine_materializations) OR EXISTS (SELECT 1 FROM routines) THEN
  RAISE EXCEPTION 'cannot roll back persisted Routine work or occurrence history';
 END IF;
END
$$;
-- +goose StatementEnd
DROP TRIGGER routine_occurrence_employee_schedule_wake ON routine_occurrences;
DROP FUNCTION signal_employee_schedule_for_routine_occurrence();
DROP TABLE routine_occurrences,routine_materializations,routines;
