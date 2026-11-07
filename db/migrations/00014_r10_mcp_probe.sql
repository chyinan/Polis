-- +goose Up
-- A stdio probe records the observed tool catalog. It does not approve
-- calls, and the probe itself only performs initialize and tools/list.
ALTER TABLE mcp_server_definitions
 ADD COLUMN qualification jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE mcp_server_definitions DROP COLUMN qualification;
