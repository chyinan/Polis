-- +goose Up
-- Employee capability bindings reference approved Skill revisions and
-- registered MCP definitions. Creating or approving a binding does not
-- execute scripts, start a process, or open a network connection.
CREATE TABLE employee_capability_bindings (
 company_id text NOT NULL,
 id text NOT NULL,
 employee_id text NOT NULL,
 status text NOT NULL CHECK(status IN ('draft','approved','disabled','revoked')),
 skill_revision_ids jsonb NOT NULL,
 mcp_definition_ids jsonb NOT NULL,
 qualification jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 FOREIGN KEY(company_id,employee_id) REFERENCES employees(company_id,id)
);
CREATE UNIQUE INDEX employee_capability_binding_live
 ON employee_capability_bindings(company_id, employee_id)
 WHERE status IN ('draft','approved','disabled');

-- +goose Down
DROP TABLE employee_capability_bindings;
