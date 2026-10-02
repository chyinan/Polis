-- +goose Up
CREATE TABLE problem_tool_call_allocations (
 company_id text NOT NULL,
 problem_key text NOT NULL,
 revision bigint NOT NULL CHECK(revision>1),
 request_id text NOT NULL,
 additional_tool_calls bigint NOT NULL CHECK(additional_tool_calls>0),
 previous_limit bigint NOT NULL CHECK(previous_limit>0),
 resulting_limit bigint NOT NULL CHECK(resulting_limit>previous_limit),
 authorized_by text NOT NULL CHECK(authorized_by='local-owner'),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 500),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,problem_key,revision),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,problem_key) REFERENCES problem_tool_call_budgets(company_id,problem_key),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 CHECK(resulting_limit::numeric=previous_limit::numeric+additional_tool_calls::numeric)
);

CREATE VIEW problem_tool_call_budget_effective AS
SELECT b.company_id,b.problem_key,b.tool_call_limit AS initial_tool_call_limit,
 CASE
  WHEN b.tool_call_limit IS NULL THEN NULL
  WHEN b.tool_call_limit=0 THEN 0
  ELSE (b.tool_call_limit::numeric+COALESCE(sum(a.additional_tool_calls),0))::bigint
 END AS tool_call_limit,
 b.tool_calls_used,COALESCE(max(a.revision),1)::bigint AS revision
FROM problem_tool_call_budgets b
LEFT JOIN problem_tool_call_allocations a USING(company_id,problem_key)
GROUP BY b.company_id,b.problem_key,b.tool_call_limit,b.tool_calls_used;

-- Check the complete revision chain in the database as well as in Kernel.
-- The budget row lock serializes concurrent owner allocations for one key.
-- +goose StatementBegin
CREATE FUNCTION validate_problem_tool_call_allocation() RETURNS trigger AS $$
DECLARE base_limit bigint; used_calls bigint; extra_calls numeric; prior_revision bigint;
BEGIN
 SELECT tool_call_limit,tool_calls_used INTO base_limit,used_calls
 FROM problem_tool_call_budgets
 WHERE company_id=NEW.company_id AND problem_key=NEW.problem_key
 FOR UPDATE;
 IF NOT FOUND OR base_limit IS NULL OR base_limit=0 THEN
  RAISE EXCEPTION 'ProblemKey has no finite allocated budget' USING ERRCODE='23514';
 END IF;
 SELECT COALESCE(sum(additional_tool_calls),0),COALESCE(max(revision),1)
 INTO extra_calls,prior_revision
 FROM problem_tool_call_allocations
 WHERE company_id=NEW.company_id AND problem_key=NEW.problem_key;
 IF NEW.revision<>prior_revision+1 OR NEW.previous_limit::numeric<>base_limit::numeric+extra_calls OR
    NEW.resulting_limit::numeric<>NEW.previous_limit::numeric+NEW.additional_tool_calls::numeric THEN
  RAISE EXCEPTION 'ProblemKey allocation revision is stale or discontinuous' USING ERRCODE='40001';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER problem_tool_call_allocations_validate
 BEFORE INSERT ON problem_tool_call_allocations FOR EACH ROW EXECUTE FUNCTION validate_problem_tool_call_allocation();
CREATE TRIGGER problem_tool_call_allocations_immutable
 BEFORE UPDATE OR DELETE ON problem_tool_call_allocations FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER problem_tool_call_allocations_no_truncate
 BEFORE TRUNCATE ON problem_tool_call_allocations FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM problem_tool_call_allocations) THEN
  RAISE EXCEPTION 'cannot remove authorized ProblemKey budget allocation history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER problem_tool_call_allocations_validate ON problem_tool_call_allocations;
DROP TRIGGER problem_tool_call_allocations_immutable ON problem_tool_call_allocations;
DROP TRIGGER problem_tool_call_allocations_no_truncate ON problem_tool_call_allocations;
DROP FUNCTION validate_problem_tool_call_allocation();
DROP VIEW problem_tool_call_budget_effective;
DROP TABLE problem_tool_call_allocations;
