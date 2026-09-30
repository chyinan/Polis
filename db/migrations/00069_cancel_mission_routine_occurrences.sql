-- +goose Up
-- Paused Missions preserve pending occurrences. Terminal Missions explicitly
-- cancel them so no stale recurrence remains actionable in historical work.
-- +goose StatementBegin
CREATE FUNCTION cancel_routine_occurrences_for_terminal_mission() RETURNS trigger AS $$
BEGIN
 IF NEW.state NOT IN ('cancelled','succeeded') OR OLD.state=NEW.state THEN
  RETURN NEW;
 END IF;
 UPDATE routine_occurrences o
 SET state='cancelled'
 FROM routines r
 WHERE o.company_id=r.company_id AND o.routine_id=r.id
  AND r.company_id=NEW.company_id AND r.mission_id=NEW.id
  AND o.state='pending';
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER missions_cancel_pending_routine_occurrences
 AFTER UPDATE OF state ON missions
 FOR EACH ROW EXECUTE FUNCTION cancel_routine_occurrences_for_terminal_mission();

-- +goose Down
DROP TRIGGER missions_cancel_pending_routine_occurrences ON missions;
DROP FUNCTION cancel_routine_occurrences_for_terminal_mission();
