-- +goose Up
CREATE TABLE feedback_comment_scans (
 company_id text NOT NULL,
 comment_scan_id text NOT NULL CHECK(comment_scan_id ~ '^[a-zA-Z0-9_-]{1,80}$'),
 source_id text NOT NULL,
 provider_item_id bigint NOT NULL CHECK(provider_item_id>0),
 issue_revision_sha256 text NOT NULL CHECK(issue_revision_sha256 ~ '^[a-f0-9]{64}$'),
 request_id text NOT NULL CHECK(octet_length(request_id) BETWEEN 1 AND 80),
 coverage_status text NOT NULL CHECK(coverage_status IN ('complete','partial')),
 coverage_reason text NOT NULL CHECK(octet_length(coverage_reason)<=96),
 page_count integer NOT NULL CHECK(page_count BETWEEN 0 AND 20),
 comment_count integer NOT NULL CHECK(comment_count BETWEEN 0 AND 500),
 body_bytes integer NOT NULL CHECK(body_bytes BETWEEN 0 AND 1048576),
 body_truncated boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,comment_scan_id),
 UNIQUE(company_id,request_id),
 UNIQUE(company_id,source_id,comment_scan_id),
 FOREIGN KEY(company_id,source_id,provider_item_id,issue_revision_sha256) REFERENCES feedback_observations(company_id,source_id,provider_item_id,revision_sha256),
 CHECK((coverage_status='complete' AND coverage_reason='') OR (coverage_status='partial' AND coverage_reason<>''))
);
CREATE INDEX feedback_comment_scans_issue ON feedback_comment_scans(company_id,source_id,provider_item_id,created_at DESC);

CREATE TABLE feedback_comment_pages (
 company_id text NOT NULL,
 source_id text NOT NULL,
 comment_scan_id text NOT NULL,
 page_number integer NOT NULL CHECK(page_number BETWEEN 1 AND 20),
 response_sha256 text NOT NULL CHECK(response_sha256 ~ '^[a-f0-9]{64}$'),
 etag text CHECK(etag IS NULL OR octet_length(etag)<=512),
 item_count integer NOT NULL CHECK(item_count BETWEEN 0 AND 100),
 received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,source_id,comment_scan_id,page_number),
 FOREIGN KEY(company_id,source_id,comment_scan_id) REFERENCES feedback_comment_scans(company_id,source_id,comment_scan_id)
);

CREATE TABLE feedback_comment_observations (
 company_id text NOT NULL,
 source_id text NOT NULL,
 provider_item_id bigint NOT NULL,
 issue_revision_sha256 text NOT NULL,
 provider_comment_id bigint NOT NULL CHECK(provider_comment_id>0),
 revision_sha256 text NOT NULL CHECK(revision_sha256 ~ '^[a-f0-9]{64}$'),
 body_sha256 text NOT NULL CHECK(body_sha256 ~ '^[a-f0-9]{64}$'),
 body text NOT NULL CHECK(octet_length(body)<=1048576),
 body_truncated boolean NOT NULL,
 source_updated_at timestamptz NOT NULL,
 observed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 html_url text NOT NULL DEFAULT '' CHECK(octet_length(html_url)<=2048),
 untrusted boolean NOT NULL DEFAULT true CHECK(untrusted),
 PRIMARY KEY(company_id,source_id,provider_item_id,issue_revision_sha256,provider_comment_id,revision_sha256),
 FOREIGN KEY(company_id,source_id,provider_item_id,issue_revision_sha256) REFERENCES feedback_observations(company_id,source_id,provider_item_id,revision_sha256)
);

CREATE TABLE feedback_comment_scan_items (
 company_id text NOT NULL,
 source_id text NOT NULL,
 comment_scan_id text NOT NULL,
 page_number integer NOT NULL,
 provider_item_id bigint NOT NULL,
 issue_revision_sha256 text NOT NULL,
 provider_comment_id bigint NOT NULL,
 comment_revision_sha256 text NOT NULL,
 PRIMARY KEY(company_id,source_id,comment_scan_id,page_number,provider_comment_id,comment_revision_sha256),
 FOREIGN KEY(company_id,source_id,comment_scan_id,page_number) REFERENCES feedback_comment_pages(company_id,source_id,comment_scan_id,page_number),
 FOREIGN KEY(company_id,source_id,provider_item_id,issue_revision_sha256,provider_comment_id,comment_revision_sha256)
  REFERENCES feedback_comment_observations(company_id,source_id,provider_item_id,issue_revision_sha256,provider_comment_id,revision_sha256)
);

CREATE TRIGGER feedback_comment_scans_immutable BEFORE UPDATE OR DELETE ON feedback_comment_scans FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();
CREATE TRIGGER feedback_comment_pages_immutable BEFORE UPDATE OR DELETE ON feedback_comment_pages FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();
CREATE TRIGGER feedback_comment_observations_immutable BEFORE UPDATE OR DELETE ON feedback_comment_observations FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();
CREATE TRIGGER feedback_comment_scan_items_immutable BEFORE UPDATE OR DELETE ON feedback_comment_scan_items FOR EACH ROW EXECUTE FUNCTION reject_feedback_record_mutation();

-- +goose Down
DROP TRIGGER feedback_comment_scan_items_immutable ON feedback_comment_scan_items;
DROP TRIGGER feedback_comment_observations_immutable ON feedback_comment_observations;
DROP TRIGGER feedback_comment_pages_immutable ON feedback_comment_pages;
DROP TRIGGER feedback_comment_scans_immutable ON feedback_comment_scans;
DROP TABLE feedback_comment_scan_items;
DROP TABLE feedback_comment_observations;
DROP TABLE feedback_comment_pages;
DROP TABLE feedback_comment_scans;
