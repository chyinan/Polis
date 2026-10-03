-- +goose Up
CREATE TABLE task_tool_call_allocations (
 company_id text NOT NULL,
 task_id text NOT NULL,
 revision bigint NOT NULL CHECK(revision>1),
 request_id text NOT NULL,
 additional_tool_calls bigint NOT NULL CHECK(additional_tool_calls>0),
 previous_limit bigint NOT NULL CHECK(previous_limit>0),
 resulting_limit bigint NOT NULL CHECK(resulting_limit>previous_limit),
 problem_budget_revision bigint NOT NULL CHECK(problem_budget_revision>0),
 closing_reserve_revision bigint NOT NULL CHECK(closing_reserve_revision>=0),
 authorized_by text NOT NULL CHECK(authorized_by='local-owner'),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 500),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,task_id,revision),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 CHECK(resulting_limit::numeric=previous_limit::numeric+additional_tool_calls::numeric)
);

-- An explicit Task allocation is the only way to revise its already-fixed
-- finite envelope. The row is written first and the exact resulting cap is
-- committed to tasks in the same Kernel transaction.
-- +goose StatementBegin
CREATE FUNCTION validate_task_tool_call_allocation() RETURNS trigger AS $$
DECLARE v_task_limit bigint; v_task_used bigint; v_problem_key text; v_task_kind text; v_task_state text;
        v_prior_revision bigint; v_problem_limit bigint; v_problem_used bigint; v_problem_revision bigint;
        v_reserved bigint; v_reserve_revision bigint; v_reserve_baseline bigint; v_closeout_used bigint;
        v_reserve_remaining bigint; v_problem_remaining numeric; v_available numeric;
BEGIN
 SELECT task_tool_call_limit,task_tool_calls_used,problem_key,kind,state
 INTO v_task_limit,v_task_used,v_problem_key,v_task_kind,v_task_state
 FROM tasks WHERE company_id=NEW.company_id AND id=NEW.task_id FOR UPDATE;
 IF NOT FOUND OR v_task_limit IS NULL OR v_task_limit<=0 OR v_task_used<v_task_limit OR v_task_state IN ('completed','cancelled') OR
    EXISTS(SELECT 1 FROM worker_sessions s WHERE s.company_id=NEW.company_id AND s.task_id=NEW.task_id AND s.state!='stopped') THEN
  RAISE EXCEPTION 'Task is not eligible for a budget allocation' USING ERRCODE='23514';
 END IF;
 SELECT COALESCE(max(a.revision),1) INTO v_prior_revision FROM task_tool_call_allocations a
 WHERE a.company_id=NEW.company_id AND a.task_id=NEW.task_id;
 IF NEW.revision<>v_prior_revision+1 OR NEW.previous_limit<>v_task_limit OR
    NEW.resulting_limit::numeric<>v_task_limit::numeric+NEW.additional_tool_calls::numeric THEN
  RAISE EXCEPTION 'Task allocation revision is stale or discontinuous' USING ERRCODE='40001';
 END IF;

 PERFORM 1 FROM problem_tool_call_budgets pb
 WHERE pb.company_id=NEW.company_id AND pb.problem_key=v_problem_key FOR UPDATE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'ProblemKey budget is missing' USING ERRCODE='23503';
 END IF;
 SELECT e.tool_call_limit,e.tool_calls_used,e.revision
 INTO v_problem_limit,v_problem_used,v_problem_revision
 FROM problem_tool_call_budget_effective e
 WHERE e.company_id=NEW.company_id AND e.problem_key=v_problem_key;
 IF NOT FOUND OR v_problem_limit IS NULL OR v_problem_revision<>NEW.problem_budget_revision THEN
  RAISE EXCEPTION 'Task allocation ProblemKey budget revision is stale or pending' USING ERRCODE='40001';
 END IF;

 SELECT r.reserved_tool_calls,r.revision,r.closeout_tool_calls_used_at_revision
 INTO v_reserved,v_reserve_revision,v_reserve_baseline
 FROM problem_tool_call_closing_reserves r
 WHERE r.company_id=NEW.company_id AND r.problem_key=v_problem_key ORDER BY r.revision DESC LIMIT 1;
 IF NOT FOUND THEN
  v_reserved:=0; v_reserve_revision:=0; v_reserve_baseline:=0;
 END IF;
 SELECT COALESCE(sum(t.task_tool_calls_used),0)::bigint INTO v_closeout_used FROM tasks t
 WHERE t.company_id=NEW.company_id AND t.problem_key=v_problem_key AND t.kind IN ('review','peer_review');
 IF v_closeout_used<v_reserve_baseline OR v_reserve_revision<>NEW.closing_reserve_revision THEN
  RAISE EXCEPTION 'Task allocation closing reserve revision is stale' USING ERRCODE='40001';
 END IF;
 v_reserve_remaining:=GREATEST(v_reserved-GREATEST(v_closeout_used-v_reserve_baseline,0),0);
 IF v_problem_limit=0 THEN
  v_available:=NULL;
 ELSE
  v_problem_remaining:=v_problem_limit::numeric-v_problem_used::numeric;
  v_available:=v_problem_remaining;
  IF v_task_kind NOT IN ('review','peer_review') THEN
   v_available:=GREATEST(v_problem_remaining-v_reserve_remaining::numeric,0);
  END IF;
  IF v_available<NEW.additional_tool_calls::numeric THEN
   RAISE EXCEPTION 'ProblemKey has insufficient available calls for Task recovery' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER task_tool_call_allocations_validate
 BEFORE INSERT ON task_tool_call_allocations FOR EACH ROW EXECUTE FUNCTION validate_task_tool_call_allocation();
CREATE TRIGGER task_tool_call_allocations_immutable
 BEFORE UPDATE OR DELETE ON task_tool_call_allocations FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER task_tool_call_allocations_no_truncate
 BEFORE TRUNCATE ON task_tool_call_allocations FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- Reject direct Task cap edits. First-session initialization still changes
-- NULL to its starting cap; every later increase must match the latest ledger row.
-- +goose StatementBegin
CREATE FUNCTION guard_task_tool_call_limit_revision() RETURNS trigger AS $$
DECLARE v_latest_previous bigint; v_latest_result bigint;
BEGIN
 IF OLD.task_tool_call_limit IS NULL OR NEW.task_tool_call_limit IS NOT DISTINCT FROM OLD.task_tool_call_limit THEN
  RETURN NEW;
 END IF;
 SELECT a.previous_limit,a.resulting_limit INTO v_latest_previous,v_latest_result
 FROM task_tool_call_allocations a WHERE a.company_id=NEW.company_id AND a.task_id=NEW.id
 ORDER BY a.revision DESC LIMIT 1;
 IF NOT FOUND OR v_latest_previous IS DISTINCT FROM OLD.task_tool_call_limit OR v_latest_result IS DISTINCT FROM NEW.task_tool_call_limit THEN
  RAISE EXCEPTION 'Task tool-call cap changes require an authorized allocation revision' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER task_tool_call_limit_revision_guard
 BEFORE UPDATE OF task_tool_call_limit ON tasks
 FOR EACH ROW EXECUTE FUNCTION guard_task_tool_call_limit_revision();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM task_tool_call_allocations) THEN
  RAISE EXCEPTION 'cannot remove authorized Task tool-call allocation history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER task_tool_call_limit_revision_guard ON tasks;
DROP FUNCTION guard_task_tool_call_limit_revision();
DROP TRIGGER task_tool_call_allocations_validate ON task_tool_call_allocations;
DROP TRIGGER task_tool_call_allocations_immutable ON task_tool_call_allocations;
DROP TRIGGER task_tool_call_allocations_no_truncate ON task_tool_call_allocations;
DROP FUNCTION validate_task_tool_call_allocation();
DROP TABLE task_tool_call_allocations;
