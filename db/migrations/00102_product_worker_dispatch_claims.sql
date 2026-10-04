-- +goose Up
CREATE TABLE product_worker_dispatch_state (
 id smallint PRIMARY KEY CHECK(id=1),
 cursor_company_id text NOT NULL DEFAULT '',
 updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO product_worker_dispatch_state(id,cursor_company_id) VALUES(1,'');

-- Claims coordinate independently running opt-in dispatchers between
-- candidate selection and the Worker admission transaction. Expiry recovers
-- claims left behind by a process exit.
CREATE TABLE product_worker_dispatch_claims (
 claim_id text PRIMARY KEY CHECK(claim_id ~ '^[a-f0-9]{32}$'),
 company_id text NOT NULL,
 task_id text NOT NULL,
 claimed_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 UNIQUE(company_id,task_id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 CHECK(expires_at>claimed_at)
);
CREATE INDEX product_worker_dispatch_claims_expiry ON product_worker_dispatch_claims(expires_at);

-- +goose Down
-- Dispatch cursor and claims coordinate multiple runtimes; resetting them
-- during downgrade could admit the same Task twice.
-- +goose StatementBegin
DO $$
BEGIN
 RAISE EXCEPTION 'Product Worker dispatch coordination is forward-only';
END
$$;
-- +goose StatementEnd
