-- +goose Up
CREATE TABLE feedback_source_bindings (
 company_id text NOT NULL REFERENCES companies(id),
 source_id text NOT NULL CHECK(source_id ~ '^[a-zA-Z0-9_-]{1,80}$'),
 provider text NOT NULL CHECK(provider='github'),
 repository_id bigint NOT NULL CHECK(repository_id>0),
 repository_owner text NOT NULL CHECK(repository_owner ~ '^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$'),
 repository_name text NOT NULL CHECK(repository_name ~ '^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$'),
 profile_revision text NOT NULL CHECK(profile_revision='github-issues-readonly@1'),
 filter_revision text NOT NULL CHECK(filter_revision='github-issues-overlap-updated-asc@1'),
 configuration_sha256 text NOT NULL CHECK(configuration_sha256 ~ '^[a-f0-9]{64}$'),
 created_by text NOT NULL CHECK(created_by='local-owner'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,source_id),
 UNIQUE(company_id,provider,repository_id)
);

CREATE TABLE feedback_source_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL CHECK(event_id ~ '^[a-zA-Z0-9_-]{1,80}$'),
 source_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('draft','approved','paused','revoked')),
 permission_status text NOT NULL CHECK(permission_status IN ('unverified','verified','denied','unknown')),
 permission_probe_sha256 text CHECK(permission_probe_sha256 IS NULL OR permission_probe_sha256 ~ '^[a-f0-9]{64}$'),
 rationale text NOT NULL CHECK(octet_length(rationale) BETWEEN 1 AND 512),
 actor text NOT NULL CHECK(actor='local-owner'),
 request_id text NOT NULL CHECK(octet_length(request_id) BETWEEN 1 AND 80),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,source_id) REFERENCES feedback_source_bindings(company_id,source_id),
 CHECK(permission_status!='verified' OR permission_probe_sha256 IS NOT NULL)
);
CREATE INDEX feedback_source_events_latest ON feedback_source_events(company_id,source_id,event_seq DESC);

CREATE TABLE feedback_scan_runs (
 company_id text NOT NULL,
 scan_id text NOT NULL CHECK(scan_id ~ '^[a-zA-Z0-9_-]{1,80}$'),
 source_id text NOT NULL,
 request_id text NOT NULL CHECK(octet_length(request_id) BETWEEN 1 AND 80),
 configuration_sha256 text NOT NULL CHECK(configuration_sha256 ~ '^[a-f0-9]{64}$'),
 since_cursor timestamptz,
 coverage_cutoff timestamptz NOT NULL,
 coverage_status text NOT NULL CHECK(coverage_status IN ('complete','partial')),
 coverage_reason text NOT NULL CHECK(octet_length(coverage_reason)<=96),
 covered_through timestamptz,
 page_count integer NOT NULL CHECK(page_count BETWEEN 0 AND 20),
 item_count integer NOT NULL CHECK(item_count BETWEEN 0 AND 2000),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,scan_id),
 UNIQUE(company_id,request_id),
 UNIQUE(company_id,source_id,scan_id),
 FOREIGN KEY(company_id,source_id) REFERENCES feedback_source_bindings(company_id,source_id),
 CHECK((coverage_status='complete' AND coverage_reason='' AND covered_through IS NOT NULL)
    OR (coverage_status='partial' AND coverage_reason<>'' AND covered_through IS NULL))
);
CREATE INDEX feedback_scan_runs_latest ON feedback_scan_runs(company_id,source_id,created_at DESC);

CREATE TABLE feedback_scan_pages (
 company_id text NOT NULL,
 source_id text NOT NULL,
 scan_id text NOT NULL,
 page_number integer NOT NULL CHECK(page_number BETWEEN 1 AND 20),
 response_sha256 text NOT NULL CHECK(response_sha256 ~ '^[a-f0-9]{64}$'),
 etag text CHECK(etag IS NULL OR octet_length(etag)<=512),
 item_count integer NOT NULL CHECK(item_count BETWEEN 0 AND 100),
 received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,source_id,scan_id,page_number),
 FOREIGN KEY(company_id,source_id,scan_id) REFERENCES feedback_scan_runs(company_id,source_id,scan_id)
);

CREATE TABLE feedback_observations (
 company_id text NOT NULL,
 source_id text NOT NULL,
 provider_item_id bigint NOT NULL CHECK(provider_item_id>0),
 issue_number integer NOT NULL CHECK(issue_number>0),
 revision_sha256 text NOT NULL CHECK(revision_sha256 ~ '^[a-f0-9]{64}$'),
 body_sha256 text NOT NULL CHECK(body_sha256 ~ '^[a-f0-9]{64}$'),
 title text NOT NULL CHECK(octet_length(title)<=4096),
 title_truncated boolean NOT NULL,
 body text NOT NULL CHECK(octet_length(body)<=65536),
 body_truncated boolean NOT NULL,
 issue_state text NOT NULL CHECK(issue_state IN ('open','closed')),
 source_updated_at timestamptz NOT NULL,
 observed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 html_url text NOT NULL DEFAULT '' CHECK(octet_length(html_url)<=2048),
 untrusted boolean NOT NULL DEFAULT true CHECK(untrusted),
 PRIMARY KEY(company_id,source_id,provider_item_id,revision_sha256),
 FOREIGN KEY(company_id,source_id) REFERENCES feedback_source_bindings(company_id,source_id)
);
CREATE INDEX feedback_observations_latest ON feedback_observations(company_id,source_id,provider_item_id,source_updated_at DESC);

CREATE TABLE feedback_scan_items (
 company_id text NOT NULL,
 source_id text NOT NULL,
 scan_id text NOT NULL,
 page_number integer NOT NULL,
 provider_item_id bigint NOT NULL,
 revision_sha256 text NOT NULL,
 PRIMARY KEY(company_id,source_id,scan_id,page_number,provider_item_id,revision_sha256),
 FOREIGN KEY(company_id,source_id,scan_id,page_number) REFERENCES feedback_scan_pages(company_id,source_id,scan_id,page_number),
 FOREIGN KEY(company_id,source_id,provider_item_id,revision_sha256) REFERENCES feedback_observations(company_id,source_id,provider_item_id,revision_sha256)
);

-- +goose StatementBegin
CREATE FUNCTION reject_feedback_record_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'feedback source and observation records are immutable';
END $$;
-- +goose StatementEnd
CREATE TRIGGER feedback_source_bindings_immutable BEFORE UPDATE OR DELETE ON feedback_source_bindings FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();
CREATE TRIGGER feedback_source_events_immutable BEFORE UPDATE OR DELETE ON feedback_source_events FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();
CREATE TRIGGER feedback_scan_runs_immutable BEFORE UPDATE OR DELETE ON feedback_scan_runs FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();
CREATE TRIGGER feedback_scan_pages_immutable BEFORE UPDATE OR DELETE ON feedback_scan_pages FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();
CREATE TRIGGER feedback_observations_immutable BEFORE UPDATE OR DELETE ON feedback_observations FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();
CREATE TRIGGER feedback_scan_items_immutable BEFORE UPDATE OR DELETE ON feedback_scan_items FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();

-- +goose Down
DROP TRIGGER feedback_scan_items_immutable ON feedback_scan_items;
DROP TRIGGER feedback_observations_immutable ON feedback_observations;
DROP TRIGGER feedback_scan_pages_immutable ON feedback_scan_pages;
DROP TRIGGER feedback_scan_runs_immutable ON feedback_scan_runs;
DROP TRIGGER feedback_source_events_immutable ON feedback_source_events;
DROP TRIGGER feedback_source_bindings_immutable ON feedback_source_bindings;
DROP FUNCTION reject_feedback_record_mutation();
DROP TABLE feedback_scan_items;
DROP TABLE feedback_observations;
DROP TABLE feedback_scan_pages;
DROP TABLE feedback_scan_runs;
DROP TABLE feedback_source_events;
DROP TABLE feedback_source_bindings;
