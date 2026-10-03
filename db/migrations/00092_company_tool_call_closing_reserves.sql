-- +goose Up
CREATE TABLE company_tool_call_closing_reserves (
 company_id text NOT NULL,
 revision bigint NOT NULL CHECK(revision>0),
 request_id text NOT NULL,
 expected_reserve_revision bigint NOT NULL CHECK(expected_reserve_revision>=0),
 expected_company_budget_revision bigint NOT NULL CHECK(expected_company_budget_revision>=0),
 expected_company_tool_call_limit bigint CHECK(expected_company_tool_call_limit IS NULL OR expected_company_tool_call_limit>0),
 previous_reserved_tool_calls bigint NOT NULL CHECK(previous_reserved_tool_calls>=0),
 reserved_tool_calls bigint NOT NULL CHECK(reserved_tool_calls>=0),
 closing_tool_calls_used_at_revision bigint NOT NULL CHECK(closing_tool_calls_used_at_revision>=0),
 authorized_by text NOT NULL CHECK(authorized_by='local-owner'),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 500),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,revision),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 CHECK((expected_company_tool_call_limit IS NULL AND expected_company_budget_revision=0) OR
       (expected_company_tool_call_limit IS NOT NULL AND expected_company_budget_revision>0))
);

-- The policy is bounded by Company calls consumed by fixed review classes.
-- +goose StatementBegin
CREATE FUNCTION validate_company_tool_call_closing_reserve() RETURNS trigger AS $$
DECLARE current_limit bigint; current_used bigint; current_budget_revision bigint;
        reserve_revision bigint; old_reserved bigint; closing_used bigint;
BEGIN
 SELECT company_tool_call_limit,company_tool_calls_used,company_tool_call_budget_revision
 INTO current_limit,current_used,current_budget_revision
 FROM companies WHERE id=NEW.company_id FOR UPDATE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'Company budget is missing' USING ERRCODE='23514';
 END IF;
 SELECT revision,reserved_tool_calls INTO reserve_revision,old_reserved
 FROM company_tool_call_closing_reserves WHERE company_id=NEW.company_id ORDER BY revision DESC LIMIT 1;
 IF NOT FOUND THEN reserve_revision:=0; old_reserved:=0; END IF;
 SELECT COALESCE(sum(task_tool_calls_used),0)::bigint INTO closing_used
 FROM tasks WHERE company_id=NEW.company_id AND kind IN ('review','peer_review');
 IF NEW.revision<>reserve_revision+1 OR NEW.expected_reserve_revision<>reserve_revision OR
    NEW.previous_reserved_tool_calls<>old_reserved OR
    NEW.expected_company_budget_revision<>current_budget_revision OR
    NEW.expected_company_tool_call_limit IS DISTINCT FROM current_limit OR
    NEW.closing_tool_calls_used_at_revision<>closing_used OR
    (current_limit IS NOT NULL AND NEW.reserved_tool_calls::numeric>current_limit::numeric-current_used::numeric) THEN
  RAISE EXCEPTION 'Company closing reserve revision or available balance is stale' USING ERRCODE='40001';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER company_tool_call_closing_reserves_validate
 BEFORE INSERT ON company_tool_call_closing_reserves FOR EACH ROW EXECUTE FUNCTION validate_company_tool_call_closing_reserve();
CREATE TRIGGER company_tool_call_closing_reserves_immutable
 BEFORE UPDATE OR DELETE ON company_tool_call_closing_reserves FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER company_tool_call_closing_reserves_no_truncate
 BEFORE TRUNCATE ON company_tool_call_closing_reserves FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- A reserve may be set before the Company cap. Its first cap must cover the
-- remaining reserve after durable recorded usage.
-- +goose StatementBegin
CREATE FUNCTION validate_initial_company_tool_call_closing_reserve() RETURNS trigger AS $$
DECLARE reserved bigint; used_at_revision bigint; closing_used bigint; reserve_remaining bigint;
BEGIN
 IF OLD.company_tool_call_limit IS NOT NULL OR NEW.company_tool_call_limit IS NULL THEN RETURN NEW; END IF;
 SELECT COALESCE((SELECT r.reserved_tool_calls FROM company_tool_call_closing_reserves r
          WHERE r.company_id=NEW.id ORDER BY r.revision DESC LIMIT 1),0),
        COALESCE((SELECT r.closing_tool_calls_used_at_revision FROM company_tool_call_closing_reserves r
          WHERE r.company_id=NEW.id ORDER BY r.revision DESC LIMIT 1),0)
 INTO reserved,used_at_revision;
 SELECT COALESCE(sum(task_tool_calls_used),0)::bigint INTO closing_used
 FROM tasks WHERE company_id=NEW.id AND kind IN ('review','peer_review');
 IF closing_used<used_at_revision THEN
  RAISE EXCEPTION 'Company closing usage moved backwards' USING ERRCODE='55000';
 END IF;
 reserve_remaining:=GREATEST(reserved-(closing_used-used_at_revision),0);
 IF reserve_remaining::numeric>NEW.company_tool_call_limit::numeric-NEW.company_tool_calls_used::numeric THEN
  RAISE EXCEPTION 'initial Company cap cannot cover its closing reserve' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER company_tool_call_closing_reserve_initial_cap
 BEFORE UPDATE OF company_tool_call_limit ON companies
 FOR EACH ROW WHEN (OLD.company_tool_call_limit IS NULL AND NEW.company_tool_call_limit IS NOT NULL)
 EXECUTE FUNCTION validate_initial_company_tool_call_closing_reserve();

ALTER TABLE company_tool_call_budget_rejections
 ADD COLUMN company_closing_reserve_tool_calls bigint NOT NULL DEFAULT 0 CHECK(company_closing_reserve_tool_calls>=0),
 ADD COLUMN company_closing_reserve_remaining bigint NOT NULL DEFAULT 0
   CHECK(company_closing_reserve_remaining>=0 AND company_closing_reserve_remaining<=company_closing_reserve_tool_calls),
 ADD COLUMN company_closing_reserve_revision bigint NOT NULL DEFAULT 0 CHECK(company_closing_reserve_revision>=0);
ALTER TABLE company_tool_call_budget_rejections
 DROP CONSTRAINT company_tool_call_budget_rejections_reason_check,
 ADD CONSTRAINT company_tool_call_budget_rejections_reason_check
 CHECK(reason IN ('company_budget_pending','company_limit','company_closing_reserve'));

-- Replace the Schema 91 validation with the complete Company and reserve
-- snapshot check while retaining Company -> Task -> WorkerSession lock order.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION validate_company_tool_call_budget_rejection() RETURNS trigger AS $$
DECLARE current_limit bigint; current_used bigint; current_revision bigint;
        current_reserve bigint; current_reserve_revision bigint; reserve_baseline bigint;
        closing_used bigint; reserve_remaining bigint; locked_task_id text; task_kind text; session_task text;
BEGIN
 SELECT company_tool_call_limit,company_tool_calls_used,company_tool_call_budget_revision
 INTO current_limit,current_used,current_revision FROM companies WHERE id=NEW.company_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'Company tool-call budget is missing' USING ERRCODE='23503'; END IF;
 SELECT id,kind INTO locked_task_id,task_kind FROM tasks
 WHERE company_id=NEW.company_id AND id=NEW.task_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'Company budget rejection Task is missing' USING ERRCODE='23514'; END IF;
 IF NEW.worker_session_id IS NOT NULL THEN
  SELECT task_id INTO session_task FROM worker_sessions
  WHERE company_id=NEW.company_id AND id=NEW.worker_session_id FOR UPDATE;
 END IF;
 SELECT COALESCE((SELECT r.reserved_tool_calls FROM company_tool_call_closing_reserves r
          WHERE r.company_id=NEW.company_id ORDER BY r.revision DESC LIMIT 1),0),
        COALESCE((SELECT r.revision FROM company_tool_call_closing_reserves r
          WHERE r.company_id=NEW.company_id ORDER BY r.revision DESC LIMIT 1),0),
        COALESCE((SELECT r.closing_tool_calls_used_at_revision FROM company_tool_call_closing_reserves r
          WHERE r.company_id=NEW.company_id ORDER BY r.revision DESC LIMIT 1),0)
 INTO current_reserve,current_reserve_revision,reserve_baseline;
 SELECT COALESCE(sum(task_tool_calls_used),0)::bigint INTO closing_used
 FROM tasks WHERE company_id=NEW.company_id AND kind IN ('review','peer_review');
 IF closing_used<reserve_baseline THEN
  RAISE EXCEPTION 'Company closing usage moved backwards' USING ERRCODE='55000';
 END IF;
 reserve_remaining:=GREATEST(current_reserve-(closing_used-reserve_baseline),0);
 IF locked_task_id IS DISTINCT FROM NEW.task_id OR
    current_limit IS DISTINCT FROM NEW.company_tool_call_limit OR
    current_used<>NEW.company_tool_calls_used OR current_revision<>NEW.company_budget_revision OR
    current_reserve<>NEW.company_closing_reserve_tool_calls OR
    reserve_remaining<>NEW.company_closing_reserve_remaining OR
    current_reserve_revision<>NEW.company_closing_reserve_revision OR
    (NEW.reason='company_budget_pending' AND current_limit IS NOT NULL) OR
    (NEW.reason='company_limit' AND (current_limit IS NULL OR current_used<current_limit)) OR
    (NEW.reason='company_closing_reserve' AND (current_limit IS NULL OR current_used>=current_limit OR
      task_kind IN ('review','peer_review') OR current_limit::numeric-current_used::numeric>reserve_remaining::numeric)) OR
    (NEW.worker_session_id IS NOT NULL AND session_task IS DISTINCT FROM NEW.task_id) THEN
  RAISE EXCEPTION 'Company budget rejection snapshot or reason is stale' USING ERRCODE='40001';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM company_tool_call_closing_reserves) OR
    EXISTS(SELECT 1 FROM company_tool_call_budget_rejections WHERE reason='company_closing_reserve') THEN
  RAISE EXCEPTION 'cannot remove Company closing reserve policy or rejection history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER company_tool_call_closing_reserve_initial_cap ON companies;
DROP FUNCTION validate_initial_company_tool_call_closing_reserve();
DROP TRIGGER company_tool_call_closing_reserves_no_truncate ON company_tool_call_closing_reserves;
DROP TRIGGER company_tool_call_closing_reserves_immutable ON company_tool_call_closing_reserves;
DROP TRIGGER company_tool_call_closing_reserves_validate ON company_tool_call_closing_reserves;
DROP FUNCTION validate_company_tool_call_closing_reserve();
DROP TABLE company_tool_call_closing_reserves;
ALTER TABLE company_tool_call_budget_rejections
 DROP CONSTRAINT company_tool_call_budget_rejections_reason_check,
 DROP COLUMN company_closing_reserve_revision,
 DROP COLUMN company_closing_reserve_remaining,
 DROP COLUMN company_closing_reserve_tool_calls,
 ADD CONSTRAINT company_tool_call_budget_rejections_reason_check
 CHECK(reason IN ('company_budget_pending','company_limit'));
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION validate_company_tool_call_budget_rejection() RETURNS trigger AS $$
DECLARE current_limit bigint; current_used bigint; current_revision bigint;
        locked_task_id text; task_found boolean := false; session_task text;
BEGIN
 SELECT company_tool_call_limit,company_tool_calls_used,company_tool_call_budget_revision
 INTO current_limit,current_used,current_revision FROM companies WHERE id=NEW.company_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'Company tool-call budget is missing' USING ERRCODE='23503'; END IF;
 SELECT id INTO locked_task_id FROM tasks WHERE company_id=NEW.company_id AND id=NEW.task_id FOR UPDATE;
 task_found:=FOUND;
 IF NEW.worker_session_id IS NOT NULL THEN
  SELECT task_id INTO session_task FROM worker_sessions WHERE company_id=NEW.company_id AND id=NEW.worker_session_id FOR UPDATE;
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
