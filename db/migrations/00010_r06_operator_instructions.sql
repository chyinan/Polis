-- +goose Up
CREATE TABLE operator_instructions (
 company_id text NOT NULL,
 id text NOT NULL,
 mission_id text NOT NULL,
 task_id text,
 employee_id text,
 content text NOT NULL CHECK(octet_length(content) BETWEEN 1 AND 4096),
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','applied','rejected')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 applied_at timestamptz,
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id),
 CHECK(state!='applied' OR applied_at IS NOT NULL)
);
CREATE INDEX operator_instructions_scope ON operator_instructions(company_id,mission_id,created_at DESC);

-- +goose Down
DROP TABLE operator_instructions;
