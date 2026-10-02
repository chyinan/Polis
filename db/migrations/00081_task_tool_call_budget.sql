-- +goose Up
-- A Task's first WorkerSession fixes its cumulative tool-call budget. Existing
-- Task histories are backfilled to the sum of their prior bounded session
-- budgets; any prior unbounded session keeps the Task unbounded.
ALTER TABLE tasks
 ADD COLUMN task_tool_call_limit bigint CHECK(task_tool_call_limit>=0),
 ADD COLUMN task_tool_calls_used bigint NOT NULL DEFAULT 0 CHECK(task_tool_calls_used>=0),
 ADD CONSTRAINT task_tool_calls_within_limit CHECK(
  task_tool_call_limit IS NULL OR task_tool_call_limit=0 OR task_tool_calls_used<=task_tool_call_limit
 );

WITH session_budgets AS (
 SELECT company_id,task_id,
  CASE WHEN bool_or(tool_call_limit=0) THEN 0 ELSE sum(tool_call_limit)::bigint END AS task_limit,
  sum(tool_calls_used)::bigint AS task_used
 FROM worker_sessions
 GROUP BY company_id,task_id
)
UPDATE tasks t SET task_tool_call_limit=b.task_limit,task_tool_calls_used=b.task_used
FROM session_budgets b
WHERE b.company_id=t.company_id AND b.task_id=t.id;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM worker_sessions) THEN
  RAISE EXCEPTION 'cannot remove cumulative Task tool-call budgets after WorkerSession history exists';
 END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE tasks
 DROP CONSTRAINT task_tool_calls_within_limit,
 DROP COLUMN task_tool_calls_used,
 DROP COLUMN task_tool_call_limit;
