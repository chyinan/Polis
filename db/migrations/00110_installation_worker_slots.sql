-- +goose Up
-- Installation-wide Worker slot policy. A null pair is deliberately
-- unconfigured: Worker admission must remain closed until the owner chooses
-- both the total cap and the protected recovery/verification reserve.
CREATE TABLE installation_worker_slot_policy (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 max_active_slots bigint,
 protected_slots bigint,
 revision bigint NOT NULL DEFAULT 0 CHECK(revision >= 0),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK((max_active_slots IS NULL AND protected_slots IS NULL) OR
       (max_active_slots IS NOT NULL AND protected_slots IS NOT NULL AND
        max_active_slots > 0 AND protected_slots >= 0 AND protected_slots <= max_active_slots))
);
INSERT INTO installation_worker_slot_policy(singleton) VALUES(true);

CREATE TABLE installation_worker_slot_policy_events (
 revision bigint PRIMARY KEY CHECK(revision > 0),
 request_id text NOT NULL UNIQUE CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 max_active_slots bigint NOT NULL CHECK(max_active_slots > 0),
 protected_slots bigint NOT NULL CHECK(protected_slots >= 0 AND protected_slots <= max_active_slots),
 occurred_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TRIGGER installation_worker_slot_policy_events_immutable
 BEFORE UPDATE OR DELETE ON installation_worker_slot_policy_events FOR EACH ROW
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER installation_worker_slot_policy_events_no_truncate
 BEFORE TRUNCATE ON installation_worker_slot_policy_events FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- A WorkerSession holds one installation slot until it reaches `stopped`.
-- Keep its original admission class as durable attribution; active capacity is
-- counted by joining this ledger to WorkerSession state.
CREATE TABLE installation_worker_slot_reservations (
 company_id text NOT NULL,
 worker_session_id text NOT NULL,
 slot_class text NOT NULL CHECK(slot_class IN ('ordinary','protected')),
 reserved_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,worker_session_id),
 FOREIGN KEY(company_id,worker_session_id) REFERENCES worker_sessions(company_id,id)
);
CREATE INDEX installation_worker_slot_reservations_class
 ON installation_worker_slot_reservations(slot_class,company_id,worker_session_id);

-- Preserve capacity accounting for sessions that predate the global ledger.
INSERT INTO installation_worker_slot_reservations(company_id,worker_session_id,slot_class)
SELECT s.company_id,s.id,
       CASE WHEN t.kind IN ('review','peer_review') THEN 'protected' ELSE 'ordinary' END
FROM worker_sessions s
JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
WHERE s.state<>'stopped';

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM installation_worker_slot_policy_events) OR
    EXISTS(SELECT 1 FROM installation_worker_slot_reservations) OR
    EXISTS(SELECT 1 FROM installation_worker_slot_policy WHERE revision>0 OR max_active_slots IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot discard installation Worker slot policy or admission history';
 END IF;
END
$$;
-- +goose StatementEnd
DROP INDEX installation_worker_slot_reservations_class;
DROP TABLE installation_worker_slot_reservations;
DROP TRIGGER installation_worker_slot_policy_events_no_truncate ON installation_worker_slot_policy_events;
DROP TRIGGER installation_worker_slot_policy_events_immutable ON installation_worker_slot_policy_events;
DROP TABLE installation_worker_slot_policy_events;
DROP TABLE installation_worker_slot_policy;
