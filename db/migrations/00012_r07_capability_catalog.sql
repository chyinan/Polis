-- +goose Up
-- Capability catalog foundation. Records are candidates only; importing a
-- Skill or MCP definition never executes scripts, starts a service, or sends
-- external traffic.
CREATE TABLE skill_revisions (
 company_id text NOT NULL,
 id text NOT NULL,
 publisher_scope text NOT NULL CHECK(publisher_scope IN ('group','company')),
 package_id text NOT NULL,
 revision text NOT NULL,
 display_name text NOT NULL CHECK(octet_length(display_name) <= 160),
 source_ref text NOT NULL CHECK(octet_length(source_ref) <= 512),
 content_digest text NOT NULL CHECK(octet_length(content_digest) <= 128),
 manifest jsonb NOT NULL DEFAULT '{}'::jsonb,
 status text NOT NULL DEFAULT 'candidate' CHECK(status IN ('candidate','approved','revoked')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 UNIQUE(company_id,publisher_scope,package_id,revision)
);
CREATE INDEX skill_revisions_scope ON skill_revisions(company_id,status,created_at DESC);

CREATE TABLE mcp_server_definitions (
 company_id text NOT NULL,
 id text NOT NULL,
 name text NOT NULL CHECK(octet_length(name) <= 160),
 transport text NOT NULL CHECK(transport IN ('stdio','streamable_http')),
 endpoint text CHECK(endpoint IS NULL OR octet_length(endpoint) <= 512),
 command text CHECK(command IS NULL OR octet_length(command) <= 512),
 args jsonb NOT NULL DEFAULT '[]'::jsonb,
 descriptor_digest text NOT NULL CHECK(octet_length(descriptor_digest) <= 128),
 status text NOT NULL DEFAULT 'unverified' CHECK(status IN ('unverified','candidate','approved','revoked')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);
CREATE INDEX mcp_server_definitions_scope ON mcp_server_definitions(company_id,status,created_at DESC);

-- +goose Down
DROP TABLE mcp_server_definitions, skill_revisions;
