-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (
  SELECT 1
  FROM operator_instructions i
  JOIN tasks t ON t.company_id=i.company_id AND t.id=i.task_id
  WHERE i.employee_id IS NOT NULL AND i.task_id IS NOT NULL AND i.employee_id<>t.owner
 ) THEN
  RAISE EXCEPTION 'cannot migrate mismatched employee/task operator instructions';
 END IF;
 IF EXISTS (
  SELECT 1
  FROM operator_instructions i
  WHERE i.employee_id IS NOT NULL AND i.task_id IS NULL AND i.mission_id IS NOT NULL
    AND NOT EXISTS (
     SELECT 1 FROM tasks t
     WHERE t.company_id=i.company_id AND t.mission_id=i.mission_id AND t.owner=i.employee_id
    )
 ) THEN
  RAISE EXCEPTION 'cannot migrate mission guidance for an employee without a task in that Mission';
 END IF;
 IF EXISTS (
  SELECT 1
  FROM operator_instructions i
  WHERE i.state='pending'
    AND NOT (
     (i.employee_id IS NOT NULL AND EXISTS (
      SELECT 1 FROM employees e WHERE e.company_id=i.company_id AND e.id=i.employee_id AND e.enabled
     ))
     OR (i.employee_id IS NULL AND i.task_id IS NOT NULL AND EXISTS (
      SELECT 1 FROM tasks t JOIN employees e ON e.company_id=t.company_id AND e.id=t.owner AND e.enabled
      WHERE t.company_id=i.company_id AND t.id=i.task_id
     ))
     OR (i.employee_id IS NULL AND i.task_id IS NULL AND i.mission_id IS NOT NULL AND EXISTS (
      SELECT 1 FROM tasks t JOIN employees e ON e.company_id=t.company_id AND e.id=t.owner AND e.enabled
      WHERE t.company_id=i.company_id AND t.mission_id=i.mission_id
     ))
     OR (i.employee_id IS NULL AND i.task_id IS NULL AND i.mission_id IS NULL AND EXISTS (
      SELECT 1 FROM employees e WHERE e.company_id=i.company_id AND e.enabled
     ))
    )
 ) THEN
  RAISE EXCEPTION 'cannot migrate pending operator guidance without an eligible recipient';
 END IF;
END
$$;
-- +goose StatementEnd

ALTER TABLE operator_instructions ALTER COLUMN mission_id DROP NOT NULL;
ALTER TABLE operator_instructions ADD CONSTRAINT operator_instruction_scope_check CHECK (task_id IS NULL OR mission_id IS NOT NULL);
CREATE INDEX operator_instructions_company_wide_pending ON operator_instructions(company_id,created_at DESC,id DESC) WHERE mission_id IS NULL AND state='pending';

CREATE TABLE operator_instruction_recipients (
 company_id text NOT NULL,
 instruction_id text NOT NULL,
 employee_id text NOT NULL,
 PRIMARY KEY(company_id,instruction_id,employee_id),
 FOREIGN KEY(company_id,instruction_id) REFERENCES operator_instructions(company_id,id),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id)
);
CREATE INDEX operator_instruction_recipients_employee ON operator_instruction_recipients(company_id,employee_id,instruction_id);
CREATE TRIGGER operator_instruction_recipients_immutable
 BEFORE UPDATE OR DELETE ON operator_instruction_recipients
 FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();

INSERT INTO operator_instruction_recipients(company_id,instruction_id,employee_id)
SELECT i.company_id,i.id,i.employee_id FROM operator_instructions i
JOIN employees e ON e.company_id=i.company_id AND e.id=i.employee_id AND e.enabled
WHERE i.employee_id IS NOT NULL
UNION
SELECT i.company_id,i.id,t.owner FROM operator_instructions i JOIN tasks t ON t.company_id=i.company_id AND t.id=i.task_id
JOIN employees e ON e.company_id=t.company_id AND e.id=t.owner AND e.enabled
WHERE i.employee_id IS NULL AND i.task_id IS NOT NULL
UNION
SELECT i.company_id,i.id,t.owner FROM operator_instructions i JOIN tasks t ON t.company_id=i.company_id AND t.mission_id=i.mission_id
JOIN employees e ON e.company_id=t.company_id AND e.id=t.owner AND e.enabled
WHERE i.employee_id IS NULL AND i.task_id IS NULL AND i.mission_id IS NOT NULL
UNION
SELECT i.company_id,i.id,e.id FROM operator_instructions i JOIN employees e ON e.company_id=i.company_id AND e.enabled
WHERE i.employee_id IS NULL AND i.task_id IS NULL AND i.mission_id IS NULL;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM operator_instruction_recipients)
    OR EXISTS (SELECT 1 FROM operator_instructions WHERE mission_id IS NULL) THEN
  RAISE EXCEPTION 'cannot roll back guidance recipient assignments without losing scope data';
 END IF;
END
$$;
-- +goose StatementEnd
DROP INDEX operator_instructions_company_wide_pending;
DROP TABLE operator_instruction_recipients;
ALTER TABLE operator_instructions DROP CONSTRAINT operator_instruction_scope_check;
ALTER TABLE operator_instructions ALTER COLUMN mission_id SET NOT NULL;
