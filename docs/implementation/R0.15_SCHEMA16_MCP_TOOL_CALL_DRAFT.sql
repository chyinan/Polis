-- DRAFT ONLY — NOT AN ACTIVE MIGRATION. DO NOT PLACE IN db/migrations/ OR APPLY.
-- Planning artifact for R0.15 (`docs/design-plans/R0_15_MCP_TOOL_CALL_QUALIFICATION_GATE.md`).
-- No execution/egress is authorized by this draft. Any real migration/application
-- requires a separately authorized implementation slice, final review, active
-- migration numbering, and `migrateVersion` gate update.
--
-- Target (if adopted): schema 16, additive.
-- Dependencies (existing, schema <= 15): companies, employees, tasks,
-- employee_capability_bindings, mcp_server_definitions, skill_loads context.

-- +goose Up (DRAFT — do not execute)

-- Explicit operator/policy qualification for a future tools.call. This is NOT
-- satisfied by R0.10 probe receipts or R0.14 descriptor reads. Rows are scoped
-- to company+binding+definition+tool handle+descriptor digest+policy revision.
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

-- Auditable tools.call receipts. Idempotency is scoped by request_digest; a
-- same key with a different request_digest must be rejected as conflict.
CREATE TABLE mcp_tool_call_receipts (
 company_id text NOT NULL,
 id text NOT NULL,
 employee_id text NOT NULL,
 task_id text, -- nullable if a future non-task control path is explicitly designed
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
 code text NOT NULL DEFAULT '', -- structured denial/error code, empty on completed
 result_digest text NOT NULL DEFAULT '',
 result_json jsonb, -- bounded/redacted; never secrets
 error_json jsonb,  -- bounded/redacted
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

-- +goose Down (DRAFT — do not execute)
-- DROP TABLE mcp_tool_call_receipts;
-- DROP TABLE mcp_tool_call_qualifications;
