-- +goose Up
CREATE TABLE problem_tool_call_budgets (
 company_id text NOT NULL,
 problem_key text NOT NULL,
 tool_call_limit bigint CHECK(tool_call_limit>=0),
 tool_calls_used bigint NOT NULL DEFAULT 0 CHECK(tool_calls_used>=0),
 PRIMARY KEY(company_id,problem_key),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 CONSTRAINT problem_tool_calls_within_limit CHECK(
  tool_call_limit IS NULL OR tool_call_limit=0 OR tool_calls_used<=tool_call_limit
 )
);

-- Preserve the sum of previously admitted Task envelopes. A historical
-- unbounded Task keeps its ProblemKey unbounded; Tasks with no session yet do
-- not contribute an allocation.
INSERT INTO problem_tool_call_budgets(company_id,problem_key,tool_call_limit,tool_calls_used)
SELECT company_id,problem_key,
 CASE
  WHEN bool_or(task_tool_call_limit=0) THEN 0
  WHEN count(task_tool_call_limit)>0 THEN sum(task_tool_call_limit)::bigint
  ELSE NULL
 END,
 COALESCE(sum(task_tool_calls_used),0)::bigint
FROM tasks
GROUP BY company_id,problem_key;

ALTER TABLE tasks ADD CONSTRAINT tasks_problem_budget_fk
 FOREIGN KEY(company_id,problem_key)
 REFERENCES problem_tool_call_budgets(company_id,problem_key);

-- A ProblemKey allowance may be initialized once. Thereafter its limit cannot
-- be raised or lowered, and usage can only move forward.
-- +goose StatementBegin
CREATE FUNCTION guard_problem_tool_call_budget() RETURNS trigger AS $$
BEGIN
 IF OLD.tool_call_limit IS NOT NULL AND NEW.tool_call_limit IS DISTINCT FROM OLD.tool_call_limit THEN
  RAISE EXCEPTION 'ProblemKey tool-call allocation is immutable' USING ERRCODE='55000';
 END IF;
 IF NEW.tool_calls_used < OLD.tool_calls_used THEN
  RAISE EXCEPTION 'ProblemKey tool-call usage cannot decrease' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION assign_task_problem_lineage() RETURNS trigger AS $$
DECLARE parent_problem_key text;
BEGIN
 IF NEW.parent_task_id IS NULL THEN
  NEW.problem_key := 'problem:'||NEW.id;
 ELSE
  SELECT problem_key INTO parent_problem_key
  FROM tasks
  WHERE company_id=NEW.company_id AND id=NEW.parent_task_id AND mission_id=NEW.mission_id;
  IF NOT FOUND THEN
   RAISE EXCEPTION 'Task parent is absent or belongs to a different Mission' USING ERRCODE='23503';
  END IF;
  NEW.problem_key := parent_problem_key;
 END IF;
 INSERT INTO problem_tool_call_budgets(company_id,problem_key,tool_call_limit,tool_calls_used)
 VALUES(NEW.company_id,NEW.problem_key,NULL,0)
 ON CONFLICT(company_id,problem_key) DO NOTHING;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER problem_tool_call_budget_immutable
 BEFORE UPDATE ON problem_tool_call_budgets FOR EACH ROW EXECUTE FUNCTION guard_problem_tool_call_budget();
CREATE TRIGGER problem_tool_call_budget_no_delete
 BEFORE DELETE ON problem_tool_call_budgets FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER problem_tool_call_budget_no_truncate
 BEFORE TRUNCATE ON problem_tool_call_budgets FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM tasks) OR EXISTS(SELECT 1 FROM problem_tool_call_budgets WHERE tool_calls_used<>0 OR tool_call_limit IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot remove persisted ProblemKey tool-call budgets';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER problem_tool_call_budget_immutable ON problem_tool_call_budgets;
DROP TRIGGER problem_tool_call_budget_no_delete ON problem_tool_call_budgets;
DROP TRIGGER problem_tool_call_budget_no_truncate ON problem_tool_call_budgets;
DROP FUNCTION guard_problem_tool_call_budget();
ALTER TABLE tasks DROP CONSTRAINT tasks_problem_budget_fk;
DROP TABLE problem_tool_call_budgets;

-- Restore the Schema 82 trigger implementation. The table will no longer exist
-- after this migration rolls back, so the insert into it must be removed.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assign_task_problem_lineage() RETURNS trigger AS $$
DECLARE parent_problem_key text;
BEGIN
 IF NEW.parent_task_id IS NULL THEN
  NEW.problem_key := 'problem:'||NEW.id;
  RETURN NEW;
 END IF;
 SELECT problem_key INTO parent_problem_key
 FROM tasks
 WHERE company_id=NEW.company_id AND id=NEW.parent_task_id AND mission_id=NEW.mission_id;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'Task parent is absent or belongs to a different Mission' USING ERRCODE='23503';
 END IF;
 NEW.problem_key := parent_problem_key;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
