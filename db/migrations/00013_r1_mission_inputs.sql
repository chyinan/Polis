-- +goose Up
-- Immutable mission input revisions. File bytes stay in the company-scoped CAS.
CREATE TABLE mission_inputs (
 company_id text NOT NULL,
 input_id text NOT NULL,
 mission_id text NOT NULL,
 revision bigint NOT NULL CHECK(revision > 0),
 request_id text NOT NULL CHECK(octet_length(request_id) <= 80),
 source_kind text NOT NULL CHECK(source_kind IN ('upload','directory_snapshot','git_snapshot')),
 display_name text NOT NULL CHECK(octet_length(display_name) BETWEEN 1 AND 255),
 media_type text NOT NULL CHECK(octet_length(media_type) BETWEEN 1 AND 128),
 byte_size bigint NOT NULL CHECK(byte_size BETWEEN 1 AND 8388608),
 content_digest text NOT NULL CHECK(content_digest ~ '^[0-9a-f]{64}$'),
 state text NOT NULL CHECK(state IN ('usable','partial','unsupported')),
 image_width integer NOT NULL DEFAULT 0 CHECK(image_width >= 0),
 image_height integer NOT NULL DEFAULT 0 CHECK(image_height >= 0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,input_id,revision),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 UNIQUE(company_id,request_id),
 CHECK((image_width = 0 AND image_height = 0) OR (image_width > 0 AND image_height > 0))
);
CREATE INDEX mission_inputs_mission_created ON mission_inputs(company_id,mission_id,created_at DESC,input_id,revision DESC);
CREATE INDEX mission_inputs_digest ON mission_inputs(company_id,content_digest);

-- +goose Down
DROP TABLE mission_inputs;
