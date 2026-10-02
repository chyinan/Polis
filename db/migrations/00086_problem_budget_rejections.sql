-- +goose Up
CREATE TABLE problem_tool_call_budget_rejections (
 company_id text NOT NULL,
 request_id text NOT NULL,
 problem_key text NOT NULL,
 task_id text NOT NULL,
 worker_session_id text,
 route text NOT NULL CHECK(route IN ('worker_admission','worker_tool_call')),
 reason text NOT NULL CHECK(reason IN ('session_limit','task_limit','problem_limit','closing_reserve','initial_closing_reserve')),
 task_kind text NOT NULL,
 session_tool_call_limit bigint NOT NULL CHECK(session_tool_call_limit>=0),
 session_tool_calls_used bigint NOT NULL CHECK(session_tool_calls_used>=0),
 task_tool_call_limit bigint CHECK(task_tool_call_limit>=0),
 task_tool_calls_used bigint NOT NULL CHECK(task_tool_calls_used>=0),
 problem_tool_call_limit bigint CHECK(problem_tool_call_limit>=0),
 problem_tool_calls_used bigint NOT NULL CHECK(problem_tool_calls_used>=0),
 problem_budget_revision bigint NOT NULL CHECK(problem_budget_revision>0),
 closing_reserve_tool_calls bigint NOT NULL CHECK(closing_reserve_tool_calls>=0),
 closing_reserve_remaining bigint NOT NULL CHECK(closing_reserve_remaining>=0 AND closing_reserve_remaining<=closing_reserve_tool_calls),
 closing_reserve_revision bigint NOT NULL CHECK(closing_reserve_revision>=0),
 occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,request_id),
 FOREIGN KEY(company_id,problem_key) REFERENCES problem_tool_call_budgets(company_id,problem_key),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,worker_session_id) REFERENCES worker_sessions(company_id,id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 CHECK((route='worker_admission' AND worker_session_id IS NULL AND session_tool_calls_used=0)
    OR (route='worker_tool_call' AND worker_session_id IS NOT NULL))
);

-- The immutable row records the precise cap and policy revisions that caused
-- a Kernel route to be refused. The trigger re-reads each live counter while
-- the Kernel still holds the same budget rows locked in its decision tx.
-- +goose StatementBegin
CREATE FUNCTION validate_problem_tool_call_budget_rejection() RETURNS trigger AS $$
DECLARE current_problem_key text; current_task_kind text; current_task_limit bigint;
        current_task_used bigint; current_session_limit bigint; current_session_used bigint;
        current_problem_limit bigint; current_problem_used bigint; current_problem_revision bigint;
        current_reserve bigint; current_reserve_revision bigint; reserve_baseline bigint;
        closeout_used bigint; reserve_remaining bigint; task_remaining bigint; problem_remaining bigint;
        effective_task_limit bigint;
BEGIN
 SELECT problem_key,kind,task_tool_call_limit,task_tool_calls_used
 INTO current_problem_key,current_task_kind,current_task_limit,current_task_used
 FROM tasks WHERE company_id=NEW.company_id AND id=NEW.task_id FOR SHARE;
 IF NOT FOUND OR current_problem_key<>NEW.problem_key OR current_task_kind<>NEW.task_kind OR
    current_task_limit IS DISTINCT FROM NEW.task_tool_call_limit OR current_task_used<>NEW.task_tool_calls_used THEN
  RAISE EXCEPTION 'ProblemKey rejection Task snapshot is stale' USING ERRCODE='40001';
 END IF;

 IF NEW.route='worker_tool_call' THEN
  SELECT tool_call_limit,tool_calls_used INTO current_session_limit,current_session_used
  FROM worker_sessions WHERE company_id=NEW.company_id AND id=NEW.worker_session_id AND task_id=NEW.task_id FOR SHARE;
  IF NOT FOUND OR current_session_limit<>NEW.session_tool_call_limit OR current_session_used<>NEW.session_tool_calls_used THEN
   RAISE EXCEPTION 'ProblemKey rejection session snapshot is stale' USING ERRCODE='40001';
  END IF;
 ELSIF NEW.worker_session_id IS NOT NULL THEN
  RAISE EXCEPTION 'Worker admission rejection cannot have a session' USING ERRCODE='23514';
 END IF;

 PERFORM 1 FROM problem_tool_call_budgets
 WHERE company_id=NEW.company_id AND problem_key=NEW.problem_key FOR SHARE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'ProblemKey budget is missing' USING ERRCODE='23503';
 END IF;
 SELECT tool_call_limit,tool_calls_used,revision INTO current_problem_limit,current_problem_used,current_problem_revision
 FROM problem_tool_call_budget_effective
 WHERE company_id=NEW.company_id AND problem_key=NEW.problem_key;
 IF NOT FOUND OR current_problem_limit IS DISTINCT FROM NEW.problem_tool_call_limit OR
    current_problem_used<>NEW.problem_tool_calls_used OR current_problem_revision<>NEW.problem_budget_revision THEN
  RAISE EXCEPTION 'ProblemKey rejection budget snapshot is stale' USING ERRCODE='40001';
 END IF;

 SELECT reserved_tool_calls,revision,closeout_tool_calls_used_at_revision
 INTO current_reserve,current_reserve_revision,reserve_baseline
 FROM problem_tool_call_closing_reserves
 WHERE company_id=NEW.company_id AND problem_key=NEW.problem_key ORDER BY revision DESC LIMIT 1;
 IF NOT FOUND THEN
  current_reserve:=0; current_reserve_revision:=0; reserve_baseline:=0;
 END IF;
 SELECT COALESCE(sum(task_tool_calls_used),0)::bigint INTO closeout_used FROM tasks
 WHERE company_id=NEW.company_id AND problem_key=NEW.problem_key AND kind IN ('review','peer_review');
 IF closeout_used<reserve_baseline THEN
  RAISE EXCEPTION 'ProblemKey closing usage moved backwards' USING ERRCODE='55000';
 END IF;
 reserve_remaining:=GREATEST(current_reserve-(closeout_used-reserve_baseline),0);
 IF current_reserve<>NEW.closing_reserve_tool_calls OR reserve_remaining<>NEW.closing_reserve_remaining OR
    current_reserve_revision<>NEW.closing_reserve_revision THEN
  RAISE EXCEPTION 'ProblemKey rejection closing policy snapshot is stale' USING ERRCODE='40001';
 END IF;

 effective_task_limit:=COALESCE(current_task_limit,NEW.session_tool_call_limit);
 task_remaining:=CASE WHEN current_task_limit IS NULL OR current_task_limit=0 THEN NULL
                      ELSE current_task_limit-current_task_used END;
 problem_remaining:=CASE WHEN current_problem_limit IS NULL OR current_problem_limit=0 THEN NULL
                         ELSE current_problem_limit-current_problem_used END;
 IF (NEW.reason='session_limit' AND NOT COALESCE((NEW.route='worker_tool_call' AND current_session_limit>0 AND current_session_used>=current_session_limit),false)) OR
    (NEW.reason='task_limit' AND NOT COALESCE((current_task_limit>0 AND current_task_used>=current_task_limit),false)) OR
    (NEW.reason='problem_limit' AND NOT COALESCE((current_problem_limit>0 AND current_problem_used>=current_problem_limit),false)) OR
    (NEW.reason='closing_reserve' AND NOT COALESCE((current_problem_limit>0 AND problem_remaining>0 AND current_task_kind NOT IN ('review','peer_review') AND problem_remaining<=reserve_remaining),false)) OR
    (NEW.reason='initial_closing_reserve' AND NOT COALESCE((NEW.route='worker_admission' AND current_problem_limit IS NULL AND reserve_remaining>0 AND
      (effective_task_limit=0 OR reserve_remaining>effective_task_limit OR current_task_kind NOT IN ('review','peer_review') AND reserve_remaining>=effective_task_limit)),false)) THEN
  RAISE EXCEPTION 'ProblemKey rejection reason does not match the budget state' USING ERRCODE='23514';
 END IF;
 IF task_remaining IS NOT NULL AND current_task_used>current_task_limit OR
    problem_remaining IS NOT NULL AND current_problem_used>current_problem_limit THEN
  RAISE EXCEPTION 'ProblemKey rejection usage exceeds its snapshot limit' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER problem_tool_call_budget_rejections_validate
 BEFORE INSERT ON problem_tool_call_budget_rejections
 FOR EACH ROW EXECUTE FUNCTION validate_problem_tool_call_budget_rejection();
CREATE TRIGGER problem_tool_call_budget_rejections_immutable
 BEFORE UPDATE OR DELETE ON problem_tool_call_budget_rejections
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER problem_tool_call_budget_rejections_no_truncate
 BEFORE TRUNCATE ON problem_tool_call_budget_rejections
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE INDEX problem_tool_call_budget_rejections_recent
 ON problem_tool_call_budget_rejections(company_id,problem_key,occurred_at DESC);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM problem_tool_call_budget_rejections) THEN
  RAISE EXCEPTION 'cannot remove ProblemKey budget rejection history';
 END IF;
END $$;
-- +goose StatementEnd
DROP INDEX problem_tool_call_budget_rejections_recent;
DROP TRIGGER problem_tool_call_budget_rejections_validate ON problem_tool_call_budget_rejections;
DROP TRIGGER problem_tool_call_budget_rejections_immutable ON problem_tool_call_budget_rejections;
DROP TRIGGER problem_tool_call_budget_rejections_no_truncate ON problem_tool_call_budget_rejections;
DROP FUNCTION validate_problem_tool_call_budget_rejection();
DROP TABLE problem_tool_call_budget_rejections;
