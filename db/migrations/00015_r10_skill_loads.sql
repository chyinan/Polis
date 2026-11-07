-- +goose Up
-- A skill load records that a task context received an approved Skill
-- revision body. Loading does not execute scripts or grant tools.
CREATE TABLE skill_loads (
 company_id text NOT NULL,
 id text NOT NULL,
 task_id text NOT NULL,
 employee_id text NOT NULL,
 skill_revision_id text NOT NULL,
 content_digest text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,id),
 UNIQUE(company_id,task_id,employee_id,skill_revision_id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,skill_revision_id) REFERENCES skill_revisions(company_id,id)
);
CREATE INDEX skill_loads_task ON skill_loads(company_id,task_id,created_at DESC);

-- +goose Down
DROP TABLE skill_loads;
