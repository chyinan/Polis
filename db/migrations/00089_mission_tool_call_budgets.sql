-- +goose Up
ALTER TABLE missions
 ADD COLUMN mission_tool_call_limit bigint,
 ADD COLUMN mission_tool_calls_used bigint NOT NULL DEFAULT 0,
 ADD COLUMN mission_tool_call_budget_revision bigint NOT NULL DEFAULT 0;

-- A protocol tool call is counted once at Task level. Sum that durable fact
-- into its Mission; do not infer any ceiling from child Task/ProblemKey caps.
UPDATE missions m SET mission_tool_calls_used=COALESCE((
 SELECT sum(t.task_tool_calls_used)::bigint FROM tasks t
 WHERE t.company_id=m.company_id AND t.mission_id=m.id
),0);

ALTER TABLE missions ADD CONSTRAINT missions_tool_call_budget_valid CHECK(
 mission_tool_calls_used>=0 AND mission_tool_call_budget_revision>=0 AND
 (mission_tool_call_limit IS NULL OR
  (mission_tool_call_limit>0 AND mission_tool_calls_used<=mission_tool_call_limit))
);

CREATE TABLE mission_tool_call_budget_allocations (
 company_id text NOT NULL,
 mission_id text NOT NULL,
 revision bigint NOT NULL CHECK(revision>0),
 request_id text NOT NULL,
 expected_revision bigint NOT NULL CHECK(expected_revision>=0),
 operation text NOT NULL CHECK(operation IN ('configure','allocate')),
 previous_limit bigint CHECK(previous_limit IS NULL OR previous_limit>0),
 additional_tool_calls bigint NOT NULL CHECK(additional_tool_calls>0),
 resulting_limit bigint NOT NULL CHECK(resulting_limit>0),
 authorized_by text NOT NULL CHECK(authorized_by='local-owner'),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 500),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,mission_id,revision),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 CHECK((operation='configure' AND previous_limit IS NULL AND expected_revision=0 AND additional_tool_calls=resulting_limit) OR
       (operation='allocate' AND previous_limit IS NOT NULL AND expected_revision>0 AND resulting_limit::numeric=previous_limit::numeric+additional_tool_calls::numeric))
);

-- Budget initialization/increases are append-only and serialized on the
-- Mission row, which is also the lifecycle lock acquired before Task locks.
-- +goose StatementBegin
CREATE FUNCTION validate_mission_tool_call_budget_allocation() RETURNS trigger AS $$
DECLARE current_limit bigint; current_used bigint; current_revision bigint;
BEGIN
 SELECT mission_tool_call_limit,mission_tool_calls_used,mission_tool_call_budget_revision
 INTO current_limit,current_used,current_revision
 FROM missions WHERE company_id=NEW.company_id AND id=NEW.mission_id FOR UPDATE;
 IF NOT FOUND OR NEW.expected_revision<>current_revision OR NEW.revision<>current_revision+1 THEN
  RAISE EXCEPTION 'Mission tool-call budget revision is stale' USING ERRCODE='40001';
 END IF;
 IF current_limit IS NULL THEN
  IF NEW.operation<>'configure' OR NEW.previous_limit IS NOT NULL OR
     NEW.resulting_limit<current_used THEN
   RAISE EXCEPTION 'pending Mission requires a finite cap covering recorded usage' USING ERRCODE='23514';
  END IF;
 ELSE
  IF NEW.operation<>'allocate' OR NEW.previous_limit IS DISTINCT FROM current_limit OR
     NEW.resulting_limit::numeric<>current_limit::numeric+NEW.additional_tool_calls::numeric THEN
   RAISE EXCEPTION 'Mission cap can only increase through a continuous allocation' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER mission_tool_call_budget_allocations_validate
 BEFORE INSERT ON mission_tool_call_budget_allocations FOR EACH ROW
 EXECUTE FUNCTION validate_mission_tool_call_budget_allocation();

-- Prevent direct cap/revision edits while allowing monotonically increasing
-- usage. The matching ledger row must be inserted first in the same tx.
-- +goose StatementBegin
CREATE FUNCTION guard_mission_tool_call_budget_update() RETURNS trigger AS $$
BEGIN
 IF NEW.mission_tool_calls_used<OLD.mission_tool_calls_used THEN
  RAISE EXCEPTION 'Mission tool-call usage cannot decrease' USING ERRCODE='55000';
 END IF;
 IF NEW.mission_tool_call_limit IS DISTINCT FROM OLD.mission_tool_call_limit OR
    NEW.mission_tool_call_budget_revision IS DISTINCT FROM OLD.mission_tool_call_budget_revision THEN
  IF NEW.mission_tool_call_budget_revision<>OLD.mission_tool_call_budget_revision+1 OR
     NOT EXISTS(SELECT 1 FROM mission_tool_call_budget_allocations a
       WHERE a.company_id=NEW.company_id AND a.mission_id=NEW.id AND
         a.revision=NEW.mission_tool_call_budget_revision AND
         a.previous_limit IS NOT DISTINCT FROM OLD.mission_tool_call_limit AND
         a.resulting_limit=NEW.mission_tool_call_limit) THEN
   RAISE EXCEPTION 'Mission cap changes require a matching owner allocation record' USING ERRCODE='55000';
  END IF;
 END IF;
 IF NEW.mission_tool_call_limit IS NOT NULL AND NEW.mission_tool_calls_used>NEW.mission_tool_call_limit THEN
  RAISE EXCEPTION 'Mission tool-call usage exceeds its cap' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER missions_tool_call_budget_guard
 BEFORE UPDATE OF mission_tool_call_limit,mission_tool_calls_used,mission_tool_call_budget_revision ON missions
 FOR EACH ROW EXECUTE FUNCTION guard_mission_tool_call_budget_update();
CREATE TRIGGER mission_tool_call_budget_allocations_immutable
 BEFORE UPDATE OR DELETE ON mission_tool_call_budget_allocations FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER mission_tool_call_budget_allocations_no_truncate
 BEFORE TRUNCATE ON mission_tool_call_budget_allocations FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE mission_tool_call_budget_rejections (
 company_id text NOT NULL,
 request_id text NOT NULL,
 mission_id text NOT NULL,
 task_id text NOT NULL,
 worker_session_id text,
 route text NOT NULL CHECK(route IN ('worker_admission','worker_tool_call')),
 reason text NOT NULL CHECK(reason IN ('mission_budget_pending','mission_limit')),
 mission_tool_call_limit bigint CHECK(mission_tool_call_limit IS NULL OR mission_tool_call_limit>0),
 mission_tool_calls_used bigint NOT NULL CHECK(mission_tool_calls_used>=0),
 mission_budget_revision bigint NOT NULL CHECK(mission_budget_revision>=0),
 occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,request_id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,worker_session_id) REFERENCES worker_sessions(company_id,id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 CHECK((route='worker_admission' AND worker_session_id IS NULL) OR
       (route='worker_tool_call' AND worker_session_id IS NOT NULL))
);

-- The deciding transaction holds Mission before Task/Session. The database
-- independently verifies that the immutable denial snapshot is current.
-- +goose StatementBegin
CREATE FUNCTION validate_mission_tool_call_budget_rejection() RETURNS trigger AS $$
DECLARE current_limit bigint; current_used bigint; current_revision bigint; task_mission text;
BEGIN
 SELECT mission_tool_call_limit,mission_tool_calls_used,mission_tool_call_budget_revision
 INTO current_limit,current_used,current_revision FROM missions
 WHERE company_id=NEW.company_id AND id=NEW.mission_id FOR UPDATE;
 SELECT mission_id INTO task_mission FROM tasks
 WHERE company_id=NEW.company_id AND id=NEW.task_id FOR UPDATE;
 IF NOT FOUND OR task_mission<>NEW.mission_id OR
    current_limit IS DISTINCT FROM NEW.mission_tool_call_limit OR
    current_used<>NEW.mission_tool_calls_used OR current_revision<>NEW.mission_budget_revision OR
    (NEW.reason='mission_budget_pending' AND current_limit IS NOT NULL) OR
    (NEW.reason='mission_limit' AND (current_limit IS NULL OR current_used<current_limit)) THEN
  RAISE EXCEPTION 'Mission budget rejection snapshot is stale or invalid' USING ERRCODE='40001';
 END IF;
 IF NEW.worker_session_id IS NOT NULL AND NOT EXISTS(
   SELECT 1 FROM worker_sessions s WHERE s.company_id=NEW.company_id AND s.id=NEW.worker_session_id AND s.task_id=NEW.task_id
 ) THEN
  RAISE EXCEPTION 'Mission budget rejection WorkerSession does not match Task' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER mission_tool_call_budget_rejections_validate
 BEFORE INSERT ON mission_tool_call_budget_rejections FOR EACH ROW
 EXECUTE FUNCTION validate_mission_tool_call_budget_rejection();
CREATE TRIGGER mission_tool_call_budget_rejections_immutable
 BEFORE UPDATE OR DELETE ON mission_tool_call_budget_rejections FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER mission_tool_call_budget_rejections_no_truncate
 BEFORE TRUNCATE ON mission_tool_call_budget_rejections FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE INDEX mission_tool_call_budget_rejections_mission_recent
 ON mission_tool_call_budget_rejections(company_id,mission_id,occurred_at DESC,request_id DESC);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM mission_tool_call_budget_allocations) OR
    EXISTS(SELECT 1 FROM mission_tool_call_budget_rejections) OR
    EXISTS(SELECT 1 FROM missions WHERE mission_tool_call_limit IS NOT NULL OR mission_tool_calls_used<>0) THEN
  RAISE EXCEPTION 'cannot remove persisted Mission tool-call budget decisions or usage';
 END IF;
END $$;
-- +goose StatementEnd
DROP INDEX mission_tool_call_budget_rejections_mission_recent;
DROP TRIGGER mission_tool_call_budget_rejections_no_truncate ON mission_tool_call_budget_rejections;
DROP TRIGGER mission_tool_call_budget_rejections_immutable ON mission_tool_call_budget_rejections;
DROP TRIGGER mission_tool_call_budget_rejections_validate ON mission_tool_call_budget_rejections;
DROP FUNCTION validate_mission_tool_call_budget_rejection();
DROP TABLE mission_tool_call_budget_rejections;
DROP TRIGGER mission_tool_call_budget_allocations_no_truncate ON mission_tool_call_budget_allocations;
DROP TRIGGER mission_tool_call_budget_allocations_immutable ON mission_tool_call_budget_allocations;
DROP TRIGGER mission_tool_call_budget_allocations_validate ON mission_tool_call_budget_allocations;
DROP TRIGGER missions_tool_call_budget_guard ON missions;
DROP FUNCTION guard_mission_tool_call_budget_update();
DROP FUNCTION validate_mission_tool_call_budget_allocation();
DROP TABLE mission_tool_call_budget_allocations;
ALTER TABLE missions DROP CONSTRAINT missions_tool_call_budget_valid;
ALTER TABLE missions
 DROP COLUMN mission_tool_call_budget_revision,
 DROP COLUMN mission_tool_calls_used,
 DROP COLUMN mission_tool_call_limit;
