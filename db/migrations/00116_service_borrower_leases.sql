-- +goose Up
-- Consumer-bound service leases are immutable identity plus append-only state
-- events. They do not start processes or extend the owner service endpoint.
ALTER TABLE job_runs
 ADD CONSTRAINT job_runs_service_borrower_identity UNIQUE(company_id,job_id,task_id);

CREATE TABLE service_borrower_lease_records (
 company_id text NOT NULL,
 lease_id text NOT NULL,
 job_id text NOT NULL,
 generation integer NOT NULL CHECK(generation>0),
 mission_id text NOT NULL,
 owner_task_id text NOT NULL,
 owner_session_id text NOT NULL,
 borrower_task_id text NOT NULL,
 borrower_session_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,lease_id),
 CHECK(owner_task_id<>borrower_task_id),
 UNIQUE(company_id,job_id,generation,borrower_session_id),
 FOREIGN KEY(company_id,job_id,owner_task_id) REFERENCES job_runs(company_id,job_id,task_id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,owner_task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,owner_session_id,owner_task_id) REFERENCES worker_sessions(company_id,id,task_id),
 FOREIGN KEY(company_id,borrower_task_id,mission_id) REFERENCES tasks(company_id,id,mission_id),
 FOREIGN KEY(company_id,borrower_session_id,borrower_task_id) REFERENCES worker_sessions(company_id,id,task_id)
);
CREATE INDEX service_borrower_lease_records_borrower
 ON service_borrower_lease_records(company_id,borrower_session_id,created_at DESC);
CREATE TABLE service_borrower_lease_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL,
 lease_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('active','released','revoked','expired')),
 expires_at timestamptz NOT NULL,
 idle_until timestamptz NOT NULL,
 last_used_at timestamptz NOT NULL,
 actor text NOT NULL CHECK(octet_length(btrim(actor)) BETWEEN 1 AND 80),
 reason text NOT NULL CHECK(octet_length(btrim(reason)) BETWEEN 1 AND 4096),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,lease_id) REFERENCES service_borrower_lease_records(company_id,lease_id)
);
CREATE INDEX service_borrower_lease_events_latest
 ON service_borrower_lease_events(company_id,lease_id,event_seq DESC);
CREATE TRIGGER service_borrower_lease_records_immutable
 BEFORE UPDATE OR DELETE ON service_borrower_lease_records
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER service_borrower_lease_records_no_truncate
 BEFORE TRUNCATE ON service_borrower_lease_records FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER service_borrower_lease_events_immutable
 BEFORE UPDATE OR DELETE ON service_borrower_lease_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER service_borrower_lease_events_no_truncate
 BEFORE TRUNCATE ON service_borrower_lease_events FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM service_borrower_lease_events) OR EXISTS(SELECT 1 FROM service_borrower_lease_records) THEN
  RAISE EXCEPTION 'cannot discard service borrower lease history';
 END IF;
END
$$;
DROP TRIGGER service_borrower_lease_events_no_truncate ON service_borrower_lease_events;
DROP TRIGGER service_borrower_lease_events_immutable ON service_borrower_lease_events;
DROP INDEX service_borrower_lease_events_latest;
DROP TABLE service_borrower_lease_events;
DROP TRIGGER service_borrower_lease_records_no_truncate ON service_borrower_lease_records;
DROP TRIGGER service_borrower_lease_records_immutable ON service_borrower_lease_records;
DROP INDEX service_borrower_lease_records_borrower;
DROP TABLE service_borrower_lease_records;
ALTER TABLE job_runs DROP CONSTRAINT job_runs_service_borrower_identity;
