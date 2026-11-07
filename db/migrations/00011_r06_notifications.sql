-- +goose Up
CREATE TABLE notification_routes (
 company_id text NOT NULL,
 id text NOT NULL,
 adapter text NOT NULL CHECK(adapter IN ('local','webhook')),
 enabled boolean NOT NULL DEFAULT false,
 destination text NOT NULL CHECK(octet_length(destination)<=512),
 status text NOT NULL DEFAULT 'unverified' CHECK(status IN ('unverified','configured','ready','failed')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,id),
 UNIQUE(company_id,adapter),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);
CREATE TABLE notification_intents (
 company_id text NOT NULL,
 id text NOT NULL,
 event_kind text NOT NULL,
 subject_id text NOT NULL,
 payload jsonb NOT NULL,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','delivered','failed','unknown')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id) REFERENCES companies(id)
);
CREATE TABLE notification_deliveries (
 company_id text NOT NULL,
 id text NOT NULL,
 intent_id text NOT NULL,
 adapter text NOT NULL,
 state text NOT NULL CHECK(state IN ('accepted','delivered','failed','unknown')),
 error_code text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id,intent_id) REFERENCES notification_intents(company_id,id)
);
CREATE INDEX notification_intents_scope ON notification_intents(company_id,created_at DESC);
CREATE INDEX notification_deliveries_scope ON notification_deliveries(company_id,created_at DESC);

-- +goose Down
DROP TABLE notification_deliveries, notification_intents, notification_routes;
