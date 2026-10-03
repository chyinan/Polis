-- +goose Up
ALTER TABLE companies
 ADD COLUMN company_tool_call_limit bigint,
 ADD COLUMN company_tool_calls_used bigint NOT NULL DEFAULT 0,
 ADD COLUMN company_tool_call_budget_revision bigint NOT NULL DEFAULT 0;

-- A Company usage projection derives from the single durable Task call count,
-- not from summing its Mission and ProblemKey projections again.
UPDATE companies c SET company_tool_calls_used=COALESCE((
 SELECT sum(t.task_tool_calls_used)::bigint FROM tasks t WHERE t.company_id=c.id
),0);

ALTER TABLE companies ADD CONSTRAINT companies_tool_call_budget_valid CHECK(
 company_tool_calls_used>=0 AND company_tool_call_budget_revision>=0 AND
 (company_tool_call_limit IS NULL OR
  (company_tool_call_limit>0 AND company_tool_calls_used<=company_tool_call_limit))
);

CREATE TABLE company_tool_call_budget_allocations (
 company_id text NOT NULL,
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
 PRIMARY KEY(company_id,revision),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 CHECK((operation='configure' AND previous_limit IS NULL AND expected_revision=0 AND additional_tool_calls=resulting_limit) OR
       (operation='allocate' AND previous_limit IS NOT NULL AND expected_revision>0 AND resulting_limit::numeric=previous_limit::numeric+additional_tool_calls::numeric))
);

-- All application writes already hold the Company lifecycle row lock first.
-- The matching append-only row is required for every cap or revision change.
-- +goose StatementBegin
CREATE FUNCTION validate_company_tool_call_budget_allocation() RETURNS trigger AS $$
DECLARE current_limit bigint; current_used bigint; current_revision bigint;
BEGIN
 SELECT company_tool_call_limit,company_tool_calls_used,company_tool_call_budget_revision
 INTO current_limit,current_used,current_revision
 FROM companies WHERE id=NEW.company_id FOR UPDATE;
 IF NOT FOUND OR NEW.expected_revision<>current_revision OR NEW.revision<>current_revision+1 THEN
  RAISE EXCEPTION 'Company tool-call budget revision is stale' USING ERRCODE='40001';
 END IF;
 IF current_limit IS NULL THEN
  IF NEW.operation<>'configure' OR NEW.previous_limit IS NOT NULL OR NEW.resulting_limit<current_used THEN
   RAISE EXCEPTION 'pending Company requires a finite cap covering recorded usage' USING ERRCODE='23514';
  END IF;
 ELSE
  IF NEW.operation<>'allocate' OR NEW.previous_limit IS DISTINCT FROM current_limit OR
     NEW.resulting_limit::numeric<>current_limit::numeric+NEW.additional_tool_calls::numeric THEN
   RAISE EXCEPTION 'Company cap can only increase through a continuous allocation' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER company_tool_call_budget_allocations_validate
 BEFORE INSERT ON company_tool_call_budget_allocations FOR EACH ROW
 EXECUTE FUNCTION validate_company_tool_call_budget_allocation();

-- Usage is monotonic; a finite company cap is never silently lowered.
-- +goose StatementBegin
CREATE FUNCTION guard_company_tool_call_budget_update() RETURNS trigger AS $$
BEGIN
 IF NEW.company_tool_calls_used<OLD.company_tool_calls_used THEN
  RAISE EXCEPTION 'Company tool-call usage cannot decrease' USING ERRCODE='55000';
 END IF;
 IF NEW.company_tool_call_limit IS DISTINCT FROM OLD.company_tool_call_limit OR
    NEW.company_tool_call_budget_revision IS DISTINCT FROM OLD.company_tool_call_budget_revision THEN
  IF NEW.company_tool_call_budget_revision<>OLD.company_tool_call_budget_revision+1 OR
     NOT EXISTS(SELECT 1 FROM company_tool_call_budget_allocations a
       WHERE a.company_id=NEW.id AND a.revision=NEW.company_tool_call_budget_revision AND
         a.previous_limit IS NOT DISTINCT FROM OLD.company_tool_call_limit AND
         a.resulting_limit=NEW.company_tool_call_limit) THEN
   RAISE EXCEPTION 'Company cap changes require a matching owner allocation record' USING ERRCODE='55000';
  END IF;
 END IF;
 IF NEW.company_tool_call_limit IS NOT NULL AND NEW.company_tool_calls_used>NEW.company_tool_call_limit THEN
  RAISE EXCEPTION 'Company tool-call usage exceeds its cap' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER companies_tool_call_budget_guard
 BEFORE UPDATE OF company_tool_call_limit,company_tool_calls_used,company_tool_call_budget_revision ON companies
 FOR EACH ROW EXECUTE FUNCTION guard_company_tool_call_budget_update();
CREATE TRIGGER company_tool_call_budget_allocations_immutable
 BEFORE UPDATE OR DELETE ON company_tool_call_budget_allocations FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER company_tool_call_budget_allocations_no_truncate
 BEFORE TRUNCATE ON company_tool_call_budget_allocations FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE company_tool_call_budget_rejections (
 company_id text NOT NULL,
 request_id text NOT NULL,
 task_id text NOT NULL,
 worker_session_id text,
 route text NOT NULL CHECK(route IN ('worker_admission','worker_tool_call')),
 reason text NOT NULL CHECK(reason IN ('company_budget_pending','company_limit')),
 company_tool_call_limit bigint CHECK(company_tool_call_limit IS NULL OR company_tool_call_limit>0),
 company_tool_calls_used bigint NOT NULL CHECK(company_tool_calls_used>=0),
 company_budget_revision bigint NOT NULL CHECK(company_budget_revision>=0),
 occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,request_id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,worker_session_id) REFERENCES worker_sessions(company_id,id),
 CHECK((route='worker_admission' AND worker_session_id IS NULL) OR
       (route='worker_tool_call' AND worker_session_id IS NOT NULL))
);

-- Rejection snapshots must match the Company counter under its lifecycle lock
-- and name a Task/session in the same Company.
-- +goose StatementBegin
CREATE FUNCTION validate_company_tool_call_budget_rejection() RETURNS trigger AS $$
DECLARE current_limit bigint; current_used bigint; current_revision bigint;
        locked_task_id text; task_found boolean := false; session_task text;
BEGIN
 SELECT company_tool_call_limit,company_tool_calls_used,company_tool_call_budget_revision
 INTO current_limit,current_used,current_revision FROM companies WHERE id=NEW.company_id FOR UPDATE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'Company tool-call budget is missing' USING ERRCODE='23503';
 END IF;
 SELECT id INTO locked_task_id FROM tasks
 WHERE company_id=NEW.company_id AND id=NEW.task_id FOR UPDATE;
 task_found := FOUND;
 IF NEW.worker_session_id IS NOT NULL THEN
  SELECT task_id INTO session_task FROM worker_sessions
  WHERE company_id=NEW.company_id AND id=NEW.worker_session_id FOR UPDATE;
 END IF;
 IF NOT task_found OR locked_task_id IS DISTINCT FROM NEW.task_id OR
    current_limit IS DISTINCT FROM NEW.company_tool_call_limit OR
    current_used<>NEW.company_tool_calls_used OR current_revision<>NEW.company_budget_revision OR
    (NEW.reason='company_budget_pending' AND current_limit IS NOT NULL) OR
    (NEW.reason='company_limit' AND (current_limit IS NULL OR current_used<current_limit)) OR
    (NEW.worker_session_id IS NOT NULL AND session_task IS DISTINCT FROM NEW.task_id) THEN
  RAISE EXCEPTION 'Company tool-call rejection snapshot or reason is stale' USING ERRCODE='40001';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER company_tool_call_budget_rejections_validate
 BEFORE INSERT ON company_tool_call_budget_rejections FOR EACH ROW
 EXECUTE FUNCTION validate_company_tool_call_budget_rejection();
CREATE TRIGGER company_tool_call_budget_rejections_immutable
 BEFORE UPDATE OR DELETE ON company_tool_call_budget_rejections FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER company_tool_call_budget_rejections_no_truncate
 BEFORE TRUNCATE ON company_tool_call_budget_rejections FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE INDEX company_tool_call_budget_rejections_recent
 ON company_tool_call_budget_rejections(company_id,occurred_at DESC,request_id DESC);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM company_tool_call_budget_allocations) OR
    EXISTS(SELECT 1 FROM company_tool_call_budget_rejections) OR
    EXISTS(SELECT 1 FROM companies WHERE company_tool_call_limit IS NOT NULL OR company_tool_calls_used<>0) THEN
  RAISE EXCEPTION 'cannot remove persisted Company tool-call budget decisions or usage';
 END IF;
END $$;
-- +goose StatementEnd
DROP INDEX company_tool_call_budget_rejections_recent;
DROP TRIGGER company_tool_call_budget_rejections_no_truncate ON company_tool_call_budget_rejections;
DROP TRIGGER company_tool_call_budget_rejections_immutable ON company_tool_call_budget_rejections;
DROP TRIGGER company_tool_call_budget_rejections_validate ON company_tool_call_budget_rejections;
DROP FUNCTION validate_company_tool_call_budget_rejection();
DROP TABLE company_tool_call_budget_rejections;
DROP TRIGGER company_tool_call_budget_allocations_no_truncate ON company_tool_call_budget_allocations;
DROP TRIGGER company_tool_call_budget_allocations_immutable ON company_tool_call_budget_allocations;
DROP TRIGGER companies_tool_call_budget_guard ON companies;
DROP TRIGGER company_tool_call_budget_allocations_validate ON company_tool_call_budget_allocations;
DROP FUNCTION guard_company_tool_call_budget_update();
DROP FUNCTION validate_company_tool_call_budget_allocation();
DROP TABLE company_tool_call_budget_allocations;
ALTER TABLE companies DROP CONSTRAINT companies_tool_call_budget_valid;
ALTER TABLE companies DROP COLUMN company_tool_call_budget_revision,
 DROP COLUMN company_tool_calls_used,
 DROP COLUMN company_tool_call_limit;
