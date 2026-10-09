-- +goose Up
-- Runtime grants intentionally exclude DELETE. Keep the last slot reference;
-- authoritative append-only lease events determine whether it is active.
-- Reusing that pointer is allowed only after its old lease is terminal.
DROP TRIGGER task_takeover_active_slots_immutable_update ON task_takeover_active_slots;
-- +goose StatementBegin
CREATE FUNCTION enforce_takeover_slot_reuse() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE prior_state text;
BEGIN
 SELECT state INTO prior_state FROM task_takeover_lease_events
 WHERE company_id=OLD.company_id AND lease_id=OLD.lease_id ORDER BY event_seq DESC LIMIT 1;
 IF prior_state NOT IN ('returned','released') OR prior_state IS NULL
    OR NEW.company_id IS DISTINCT FROM OLD.company_id OR NEW.task_id IS DISTINCT FROM OLD.task_id
    OR NEW.mission_id IS DISTINCT FROM OLD.mission_id OR NEW.lease_id IS NOT DISTINCT FROM OLD.lease_id THEN
  RAISE EXCEPTION 'takeover slot reuse requires an unchanged Task and a terminal prior lease' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER task_takeover_active_slots_immutable_update
 BEFORE UPDATE ON task_takeover_active_slots
 FOR EACH ROW EXECUTE FUNCTION enforce_takeover_slot_reuse();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM task_takeover_active_slots) THEN
  RAISE EXCEPTION 'cannot roll back retained takeover slot references';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER task_takeover_active_slots_immutable_update ON task_takeover_active_slots;
DROP FUNCTION enforce_takeover_slot_reuse();
CREATE TRIGGER task_takeover_active_slots_immutable_update
 BEFORE UPDATE ON task_takeover_active_slots
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
