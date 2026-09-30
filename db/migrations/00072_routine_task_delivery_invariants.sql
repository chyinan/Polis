-- +goose Up
ALTER TABLE routine_occurrences
 ADD CONSTRAINT routine_occurrences_task_snapshot_consistent
  CHECK((task_id IS NULL) = (task_instruction_snapshot IS NULL)),
 ADD CONSTRAINT routine_occurrences_delivered_task
  CHECK(state NOT IN ('delivered','completed') OR task_id IS NOT NULL),
 ADD CONSTRAINT routine_occurrences_unassigned_state
  CHECK(state NOT IN ('pending','needs_instruction') OR task_id IS NULL);

-- +goose Down
-- These invariants protect durable Routine-to-Task history.
-- +goose StatementBegin
DO $$
BEGIN
 RAISE EXCEPTION 'Routine Task delivery invariants are forward-only';
END
$$;
-- +goose StatementEnd
