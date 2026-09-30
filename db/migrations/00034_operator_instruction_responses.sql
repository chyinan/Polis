-- +goose Up
ALTER TABLE operator_instructions
 DROP CONSTRAINT operator_instructions_state_check,
 ADD CONSTRAINT operator_instructions_state_check CHECK (state IN ('pending','applied','rejected','needs_clarification'));

CREATE TABLE operator_instruction_responses (
 company_id text NOT NULL,
 response_id text NOT NULL,
 instruction_id text NOT NULL,
 employee_id text NOT NULL,
 session_id text NOT NULL,
 task_id text NOT NULL,
 outcome text NOT NULL CHECK (outcome IN ('applied','rejected','needs_clarification')),
 summary text NOT NULL CHECK (char_length(btrim(summary)) BETWEEN 1 AND 128 AND octet_length(summary) BETWEEN 1 AND 512 AND summary !~ '^[[:space:]]*$'),
 request_id text NOT NULL,
 responded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,response_id),
 UNIQUE(company_id,instruction_id,employee_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,instruction_id) REFERENCES operator_instructions(company_id,id),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,session_id) REFERENCES worker_sessions(company_id,id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id)
);
CREATE INDEX operator_instruction_responses_task ON operator_instruction_responses(company_id,task_id,responded_at DESC);
CREATE TRIGGER operator_instruction_responses_immutable
 BEFORE UPDATE OR DELETE ON operator_instruction_responses
 FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM operator_instruction_responses)
    OR EXISTS (SELECT 1 FROM operator_instructions WHERE state='needs_clarification') THEN
  RAISE EXCEPTION 'cannot roll back operator response history without losing response data';
 END IF;
END
$$;
-- +goose StatementEnd
DROP TABLE operator_instruction_responses;
ALTER TABLE operator_instructions
 DROP CONSTRAINT operator_instructions_state_check,
 ADD CONSTRAINT operator_instructions_state_check CHECK (state IN ('pending','applied','rejected'));
