-- +goose Up
-- R0.15: explicit call qualification and auditable tools.call receipts.
-- Probe/descriptor read paths do not authorize execution. Stdio-first.

CREATE TABLE mcp_tool_call_qualifications (
 company_id text NOT NULL,
 id text NOT NULL,
 binding_id text NOT NULL,
 definition_id text NOT NULL,
 server_id text NOT NULL,
 tool_name text NOT NULL,
 handle text NOT NULL, -- serverId:toolName
 descriptor_digest text NOT NULL,
 policy_revision text NOT NULL,
 effect_class text NOT NULL DEFAULT 'unknown' CHECK (effect_class IN ('read','idempotent','effectful','unknown')),
 status text NOT NULL CHECK (status IN ('draft','approved','disabled','revoked','expired')),
 authorization_ref text NOT NULL DEFAULT '',
 transport text NOT NULL DEFAULT 'stdio' CHECK (transport IN ('stdio')),
 qualified_by text NOT NULL DEFAULT '',
 qualified_at timestamptz,
 expires_at timestamptz,
 notes text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (company_id, id),
 FOREIGN KEY (company_id) REFERENCES companies(id),
 FOREIGN KEY (company_id, binding_id) REFERENCES employee_capability_bindings(company_id, id),
 FOREIGN KEY (company_id, definition_id) REFERENCES mcp_server_definitions(company_id, id),
 CHECK (handle = server_id || ':' || tool_name),
 CHECK (tool_name <> '' AND position(':' in tool_name) = 0 AND position('/' in tool_name) = 0)
);

CREATE INDEX mcp_tool_call_qual_lookup
 ON mcp_tool_call_qualifications (company_id, binding_id, definition_id, handle, descriptor_digest, status);
CREATE INDEX mcp_tool_call_qual_binding
 ON mcp_tool_call_qualifications (company_id, binding_id, updated_at DESC);

CREATE TABLE mcp_tool_call_receipts (
 company_id text NOT NULL,
 id text NOT NULL,
 employee_id text NOT NULL,
 task_id text,
 session_id text,
 session_epoch bigint,
 session_incarnation text NOT NULL DEFAULT '',
 binding_id text NOT NULL,
 definition_id text NOT NULL,
 qualification_id text,
 handle text NOT NULL,
 descriptor_digest text NOT NULL,
 arguments_digest text NOT NULL,
 request_digest text NOT NULL,
 idempotency_key text NOT NULL,
 policy_revision text NOT NULL,
 authorization_ref text NOT NULL DEFAULT '',
 effect_class text NOT NULL DEFAULT 'unknown',
 outcome text NOT NULL CHECK (outcome IN ('denied','completed','failed','timeout','cancelled','unknown')),
 code text NOT NULL DEFAULT '',
 result_digest text NOT NULL DEFAULT '',
 result_json jsonb,
 error_json jsonb,
 process_observed jsonb NOT NULL DEFAULT '{}'::jsonb,
 egress_observed jsonb NOT NULL DEFAULT '{}'::jsonb,
 started_at timestamptz,
 finished_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (company_id, id),
 FOREIGN KEY (company_id) REFERENCES companies(id),
 FOREIGN KEY (company_id, employee_id) REFERENCES employees(company_id, id),
 FOREIGN KEY (company_id, binding_id) REFERENCES employee_capability_bindings(company_id, id),
 FOREIGN KEY (company_id, definition_id) REFERENCES mcp_server_definitions(company_id, id),
 FOREIGN KEY (company_id, task_id) REFERENCES tasks(company_id, id)
);

CREATE UNIQUE INDEX mcp_tool_call_receipt_idem
 ON mcp_tool_call_receipts (company_id, employee_id, handle, idempotency_key);
CREATE INDEX mcp_tool_call_receipts_target
 ON mcp_tool_call_receipts (company_id, binding_id, handle, created_at DESC);
CREATE INDEX mcp_tool_call_receipts_task
 ON mcp_tool_call_receipts (company_id, task_id, created_at DESC)
 WHERE task_id IS NOT NULL;

-- +goose Down
DROP TABLE mcp_tool_call_receipts;
DROP TABLE mcp_tool_call_qualifications;
