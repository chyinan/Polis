-- +goose Up
CREATE INDEX problem_tool_call_budget_rejections_task_recent
 ON problem_tool_call_budget_rejections(company_id,task_id,occurred_at DESC,request_id DESC);

CREATE TABLE task_tool_call_budget_incomplete_closures (
 company_id text NOT NULL,
 task_id text NOT NULL,
 problem_key text NOT NULL,
 request_id text NOT NULL,
 source_rejection_request_id text NOT NULL,
 task_tool_call_limit bigint CHECK(task_tool_call_limit>=0),
 task_tool_calls_used bigint NOT NULL CHECK(task_tool_calls_used>=0),
 task_allocation_revision bigint NOT NULL CHECK(task_allocation_revision>=1),
 problem_tool_call_limit bigint CHECK(problem_tool_call_limit>=0),
 problem_tool_calls_used bigint NOT NULL CHECK(problem_tool_calls_used>=0),
 problem_budget_revision bigint NOT NULL CHECK(problem_budget_revision>=1),
 closing_reserve_tool_calls bigint NOT NULL CHECK(closing_reserve_tool_calls>=0),
 closing_reserve_remaining bigint NOT NULL CHECK(closing_reserve_remaining>=0 AND closing_reserve_remaining<=closing_reserve_tool_calls),
 closing_reserve_revision bigint NOT NULL CHECK(closing_reserve_revision>=0),
 authorized_by text NOT NULL CHECK(authorized_by='local-owner'),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 500),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,task_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,problem_key) REFERENCES problem_tool_call_budgets(company_id,problem_key),
 FOREIGN KEY(company_id,source_rejection_request_id) REFERENCES problem_tool_call_budget_rejections(company_id,request_id),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);

-- A closeout is an explicit owner decision tied to the latest denial and exact
-- Task, ProblemKey, and reserve snapshots. It is irreversible and requires
-- that no WorkerSession is still live for the Task.
-- +goose StatementBegin
CREATE FUNCTION validate_task_tool_call_budget_incomplete_closure() RETURNS trigger AS $$
DECLARE v_problem_key text; v_task_state text; v_task_limit bigint; v_task_used bigint;
        v_task_revision bigint; v_problem_limit bigint; v_problem_used bigint; v_problem_revision bigint;
        v_reserve_revision bigint; v_reserve_calls bigint; v_reserve_remaining bigint;
        v_reserve_baseline bigint; v_closeout_used bigint;
        v_latest_rejection text; v_latest_rejection_reason text;
BEGIN
 SELECT problem_key,state,task_tool_call_limit,task_tool_calls_used
 INTO v_problem_key,v_task_state,v_task_limit,v_task_used
 FROM tasks WHERE company_id=NEW.company_id AND id=NEW.task_id FOR UPDATE;
 IF NOT FOUND OR v_problem_key<>NEW.problem_key OR v_task_state IN ('completed','cancelled') OR
    v_task_limit IS DISTINCT FROM NEW.task_tool_call_limit OR v_task_used<>NEW.task_tool_calls_used OR
    EXISTS(SELECT 1 FROM worker_sessions s WHERE s.company_id=NEW.company_id AND s.task_id=NEW.task_id AND s.state!='stopped') THEN
  RAISE EXCEPTION 'Task is not eligible for incomplete budget closeout' USING ERRCODE='23514';
 END IF;
 SELECT COALESCE(max(a.revision),1)::bigint INTO v_task_revision FROM task_tool_call_allocations a
 WHERE a.company_id=NEW.company_id AND a.task_id=NEW.task_id;
 IF v_task_revision<>NEW.task_allocation_revision THEN
  RAISE EXCEPTION 'Task closeout allocation revision is stale' USING ERRCODE='40001';
 END IF;
 SELECT d.request_id,d.reason INTO v_latest_rejection,v_latest_rejection_reason FROM problem_tool_call_budget_rejections d
 WHERE d.company_id=NEW.company_id AND d.task_id=NEW.task_id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1;
 IF NOT FOUND OR v_latest_rejection<>NEW.source_rejection_request_id OR v_latest_rejection_reason='session_limit' THEN
  RAISE EXCEPTION 'Task closeout rejection snapshot is stale or absent' USING ERRCODE='40001';
 END IF;
 PERFORM 1 FROM problem_tool_call_budgets pb
 WHERE pb.company_id=NEW.company_id AND pb.problem_key=NEW.problem_key FOR UPDATE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'ProblemKey budget is missing' USING ERRCODE='23503';
 END IF;
 SELECT e.tool_call_limit,e.tool_calls_used,e.revision
 INTO v_problem_limit,v_problem_used,v_problem_revision
 FROM problem_tool_call_budget_effective e
 WHERE e.company_id=NEW.company_id AND e.problem_key=NEW.problem_key;
 IF NOT FOUND OR v_problem_limit IS DISTINCT FROM NEW.problem_tool_call_limit OR
    v_problem_used<>NEW.problem_tool_calls_used OR v_problem_revision<>NEW.problem_budget_revision THEN
  RAISE EXCEPTION 'Task closeout ProblemKey snapshot is stale' USING ERRCODE='40001';
 END IF;
 SELECT r.reserved_tool_calls,r.revision,r.closeout_tool_calls_used_at_revision
 INTO v_reserve_calls,v_reserve_revision,v_reserve_baseline
 FROM problem_tool_call_closing_reserves r
 WHERE r.company_id=NEW.company_id AND r.problem_key=NEW.problem_key ORDER BY r.revision DESC LIMIT 1;
 IF NOT FOUND THEN
  v_reserve_calls:=0; v_reserve_revision:=0; v_reserve_baseline:=0;
 END IF;
 SELECT COALESCE(sum(t.task_tool_calls_used),0)::bigint INTO v_closeout_used FROM tasks t
 WHERE t.company_id=NEW.company_id AND t.problem_key=NEW.problem_key AND t.kind IN ('review','peer_review');
 IF v_closeout_used<v_reserve_baseline THEN
  RAISE EXCEPTION 'ProblemKey closing usage moved backwards' USING ERRCODE='55000';
 END IF;
 v_reserve_remaining:=GREATEST(v_reserve_calls-(v_closeout_used-v_reserve_baseline),0);
 IF v_reserve_revision<>NEW.closing_reserve_revision OR v_reserve_calls<>NEW.closing_reserve_tool_calls OR
    v_reserve_remaining<>NEW.closing_reserve_remaining THEN
  RAISE EXCEPTION 'Task closeout reserve snapshot is stale' USING ERRCODE='40001';
 END IF;
 IF EXISTS(SELECT 1 FROM problem_tool_call_budget_rejections d
           WHERE d.company_id=NEW.company_id AND d.request_id=NEW.source_rejection_request_id AND
             (d.reason='session_limit' OR d.task_tool_call_limit IS DISTINCT FROM NEW.task_tool_call_limit OR
              d.task_tool_calls_used<>NEW.task_tool_calls_used OR d.problem_tool_call_limit IS DISTINCT FROM NEW.problem_tool_call_limit OR
              d.problem_tool_calls_used<>NEW.problem_tool_calls_used OR d.problem_budget_revision<>NEW.problem_budget_revision OR
              d.closing_reserve_tool_calls<>NEW.closing_reserve_tool_calls OR
              d.closing_reserve_remaining<>NEW.closing_reserve_remaining OR
              d.closing_reserve_revision<>NEW.closing_reserve_revision)) THEN
  RAISE EXCEPTION 'Task closeout rejection no longer matches the current budget state' USING ERRCODE='40001';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER task_tool_call_budget_incomplete_closures_validate
 BEFORE INSERT ON task_tool_call_budget_incomplete_closures FOR EACH ROW
 EXECUTE FUNCTION validate_task_tool_call_budget_incomplete_closure();
CREATE TRIGGER task_tool_call_budget_incomplete_closures_immutable
 BEFORE UPDATE OR DELETE ON task_tool_call_budget_incomplete_closures FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER task_tool_call_budget_incomplete_closures_no_truncate
 BEFORE TRUNCATE ON task_tool_call_budget_incomplete_closures FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- Once a budget-limited Task has been closed as incomplete, it cannot acquire
-- another allocation revision.
-- +goose StatementBegin
CREATE FUNCTION reject_closed_task_tool_call_budget_allocation() RETURNS trigger AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM task_tool_call_budget_incomplete_closures c WHERE c.company_id=NEW.company_id AND c.task_id=NEW.task_id) THEN
  RAISE EXCEPTION 'Task budget was closed as incomplete' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER task_tool_call_allocations_closed_guard
 BEFORE INSERT ON task_tool_call_allocations FOR EACH ROW
 EXECUTE FUNCTION reject_closed_task_tool_call_budget_allocation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM task_tool_call_budget_incomplete_closures) THEN
  RAISE EXCEPTION 'cannot remove Task incomplete budget closeout history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER task_tool_call_allocations_closed_guard ON task_tool_call_allocations;
DROP FUNCTION reject_closed_task_tool_call_budget_allocation();
DROP TRIGGER task_tool_call_budget_incomplete_closures_validate ON task_tool_call_budget_incomplete_closures;
DROP TRIGGER task_tool_call_budget_incomplete_closures_immutable ON task_tool_call_budget_incomplete_closures;
DROP TRIGGER task_tool_call_budget_incomplete_closures_no_truncate ON task_tool_call_budget_incomplete_closures;
DROP FUNCTION validate_task_tool_call_budget_incomplete_closure();
DROP TABLE task_tool_call_budget_incomplete_closures;
DROP INDEX problem_tool_call_budget_rejections_task_recent;
