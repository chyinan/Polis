-- +goose Up
ALTER TABLE missions DROP CONSTRAINT missions_state_check;
ALTER TABLE missions ADD CONSTRAINT missions_state_check
 CHECK(state IN ('draft','active','paused','closing','succeeded','ended_not_met','cancelled'));

DROP INDEX company_mission_slot;
CREATE UNIQUE INDEX company_mission_slot ON missions(company_id)
 WHERE state IN ('active','paused','closing');

CREATE TABLE mission_closeouts (
 company_id text NOT NULL,
 mission_id text NOT NULL,
 closeout_id text NOT NULL CHECK(closeout_id ~ '^[a-f0-9]{32}$'),
 requested_outcome text NOT NULL CHECK(requested_outcome IN ('succeeded','ended_not_met','cancelled')),
 rationale text NOT NULL CHECK(octet_length(rationale) BETWEEN 1 AND 4096 AND rationale=btrim(rationale)),
 acceptance_artifact_ids text[] NOT NULL DEFAULT '{}',
 request_id text NOT NULL CHECK(octet_length(request_id) BETWEEN 1 AND 80),
 opened_at timestamptz NOT NULL DEFAULT now(),
 terminal_outcome text,
 closeout_report jsonb,
 finished_at timestamptz,
 PRIMARY KEY(company_id,mission_id),
 UNIQUE(company_id,closeout_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 CHECK(terminal_outcome IS NULL OR terminal_outcome=requested_outcome),
 CHECK((terminal_outcome IS NULL AND closeout_report IS NULL AND finished_at IS NULL)
    OR (terminal_outcome IS NOT NULL AND closeout_report IS NOT NULL AND finished_at IS NOT NULL)),
 CHECK((requested_outcome='succeeded') = (cardinality(acceptance_artifact_ids)>0))
);

-- Closeout intent is admitted only after the Mission has atomically stopped
-- ordinary business work by entering closing.
-- +goose StatementBegin
CREATE FUNCTION guard_mission_closeout_insert() RETURNS trigger AS $$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM missions m WHERE m.company_id=NEW.company_id AND m.id=NEW.mission_id AND m.state='closing') THEN
  RAISE EXCEPTION 'Mission must be closing before a closeout intent is recorded';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER mission_closeout_insert_guard BEFORE INSERT ON mission_closeouts
 FOR EACH ROW EXECUTE FUNCTION guard_mission_closeout_insert();

-- The closeout intent and evidence references cannot be rewritten after entry.
-- Finalization is the only permitted update and fills only terminal columns.
-- +goose StatementBegin
CREATE FUNCTION guard_mission_closeout_update() RETURNS trigger AS $$
BEGIN
 IF OLD.terminal_outcome IS NOT NULL OR NEW.company_id<>OLD.company_id OR NEW.mission_id<>OLD.mission_id
  OR NEW.closeout_id<>OLD.closeout_id OR NEW.requested_outcome<>OLD.requested_outcome
  OR NEW.rationale<>OLD.rationale OR NEW.acceptance_artifact_ids<>OLD.acceptance_artifact_ids
  OR NEW.request_id<>OLD.request_id OR NEW.opened_at<>OLD.opened_at
  OR NEW.terminal_outcome IS NULL OR NEW.closeout_report IS NULL OR NEW.finished_at IS NULL THEN
  RAISE EXCEPTION 'Mission closeout intent is immutable and can only be finalized once';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM missions m WHERE m.company_id=NEW.company_id AND m.id=NEW.mission_id
    AND m.state=NEW.terminal_outcome) THEN
  RAISE EXCEPTION 'Mission terminal state must match its closeout record';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER mission_closeout_update_guard BEFORE UPDATE ON mission_closeouts
 FOR EACH ROW EXECUTE FUNCTION guard_mission_closeout_update();

-- Every terminal Mission settles any occurrence left pending by a crash during
-- closeout. Application finalization records task/obligation disposition first.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION cancel_routine_occurrences_for_terminal_mission() RETURNS trigger AS $$
BEGIN
 IF NEW.state NOT IN ('cancelled','succeeded','ended_not_met') OR OLD.state=NEW.state THEN
  RETURN NEW;
 END IF;
 UPDATE routine_occurrences o
 SET state='cancelled'
 FROM routines r
 WHERE o.company_id=r.company_id AND o.routine_id=r.id
  AND r.company_id=NEW.company_id AND r.mission_id=NEW.id
  AND (o.state IN ('pending','needs_instruction') OR (o.state='delivered' AND EXISTS(
   SELECT 1 FROM tasks t WHERE t.company_id=o.company_id AND t.id=o.task_id AND t.state='ready'
  )));
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- Closeout history and ended_not_met are durable lifecycle evidence.
-- +goose StatementBegin
DO $$
BEGIN
 RAISE EXCEPTION 'Mission closeout state machine is forward-only';
END
$$;
-- +goose StatementEnd
