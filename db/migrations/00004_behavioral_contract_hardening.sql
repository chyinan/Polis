-- +goose Up
ALTER TABLE worker_sessions
 ADD COLUMN tool_call_limit bigint NOT NULL DEFAULT 0 CHECK(tool_call_limit>=0),
 ADD COLUMN tool_calls_used bigint NOT NULL DEFAULT 0 CHECK(tool_calls_used>=0),
 ADD CONSTRAINT worker_tool_calls_within_limit CHECK(tool_call_limit=0 OR tool_calls_used<=tool_call_limit);

-- +goose Down
ALTER TABLE worker_sessions DROP CONSTRAINT worker_tool_calls_within_limit;
ALTER TABLE worker_sessions DROP COLUMN tool_calls_used;
ALTER TABLE worker_sessions DROP COLUMN tool_call_limit;
