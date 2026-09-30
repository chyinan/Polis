-- +goose Up
CREATE INDEX routines_due_backfill ON routines(company_id,id)
 WHERE next_due_at IS NULL;

-- +goose Down
DROP INDEX routines_due_backfill;
