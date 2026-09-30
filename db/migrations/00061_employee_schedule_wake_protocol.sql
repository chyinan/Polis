-- +goose Up
CREATE TABLE employee_schedules (
 company_id text NOT NULL,
 employee_id text NOT NULL,
 state text NOT NULL DEFAULT 'sleeping'
  CHECK(state IN ('quiescing','sleeping','wake_pending','admitted','working','paused','waiting_quota')),
 work_generation bigint NOT NULL DEFAULT 0 CHECK(work_generation>=0),
 checked_generation bigint NOT NULL DEFAULT 0 CHECK(checked_generation>=0 AND checked_generation<=work_generation),
 next_due_at timestamptz,
 pause_reason text,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(company_id,employee_id),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id)
);

INSERT INTO employee_schedules(company_id,employee_id,state,work_generation,checked_generation,pause_reason)
SELECT e.company_id,e.id,
 CASE
  WHEN facts.mission_paused THEN 'paused'
  WHEN facts.active_worker THEN 'working'
  WHEN facts.pending_peer_work THEN 'wake_pending'
  ELSE 'sleeping'
 END,
 CASE WHEN facts.pending_peer_work THEN 1 ELSE 0 END,
 0,
 CASE WHEN facts.mission_paused THEN 'mission_paused' ELSE NULL END
FROM employees e
CROSS JOIN LATERAL (
 SELECT
  EXISTS(
   SELECT 1 FROM tasks t JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
   WHERE t.company_id=e.company_id AND t.owner=e.id AND m.state='paused'
  ) AS mission_paused,
  EXISTS(
   SELECT 1 FROM worker_sessions s
   WHERE s.company_id=e.company_id AND s.employee_id=e.id AND s.state<>'stopped'
  ) AS active_worker,
  EXISTS(
   SELECT 1 FROM peer_work_signals s JOIN obligations o
    ON o.company_id=s.company_id AND o.id=s.obligation_id
   WHERE s.company_id=e.company_id AND s.recipient=e.id
    AND s.state IN ('pending','observed','acknowledged','applied') AND o.state IN ('pending','observed','applied')
  ) AS pending_peer_work
) facts;

CREATE INDEX employee_schedules_due ON employee_schedules(company_id,next_due_at,employee_id)
 WHERE state IN ('sleeping','waiting_quota');

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM employee_schedules
  WHERE state<>'sleeping' OR work_generation<>0 OR checked_generation<>0 OR next_due_at IS NOT NULL OR pause_reason IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot roll back employee schedule state without losing pending work or lifecycle history';
 END IF;
END
$$;
-- +goose StatementEnd
DROP TABLE employee_schedules;
