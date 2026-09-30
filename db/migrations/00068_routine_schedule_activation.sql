-- +goose Up
ALTER TABLE routines ADD COLUMN scheduling_active boolean NOT NULL DEFAULT false;

UPDATE routines r
SET scheduling_active=(m.state='active' AND c.state='active')
FROM missions m JOIN companies c ON c.id=m.company_id
WHERE m.company_id=r.company_id AND m.id=r.mission_id;

DROP INDEX routines_due;
DROP INDEX routines_due_backfill;
DROP INDEX routines_employee_due;
CREATE INDEX routines_due ON routines(next_due_at,company_id,id)
 WHERE scheduling_active AND next_due_at IS NOT NULL;
CREATE INDEX routines_due_backfill ON routines(company_id,id)
 WHERE scheduling_active AND next_due_at IS NULL;
CREATE INDEX routines_employee_due ON routines(company_id,employee_id,next_due_at,id)
 WHERE scheduling_active AND next_due_at IS NOT NULL;

-- Keep the cached schedule eligibility in the same transaction as Mission or
-- Company activation changes. The occurrence and cursor remain authoritative.
-- +goose StatementBegin
CREATE FUNCTION set_routine_schedule_activation() RETURNS trigger AS $$
BEGIN
 SELECT (m.state='active' AND c.state='active') INTO NEW.scheduling_active
 FROM missions m JOIN companies c ON c.id=m.company_id
 WHERE m.company_id=NEW.company_id AND m.id=NEW.mission_id;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE FUNCTION sync_routine_schedule_activation_from_mission() RETURNS trigger AS $$
DECLARE
 company_active boolean;
BEGIN
 IF NEW.state=OLD.state THEN RETURN NEW; END IF;
 SELECT state='active' INTO company_active FROM companies WHERE id=NEW.company_id;
 UPDATE routines
 SET scheduling_active=(NEW.state='active' AND company_active),updated_at=now()
 WHERE company_id=NEW.company_id AND mission_id=NEW.id;
 UPDATE employee_schedules s
 SET next_due_at=(
  SELECT min(r.next_due_at) FROM routines r
  JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
  WHERE r.company_id=s.company_id AND r.employee_id=s.employee_id
   AND r.scheduling_active AND r.next_due_at IS NOT NULL AND m.state='active'
 ),updated_at=now()
 WHERE s.company_id=NEW.company_id
  AND EXISTS(SELECT 1 FROM routines r WHERE r.company_id=s.company_id AND r.employee_id=s.employee_id AND r.mission_id=NEW.id);
 RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE FUNCTION sync_routine_schedule_activation_from_company() RETURNS trigger AS $$
BEGIN
 IF NEW.state=OLD.state THEN RETURN NEW; END IF;
 UPDATE routines r
 SET scheduling_active=(NEW.state='active' AND m.state='active'),updated_at=now()
 FROM missions m
 WHERE r.company_id=NEW.id AND m.company_id=r.company_id AND m.id=r.mission_id;
 UPDATE employee_schedules s
 SET next_due_at=(
  SELECT min(r.next_due_at) FROM routines r
  JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
  WHERE r.company_id=s.company_id AND r.employee_id=s.employee_id
   AND r.scheduling_active AND r.next_due_at IS NOT NULL AND m.state='active'
 ),updated_at=now()
 WHERE s.company_id=NEW.id;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER routines_schedule_activation_before_write
 BEFORE INSERT OR UPDATE OF company_id,mission_id ON routines
 FOR EACH ROW EXECUTE FUNCTION set_routine_schedule_activation();
CREATE TRIGGER missions_sync_routine_schedule_activation
 AFTER UPDATE OF state ON missions
 FOR EACH ROW EXECUTE FUNCTION sync_routine_schedule_activation_from_mission();
CREATE TRIGGER companies_sync_routine_schedule_activation
 AFTER UPDATE OF state ON companies
 FOR EACH ROW EXECUTE FUNCTION sync_routine_schedule_activation_from_company();

UPDATE employee_schedules s
SET next_due_at=(
 SELECT min(r.next_due_at) FROM routines r
 JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
 JOIN companies c ON c.id=r.company_id
 WHERE r.company_id=s.company_id AND r.employee_id=s.employee_id
  AND c.state='active' AND m.state='active' AND r.scheduling_active AND r.next_due_at IS NOT NULL
),updated_at=now()
WHERE EXISTS(SELECT 1 FROM routines r WHERE r.company_id=s.company_id AND r.employee_id=s.employee_id);

-- +goose Down
DROP TRIGGER companies_sync_routine_schedule_activation ON companies;
DROP TRIGGER missions_sync_routine_schedule_activation ON missions;
DROP TRIGGER routines_schedule_activation_before_write ON routines;
DROP FUNCTION sync_routine_schedule_activation_from_company();
DROP FUNCTION sync_routine_schedule_activation_from_mission();
DROP FUNCTION set_routine_schedule_activation();
DROP INDEX routines_due;
DROP INDEX routines_due_backfill;
DROP INDEX routines_employee_due;
ALTER TABLE routines DROP COLUMN scheduling_active;
CREATE INDEX routines_due ON routines(next_due_at,company_id,id)
 WHERE next_due_at IS NOT NULL;
CREATE INDEX routines_due_backfill ON routines(company_id,id)
 WHERE next_due_at IS NULL;
CREATE INDEX routines_employee_due ON routines(company_id,employee_id,next_due_at,id)
 WHERE next_due_at IS NOT NULL;
