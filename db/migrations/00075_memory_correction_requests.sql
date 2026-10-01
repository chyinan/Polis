-- +goose Up
-- A correction is a source-backed proposal. Approval and the replacement
-- revision, old-revision supersession, and dependent-work invalidations commit
-- together in one Kernel transaction.
CREATE TABLE memory_correction_requests (
 company_id text NOT NULL,
 correction_id text NOT NULL,
 record_id text NOT NULL,
 base_revision bigint NOT NULL CHECK(base_revision>0),
 proposed_content text NOT NULL CHECK(octet_length(proposed_content) BETWEEN 1 AND 32768),
 content_sha256 text NOT NULL CHECK(content_sha256 ~ '^[0-9a-f]{64}$'),
 source_kind text NOT NULL CHECK(source_kind IN ('mission_input','artifact')),
 source_id text NOT NULL,
 source_revision bigint NOT NULL CHECK(source_revision>0),
 source_sha256 text NOT NULL CHECK(source_sha256 ~ '^[0-9a-f]{64}$'),
 observed_at timestamptz NOT NULL,
 proposer_reason text NOT NULL CHECK(octet_length(proposer_reason) BETWEEN 1 AND 2048),
 proposed_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,correction_id),
 FOREIGN KEY(company_id,record_id,base_revision) REFERENCES memory_record_revisions(company_id,record_id,revision),
 FOREIGN KEY(company_id,proposed_by) REFERENCES employees(company_id,id),
 CHECK(source_kind!='artifact' OR source_revision=1)
);
CREATE INDEX memory_correction_requests_base ON memory_correction_requests(company_id,record_id,base_revision,created_at,correction_id);
CREATE TRIGGER memory_correction_requests_immutable
 BEFORE UPDATE OR DELETE ON memory_correction_requests
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER memory_correction_requests_no_truncate
 BEFORE TRUNCATE ON memory_correction_requests
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

CREATE TABLE memory_correction_review_events (
 company_id text NOT NULL,
 company_seq bigint NOT NULL,
 correction_id text NOT NULL,
 record_id text NOT NULL,
 decision text NOT NULL CHECK(decision IN ('approved','rejected')),
 actor text NOT NULL,
 reason text NOT NULL CHECK(octet_length(reason) BETWEEN 1 AND 2048),
 result_revision bigint,
 PRIMARY KEY(company_id,company_seq),
 UNIQUE(company_id,correction_id),
 FOREIGN KEY(company_id,company_seq) REFERENCES events(company_id,company_seq),
 FOREIGN KEY(company_id,correction_id) REFERENCES memory_correction_requests(company_id,correction_id),
 FOREIGN KEY(company_id,record_id,result_revision) REFERENCES memory_record_revisions(company_id,record_id,revision),
 FOREIGN KEY(company_id,actor) REFERENCES employees(company_id,id),
 CHECK((decision='approved' AND result_revision IS NOT NULL) OR (decision='rejected' AND result_revision IS NULL))
);
CREATE INDEX memory_correction_reviews_record ON memory_correction_review_events(company_id,record_id,company_seq DESC);
CREATE TRIGGER memory_correction_review_events_immutable
 BEFORE UPDATE OR DELETE ON memory_correction_review_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER memory_correction_review_events_no_truncate
 BEFORE TRUNCATE ON memory_correction_review_events
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM memory_correction_requests) OR EXISTS(SELECT 1 FROM memory_correction_review_events) THEN
  RAISE EXCEPTION 'cannot roll back memory correction history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER memory_correction_review_events_no_truncate ON memory_correction_review_events;
DROP TRIGGER memory_correction_review_events_immutable ON memory_correction_review_events;
DROP INDEX memory_correction_reviews_record;
DROP TABLE memory_correction_review_events;
DROP TRIGGER memory_correction_requests_no_truncate ON memory_correction_requests;
DROP TRIGGER memory_correction_requests_immutable ON memory_correction_requests;
DROP INDEX memory_correction_requests_base;
DROP TABLE memory_correction_requests;
