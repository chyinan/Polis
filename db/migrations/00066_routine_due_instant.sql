-- +goose Up
ALTER TABLE routines ADD COLUMN next_due_at timestamptz;
CREATE INDEX routines_due ON routines(next_due_at,company_id,id)
 WHERE next_due_at IS NOT NULL;
CREATE INDEX routines_employee_due ON routines(company_id,employee_id,next_due_at,id)
 WHERE next_due_at IS NOT NULL;

-- +goose Down
DROP INDEX routines_due;
DROP INDEX routines_employee_due;
ALTER TABLE routines DROP COLUMN next_due_at;
