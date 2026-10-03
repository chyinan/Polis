-- +goose Up
-- New correction actions retain the exact durable WorkerSession and Task that
-- authorized them. Legacy rows remain unbound because their session history
-- is not reconstructable.
ALTER TABLE worker_sessions
 ADD CONSTRAINT worker_sessions_memory_command_identity
 UNIQUE(company_id,id,employee_id,task_id);

ALTER TABLE memory_correction_requests
 ADD COLUMN proposer_session_id text,
 ADD COLUMN proposer_task_id text,
 ADD CONSTRAINT memory_correction_proposer_session_pair
  CHECK((proposer_session_id IS NULL)=(proposer_task_id IS NULL)),
 ADD CONSTRAINT memory_correction_proposer_session_identity
  FOREIGN KEY(company_id,proposer_session_id,proposed_by,proposer_task_id)
  REFERENCES worker_sessions(company_id,id,employee_id,task_id);

ALTER TABLE memory_correction_review_events
 ADD COLUMN reviewer_session_id text,
 ADD COLUMN reviewer_task_id text,
 ADD CONSTRAINT memory_correction_reviewer_session_pair
  CHECK((reviewer_session_id IS NULL)=(reviewer_task_id IS NULL)),
 ADD CONSTRAINT memory_correction_reviewer_session_identity
  FOREIGN KEY(company_id,reviewer_session_id,actor,reviewer_task_id)
  REFERENCES worker_sessions(company_id,id,employee_id,task_id);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM memory_correction_requests WHERE proposer_session_id IS NOT NULL) OR
    EXISTS(SELECT 1 FROM memory_correction_review_events WHERE reviewer_session_id IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot roll back memory correction WorkerSession provenance';
 END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE memory_correction_review_events
 DROP CONSTRAINT memory_correction_reviewer_session_identity,
 DROP CONSTRAINT memory_correction_reviewer_session_pair,
 DROP COLUMN reviewer_session_id,
 DROP COLUMN reviewer_task_id;
ALTER TABLE memory_correction_requests
 DROP CONSTRAINT memory_correction_proposer_session_identity,
 DROP CONSTRAINT memory_correction_proposer_session_pair,
 DROP COLUMN proposer_session_id,
 DROP COLUMN proposer_task_id;
ALTER TABLE worker_sessions DROP CONSTRAINT worker_sessions_memory_command_identity;
