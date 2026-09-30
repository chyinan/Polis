-- +goose Up
DROP INDEX employee_schedules_due;
CREATE INDEX employee_schedules_due ON employee_schedules(next_due_at,company_id,employee_id)
 WHERE next_due_at IS NOT NULL AND state<>'paused';

-- +goose Down
DROP INDEX employee_schedules_due;
CREATE INDEX employee_schedules_due ON employee_schedules(company_id,next_due_at,employee_id)
 WHERE state IN ('sleeping','waiting_quota');
