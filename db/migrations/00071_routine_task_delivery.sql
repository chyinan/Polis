-- +goose Up
ALTER TABLE routines
 ADD COLUMN task_instruction text;

ALTER TABLE routines
 ADD CONSTRAINT routines_task_instruction_bounds
 CHECK(task_instruction IS NULL OR (octet_length(task_instruction) BETWEEN 1 AND 4096 AND task_instruction = btrim(task_instruction)));

ALTER TABLE routine_occurrences
 ADD COLUMN task_instruction_snapshot text,
 ADD COLUMN task_id text;

ALTER TABLE routine_occurrences
 DROP CONSTRAINT routine_occurrences_state_check,
 ADD CONSTRAINT routine_occurrences_state_check
  CHECK(state IN ('pending','delivered','needs_instruction','completed','cancelled')),
 ADD CONSTRAINT routine_occurrences_instruction_snapshot_bounds
  CHECK(task_instruction_snapshot IS NULL OR octet_length(task_instruction_snapshot) BETWEEN 1 AND 4096),
 ADD CONSTRAINT routine_occurrences_task_fk
  FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id);

CREATE UNIQUE INDEX routine_occurrences_task
 ON routine_occurrences(company_id,task_id) WHERE task_id IS NOT NULL;

-- Completing the linked Task is the only automatic terminal signal for a
-- delivered occurrence; candidate delivery still requires the existing review.
-- +goose StatementBegin
CREATE FUNCTION complete_routine_occurrence_with_task() RETURNS trigger AS $$
BEGIN
 IF NEW.state <> 'completed' OR OLD.state = NEW.state THEN
  RETURN NEW;
 END IF;
 UPDATE routine_occurrences
 SET state='completed'
 WHERE company_id=NEW.company_id AND task_id=NEW.id AND state='delivered';
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER routine_task_completion
 AFTER UPDATE OF state ON tasks
 FOR EACH ROW EXECUTE FUNCTION complete_routine_occurrence_with_task();

-- Existing recurrence records were created without a work instruction. Keep
-- them visible as blocked history instead of inventing Task content.
UPDATE routine_occurrences o
SET state='needs_instruction'
FROM routines r
WHERE o.company_id=r.company_id AND o.routine_id=r.id
 AND r.task_instruction IS NULL AND o.state='pending';

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION cancel_routine_occurrences_for_terminal_mission() RETURNS trigger AS $$
BEGIN
 IF NEW.state NOT IN ('cancelled','succeeded') OR OLD.state=NEW.state THEN
  RETURN NEW;
 END IF;
 UPDATE routine_occurrences o
 SET state='cancelled'
 FROM routines r
 WHERE o.company_id=r.company_id AND o.routine_id=r.id
  AND r.company_id=NEW.company_id AND r.mission_id=NEW.id
  AND (o.state IN ('pending','needs_instruction') OR (o.state='delivered' AND EXISTS(
   SELECT 1 FROM tasks t WHERE t.company_id=o.company_id AND t.id=o.task_id AND t.state='ready'
  )));
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

UPDATE routine_occurrences o
SET state='cancelled'
FROM routines r JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
WHERE o.company_id=r.company_id AND o.routine_id=r.id
 AND m.state IN ('cancelled','succeeded')
 AND (o.state IN ('pending','needs_instruction') OR (o.state='delivered' AND EXISTS(
  SELECT 1 FROM tasks t WHERE t.company_id=o.company_id AND t.id=o.task_id AND t.state='ready'
 )));

-- +goose Down
-- Routine Task plans and their occurrence links are durable history.
-- +goose StatementBegin
DO $$
BEGIN
 RAISE EXCEPTION 'routine Task delivery is forward-only';
END
$$;
-- +goose StatementEnd
