-- +goose Up
-- Keep the terminal feedback backlog semantically tied to a genuine
-- changes_requested disposition and a terminal Mission. Foreign keys alone
-- only prove identity; this trigger preserves the lifecycle invariant.
-- +goose StatementBegin
CREATE FUNCTION guard_delivery_feedback_backlog_insert() RETURNS trigger AS $$
BEGIN
 IF NEW.status <> 'open' THEN
  RAISE EXCEPTION 'delivery feedback backlog events must start open';
 END IF;
 IF NOT EXISTS(
  SELECT 1 FROM missions m
  WHERE m.company_id=NEW.company_id AND m.id=NEW.mission_id
    AND m.state IN ('succeeded','ended_not_met','cancelled')
 ) THEN
  RAISE EXCEPTION 'delivery feedback backlog requires a terminal Mission';
 END IF;
 IF NOT EXISTS(
  SELECT 1 FROM delivery_user_dispositions d
  WHERE d.company_id=NEW.company_id AND d.delivery_id=NEW.delivery_id
    AND d.manifest_revision=NEW.manifest_revision AND d.revision=NEW.disposition_revision
    AND d.state='changes_requested' AND d.actor='installation-owner' AND d.reason=NEW.reason
 ) THEN
  RAISE EXCEPTION 'delivery feedback backlog must reference changes_requested disposition';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER delivery_feedback_backlog_insert_guard
 BEFORE INSERT ON delivery_feedback_backlog_events FOR EACH ROW
 EXECUTE FUNCTION guard_delivery_feedback_backlog_insert();

-- +goose Down
DROP TRIGGER delivery_feedback_backlog_insert_guard ON delivery_feedback_backlog_events;
DROP FUNCTION guard_delivery_feedback_backlog_insert();
