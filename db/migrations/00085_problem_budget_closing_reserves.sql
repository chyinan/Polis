-- +goose Up
CREATE TABLE problem_tool_call_closing_reserves (
 company_id text NOT NULL,
 problem_key text NOT NULL,
 revision bigint NOT NULL CHECK(revision>0),
 request_id text NOT NULL,
 expected_reserve_revision bigint NOT NULL CHECK(expected_reserve_revision>=0),
 expected_budget_revision bigint NOT NULL CHECK(expected_budget_revision>0),
 expected_tool_call_limit bigint NOT NULL CHECK(expected_tool_call_limit>=0),
 previous_reserved_tool_calls bigint NOT NULL CHECK(previous_reserved_tool_calls>=0),
 reserved_tool_calls bigint NOT NULL CHECK(reserved_tool_calls>=0),
 closeout_tool_calls_used_at_revision bigint NOT NULL CHECK(closeout_tool_calls_used_at_revision>=0),
 authorized_by text NOT NULL CHECK(authorized_by='local-owner'),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 500),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,problem_key,revision),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,problem_key) REFERENCES problem_tool_call_budgets(company_id,problem_key),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);

-- The fixed protected classes are Kernel-created review and peer-review Tasks.
-- Reserve revisions are snapshots bound to the current cap and closeout usage.
-- +goose StatementBegin
CREATE FUNCTION validate_problem_tool_call_closing_reserve() RETURNS trigger AS $$
DECLARE base_limit bigint; used_calls bigint; extra_calls numeric; budget_revision bigint;
        effective_limit numeric; reserve_revision bigint; old_reserved bigint;
        closeout_used bigint;
BEGIN
 SELECT tool_call_limit,tool_calls_used INTO base_limit,used_calls
 FROM problem_tool_call_budgets
 WHERE company_id=NEW.company_id AND problem_key=NEW.problem_key
 FOR UPDATE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'ProblemKey budget is missing' USING ERRCODE='23514';
 END IF;
 SELECT COALESCE(sum(additional_tool_calls),0),COALESCE(max(revision),1)
 INTO extra_calls,budget_revision
 FROM problem_tool_call_allocations
 WHERE company_id=NEW.company_id AND problem_key=NEW.problem_key;
 IF base_limit IS NULL THEN
  effective_limit:=0;
  IF used_calls<>0 OR extra_calls<>0 THEN
   RAISE EXCEPTION 'pending ProblemKey budget has prior use or allocation history' USING ERRCODE='23514';
  END IF;
 ELSIF base_limit=0 THEN
  RAISE EXCEPTION 'unbounded ProblemKey budget does not accept a closing reserve' USING ERRCODE='23514';
 ELSE
  effective_limit:=base_limit::numeric+extra_calls;
 END IF;
 SELECT revision,reserved_tool_calls INTO reserve_revision,old_reserved
 FROM problem_tool_call_closing_reserves
 WHERE company_id=NEW.company_id AND problem_key=NEW.problem_key
 ORDER BY revision DESC LIMIT 1;
 IF NOT FOUND THEN
  reserve_revision:=0;
  old_reserved:=0;
 END IF;
 SELECT COALESCE(sum(task_tool_calls_used),0)::bigint INTO closeout_used
 FROM tasks
 WHERE company_id=NEW.company_id AND problem_key=NEW.problem_key AND kind IN ('review','peer_review');
 IF NEW.revision<>reserve_revision+1 OR NEW.expected_reserve_revision<>reserve_revision OR
    NEW.previous_reserved_tool_calls<>old_reserved OR
    NEW.expected_budget_revision<>budget_revision OR
    NEW.expected_tool_call_limit::numeric<>effective_limit OR
    NEW.closeout_tool_calls_used_at_revision<>closeout_used OR
    (base_limit IS NOT NULL AND NEW.reserved_tool_calls::numeric>effective_limit-used_calls::numeric) THEN
  RAISE EXCEPTION 'ProblemKey closing reserve revision or available balance is stale' USING ERRCODE='40001';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER problem_tool_call_closing_reserves_validate
 BEFORE INSERT ON problem_tool_call_closing_reserves FOR EACH ROW EXECUTE FUNCTION validate_problem_tool_call_closing_reserve();
CREATE TRIGGER problem_tool_call_closing_reserves_immutable
 BEFORE UPDATE OR DELETE ON problem_tool_call_closing_reserves FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER problem_tool_call_closing_reserves_no_truncate
 BEFORE TRUNCATE ON problem_tool_call_closing_reserves FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- Pending reserves must fit the cap selected when the first WorkerSession
-- initializes the ProblemKey budget, even if a writer bypasses the Kernel.
-- +goose StatementBegin
CREATE FUNCTION validate_initial_problem_tool_call_closing_reserve() RETURNS trigger AS $$
DECLARE reserved bigint; used_at_revision bigint; closeout_used bigint; reserve_remaining bigint;
BEGIN
 IF OLD.tool_call_limit IS NOT NULL OR NEW.tool_call_limit IS NULL THEN
  RETURN NEW;
 END IF;
 SELECT COALESCE((SELECT r.reserved_tool_calls FROM problem_tool_call_closing_reserves r WHERE r.company_id=NEW.company_id AND r.problem_key=NEW.problem_key ORDER BY r.revision DESC LIMIT 1),0),
        COALESCE((SELECT r.closeout_tool_calls_used_at_revision FROM problem_tool_call_closing_reserves r WHERE r.company_id=NEW.company_id AND r.problem_key=NEW.problem_key ORDER BY r.revision DESC LIMIT 1),0)
 INTO reserved,used_at_revision;
 SELECT COALESCE(sum(task_tool_calls_used),0)::bigint INTO closeout_used
 FROM tasks WHERE company_id=NEW.company_id AND problem_key=NEW.problem_key AND kind IN ('review','peer_review');
 IF closeout_used<used_at_revision THEN
  RAISE EXCEPTION 'ProblemKey closeout usage moved backwards' USING ERRCODE='55000';
 END IF;
 reserve_remaining:=GREATEST(reserved-(closeout_used-used_at_revision),0);
 IF reserve_remaining>0 AND
    (NEW.tool_call_limit=0 OR reserve_remaining::numeric>NEW.tool_call_limit::numeric-NEW.tool_calls_used::numeric) THEN
  RAISE EXCEPTION 'initial ProblemKey cap cannot cover its protected closing reserve' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER problem_tool_call_closing_reserve_initial_cap
 BEFORE UPDATE OF tool_call_limit ON problem_tool_call_budgets
 FOR EACH ROW WHEN (OLD.tool_call_limit IS NULL AND NEW.tool_call_limit IS NOT NULL)
 EXECUTE FUNCTION validate_initial_problem_tool_call_closing_reserve();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM problem_tool_call_closing_reserves) THEN
  RAISE EXCEPTION 'cannot remove ProblemKey closing reserve policy history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER problem_tool_call_closing_reserves_validate ON problem_tool_call_closing_reserves;
DROP TRIGGER problem_tool_call_closing_reserves_immutable ON problem_tool_call_closing_reserves;
DROP TRIGGER problem_tool_call_closing_reserves_no_truncate ON problem_tool_call_closing_reserves;
DROP FUNCTION validate_problem_tool_call_closing_reserve();
DROP TRIGGER problem_tool_call_closing_reserve_initial_cap ON problem_tool_call_budgets;
DROP FUNCTION validate_initial_problem_tool_call_closing_reserve();
DROP TABLE problem_tool_call_closing_reserves;
