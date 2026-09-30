-- +goose Up
CREATE TABLE feedback_backlog_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL CHECK(event_id ~ '^[a-zA-Z0-9_-]{1,80}$'),
 source_id text NOT NULL,
 provider_item_id bigint NOT NULL CHECK(provider_item_id>0),
 revision_sha256 text NOT NULL CHECK(revision_sha256 ~ '^[a-f0-9]{64}$'),
 status text NOT NULL CHECK(status IN ('open','needs_review','triaging','waiting','handled','archived')),
 reason_code text NOT NULL CHECK(reason_code ~ '^[a-z0-9_-]{1,96}$'),
 rationale text NOT NULL CHECK(octet_length(rationale)<=512),
 actor text NOT NULL CHECK(actor IN ('system','local-owner')),
 request_id text NOT NULL CHECK(octet_length(request_id) BETWEEN 1 AND 80),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,source_id,provider_item_id,revision_sha256)
   REFERENCES feedback_observations(company_id,source_id,provider_item_id,revision_sha256),
 CHECK((actor='system' AND rationale='') OR (actor='local-owner' AND octet_length(btrim(rationale)) BETWEEN 1 AND 512)),
 CHECK(status!='needs_review' OR actor='system')
);
CREATE INDEX feedback_backlog_events_latest
 ON feedback_backlog_events(company_id,source_id,provider_item_id,event_seq DESC);

CREATE TRIGGER feedback_backlog_events_immutable
 BEFORE UPDATE OR DELETE ON feedback_backlog_events
 FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();

-- +goose Down
DROP TRIGGER feedback_backlog_events_immutable ON feedback_backlog_events;
DROP TABLE feedback_backlog_events;
