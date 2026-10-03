-- +goose Up
CREATE TABLE mission_tool_call_closing_reserves (
 company_id text NOT NULL,
 mission_id text NOT NULL,
 revision bigint NOT NULL CHECK(revision>0),
 request_id text NOT NULL,
 expected_reserve_revision bigint NOT NULL CHECK(expected_reserve_revision>=0),
 expected_mission_budget_revision bigint NOT NULL CHECK(expected_mission_budget_revision>=0),
 expected_mission_tool_call_limit bigint CHECK(expected_mission_tool_call_limit IS NULL OR expected_mission_tool_call_limit>0),
 previous_reserved_tool_calls bigint NOT NULL CHECK(previous_reserved_tool_calls>=0),
 reserved_tool_calls bigint NOT NULL CHECK(reserved_tool_calls>=0),
 closing_tool_calls_used_at_revision bigint NOT NULL CHECK(closing_tool_calls_used_at_revision>=0),
 authorized_by text NOT NULL CHECK(authorized_by='local-owner'),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 500),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,mission_id,revision),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 CHECK((expected_mission_tool_call_limit IS NULL AND expected_mission_budget_revision=0) OR
       (expected_mission_tool_call_limit IS NOT NULL AND expected_mission_budget_revision>0))
);

-- Policy versions bind the Mission cap snapshot and current closing-class use.
-- Only Kernel-created review and peer_review Tasks count as closing work.
-- +goose StatementBegin
CREATE FUNCTION validate_mission_tool_call_closing_reserve() RETURNS trigger AS $$
DECLARE current_limit bigint; current_used bigint; current_budget_revision bigint;
        reserve_revision bigint; old_reserved bigint; closing_used bigint;
BEGIN
 SELECT mission_tool_call_limit,mission_tool_calls_used,mission_tool_call_budget_revision
 INTO current_limit,current_used,current_budget_revision
 FROM missions WHERE company_id=NEW.company_id AND id=NEW.mission_id FOR UPDATE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'Mission budget is missing' USING ERRCODE='23514';
 END IF;
 SELECT revision,reserved_tool_calls INTO reserve_revision,old_reserved
 FROM mission_tool_call_closing_reserves
 WHERE company_id=NEW.company_id AND mission_id=NEW.mission_id
 ORDER BY revision DESC LIMIT 1;
 IF NOT FOUND THEN
  reserve_revision:=0;
  old_reserved:=0;
 END IF;
 SELECT COALESCE(sum(task_tool_calls_used),0)::bigint INTO closing_used
 FROM tasks WHERE company_id=NEW.company_id AND mission_id=NEW.mission_id AND kind IN ('review','peer_review');
 IF NEW.revision<>reserve_revision+1 OR NEW.expected_reserve_revision<>reserve_revision OR
    NEW.previous_reserved_tool_calls<>old_reserved OR
    NEW.expected_mission_budget_revision<>current_budget_revision OR
    NEW.expected_mission_tool_call_limit IS DISTINCT FROM current_limit OR
    NEW.closing_tool_calls_used_at_revision<>closing_used OR
    (current_limit IS NOT NULL AND NEW.reserved_tool_calls::numeric>current_limit::numeric-current_used::numeric) THEN
  RAISE EXCEPTION 'Mission closing reserve revision or available balance is stale' USING ERRCODE='40001';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER mission_tool_call_closing_reserves_validate
 BEFORE INSERT ON mission_tool_call_closing_reserves FOR EACH ROW EXECUTE FUNCTION validate_mission_tool_call_closing_reserve();
CREATE TRIGGER mission_tool_call_closing_reserves_immutable
 BEFORE UPDATE OR DELETE ON mission_tool_call_closing_reserves FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER mission_tool_call_closing_reserves_no_truncate
 BEFORE TRUNCATE ON mission_tool_call_closing_reserves FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- A reserve may be configured while a legacy Mission cap is pending. Its first
-- explicit finite cap must cover the unspent reserve after recorded usage.
-- +goose StatementBegin
CREATE FUNCTION validate_initial_mission_tool_call_closing_reserve() RETURNS trigger AS $$
DECLARE reserved bigint; used_at_revision bigint; closing_used bigint; reserve_remaining bigint;
BEGIN
 IF OLD.mission_tool_call_limit IS NOT NULL OR NEW.mission_tool_call_limit IS NULL THEN
  RETURN NEW;
 END IF;
 SELECT COALESCE((SELECT r.reserved_tool_calls FROM mission_tool_call_closing_reserves r
          WHERE r.company_id=NEW.company_id AND r.mission_id=NEW.id ORDER BY r.revision DESC LIMIT 1),0),
        COALESCE((SELECT r.closing_tool_calls_used_at_revision FROM mission_tool_call_closing_reserves r
          WHERE r.company_id=NEW.company_id AND r.mission_id=NEW.id ORDER BY r.revision DESC LIMIT 1),0)
 INTO reserved,used_at_revision;
 SELECT COALESCE(sum(task_tool_calls_used),0)::bigint INTO closing_used
 FROM tasks WHERE company_id=NEW.company_id AND mission_id=NEW.id AND kind IN ('review','peer_review');
 IF closing_used<used_at_revision THEN
  RAISE EXCEPTION 'Mission closing usage moved backwards' USING ERRCODE='55000';
 END IF;
 reserve_remaining:=GREATEST(reserved-(closing_used-used_at_revision),0);
 IF reserve_remaining::numeric>NEW.mission_tool_call_limit::numeric-NEW.mission_tool_calls_used::numeric THEN
  RAISE EXCEPTION 'initial Mission cap cannot cover its closing reserve' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER mission_tool_call_closing_reserve_initial_cap
 BEFORE UPDATE OF mission_tool_call_limit ON missions
 FOR EACH ROW WHEN (OLD.mission_tool_call_limit IS NULL AND NEW.mission_tool_call_limit IS NOT NULL)
 EXECUTE FUNCTION validate_initial_mission_tool_call_closing_reserve();

ALTER TABLE mission_tool_call_budget_rejections
 ADD COLUMN mission_closing_reserve_tool_calls bigint NOT NULL DEFAULT 0 CHECK(mission_closing_reserve_tool_calls>=0),
 ADD COLUMN mission_closing_reserve_remaining bigint NOT NULL DEFAULT 0
   CHECK(mission_closing_reserve_remaining>=0 AND mission_closing_reserve_remaining<=mission_closing_reserve_tool_calls),
 ADD COLUMN mission_closing_reserve_revision bigint NOT NULL DEFAULT 0 CHECK(mission_closing_reserve_revision>=0);
ALTER TABLE mission_tool_call_budget_rejections
 DROP CONSTRAINT mission_tool_call_budget_rejections_reason_check,
 ADD CONSTRAINT mission_tool_call_budget_rejections_reason_check
 CHECK(reason IN ('mission_budget_pending','mission_limit','mission_closing_reserve'));

-- Recheck the complete Mission and closing-reserve snapshot under the same
-- Mission → Task lock order used by admission and tool-call charging.
-- +goose StatementBegin
CREATE FUNCTION validate_mission_tool_call_budget_rejection_reserve() RETURNS trigger AS $$
DECLARE current_limit bigint; current_used bigint; current_budget_revision bigint;
        current_reserve bigint; current_reserve_revision bigint; reserve_baseline bigint;
        closing_used bigint; reserve_remaining bigint; task_mission text; task_kind text;
BEGIN
 SELECT mission_tool_call_limit,mission_tool_calls_used,mission_tool_call_budget_revision
 INTO current_limit,current_used,current_budget_revision FROM missions
 WHERE company_id=NEW.company_id AND id=NEW.mission_id FOR UPDATE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'Mission budget is missing' USING ERRCODE='23503';
 END IF;
 SELECT mission_id,kind INTO task_mission,task_kind FROM tasks
 WHERE company_id=NEW.company_id AND id=NEW.task_id FOR UPDATE;
 IF NOT FOUND OR task_mission<>NEW.mission_id THEN
  RAISE EXCEPTION 'Mission budget rejection Task does not match Mission' USING ERRCODE='23514';
 END IF;
 SELECT COALESCE((SELECT r.reserved_tool_calls FROM mission_tool_call_closing_reserves r
          WHERE r.company_id=NEW.company_id AND r.mission_id=NEW.mission_id ORDER BY r.revision DESC LIMIT 1),0),
        COALESCE((SELECT r.revision FROM mission_tool_call_closing_reserves r
          WHERE r.company_id=NEW.company_id AND r.mission_id=NEW.mission_id ORDER BY r.revision DESC LIMIT 1),0),
        COALESCE((SELECT r.closing_tool_calls_used_at_revision FROM mission_tool_call_closing_reserves r
          WHERE r.company_id=NEW.company_id AND r.mission_id=NEW.mission_id ORDER BY r.revision DESC LIMIT 1),0)
 INTO current_reserve,current_reserve_revision,reserve_baseline;
 SELECT COALESCE(sum(task_tool_calls_used),0)::bigint INTO closing_used
 FROM tasks WHERE company_id=NEW.company_id AND mission_id=NEW.mission_id AND kind IN ('review','peer_review');
 IF closing_used<reserve_baseline THEN
  RAISE EXCEPTION 'Mission closing usage moved backwards' USING ERRCODE='55000';
 END IF;
 reserve_remaining:=GREATEST(current_reserve-(closing_used-reserve_baseline),0);
 IF current_limit IS DISTINCT FROM NEW.mission_tool_call_limit OR
    current_used<>NEW.mission_tool_calls_used OR current_budget_revision<>NEW.mission_budget_revision OR
    current_reserve<>NEW.mission_closing_reserve_tool_calls OR
    reserve_remaining<>NEW.mission_closing_reserve_remaining OR
    current_reserve_revision<>NEW.mission_closing_reserve_revision OR
    (NEW.reason='mission_budget_pending' AND current_limit IS NOT NULL) OR
    (NEW.reason='mission_limit' AND (current_limit IS NULL OR current_used<current_limit)) OR
    (NEW.reason='mission_closing_reserve' AND (current_limit IS NULL OR current_used>=current_limit OR
      task_kind IN ('review','peer_review') OR current_limit::numeric-current_used::numeric>reserve_remaining::numeric)) THEN
  RAISE EXCEPTION 'Mission budget rejection snapshot or reason is stale' USING ERRCODE='40001';
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
CREATE TRIGGER mission_tool_call_budget_rejections_validate_reserve
 BEFORE INSERT ON mission_tool_call_budget_rejections FOR EACH ROW
 EXECUTE FUNCTION validate_mission_tool_call_budget_rejection_reserve();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM mission_tool_call_closing_reserves) OR
    EXISTS(SELECT 1 FROM mission_tool_call_budget_rejections WHERE reason='mission_closing_reserve') THEN
  RAISE EXCEPTION 'cannot remove Mission closing reserve policy or rejection history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER mission_tool_call_budget_rejections_validate_reserve ON mission_tool_call_budget_rejections;
DROP FUNCTION validate_mission_tool_call_budget_rejection_reserve();
DROP TRIGGER mission_tool_call_closing_reserve_initial_cap ON missions;
DROP FUNCTION validate_initial_mission_tool_call_closing_reserve();
DROP TRIGGER mission_tool_call_closing_reserves_no_truncate ON mission_tool_call_closing_reserves;
DROP TRIGGER mission_tool_call_closing_reserves_immutable ON mission_tool_call_closing_reserves;
DROP TRIGGER mission_tool_call_closing_reserves_validate ON mission_tool_call_closing_reserves;
DROP FUNCTION validate_mission_tool_call_closing_reserve();
DROP TABLE mission_tool_call_closing_reserves;
ALTER TABLE mission_tool_call_budget_rejections
 DROP CONSTRAINT mission_tool_call_budget_rejections_reason_check,
 DROP COLUMN mission_closing_reserve_revision,
 DROP COLUMN mission_closing_reserve_remaining,
 DROP COLUMN mission_closing_reserve_tool_calls,
 ADD CONSTRAINT mission_tool_call_budget_rejections_reason_check
 CHECK(reason IN ('mission_budget_pending','mission_limit'));
