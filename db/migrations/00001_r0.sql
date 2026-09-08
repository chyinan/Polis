-- +goose Up
CREATE TABLE runtime_control (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 incarnation text NOT NULL
);
INSERT INTO runtime_control VALUES (true, 'not-started');
CREATE TABLE companies (
 id text PRIMARY KEY CHECK (id ~ '^[a-zA-Z0-9_-]{1,80}$'),
 company_seq bigint NOT NULL DEFAULT 0 CHECK(company_seq>=0)
);
CREATE TABLE employees (
 company_id text NOT NULL REFERENCES companies(id),
 id text NOT NULL CHECK(id IN ('emp-planning','emp-backend','emp-frontend','emp-review')),
 epoch bigint NOT NULL DEFAULT 1 CHECK(epoch>0),
 PRIMARY KEY(company_id,id)
);
CREATE TABLE missions (
 company_id text NOT NULL REFERENCES companies(id),
 id text NOT NULL CHECK(id ~ '^[a-zA-Z0-9_-]{1,80}$'),
 state text NOT NULL DEFAULT 'draft' CHECK(state IN ('draft','active','paused','succeeded')),
 activation_id text,
 contract text NOT NULL CHECK(contract='r0-arithmetic@1'),
 PRIMARY KEY(company_id,id)
);
CREATE UNIQUE INDEX company_mission_slot ON missions(company_id) WHERE state IN ('active','paused');
CREATE TABLE tasks (
 company_id text NOT NULL,
 id text NOT NULL,
 mission_id text NOT NULL,
 owner text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('bootstrap_plan','compute')),
 state text NOT NULL DEFAULT 'ready' CHECK(state IN ('ready','working','candidate','completed')),
 generation bigint NOT NULL DEFAULT 0 CHECK(generation>=0),
 plan jsonb,
 PRIMARY KEY(company_id,id),
 UNIQUE(company_id,mission_id,kind),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,owner) REFERENCES employees(company_id,id)
);
CREATE UNIQUE INDEX employee_work_slot ON tasks(company_id,owner) WHERE state='working';
CREATE TABLE messages (
 company_id text NOT NULL,
 id text NOT NULL,
 mission_id text NOT NULL,
 task_id text NOT NULL,
 sender text NOT NULL,
 recipient text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('request','response')),
 body text NOT NULL CHECK(octet_length(body)<=4096),
 evidence_id text,
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,sender) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,recipient) REFERENCES employees(company_id,id)
);
CREATE TABLE artifacts (
 company_id text NOT NULL,
 id text NOT NULL,
 task_id text NOT NULL,
 author text NOT NULL,
 digest text NOT NULL CHECK(digest ~ '^[0-9a-f]{64}$'),
 bytes bigint NOT NULL CHECK(bytes BETWEEN 1 AND 4096),
 state text NOT NULL CHECK(state IN ('ready','missing','corrupt')),
 verdict text NOT NULL DEFAULT 'candidate' CHECK(verdict IN ('candidate','passed','failed','invalidated')),
 verifier text,
 contract text NOT NULL CHECK(contract='r0-arithmetic@1'),
 PRIMARY KEY(company_id,id),
 UNIQUE(company_id,task_id),
 CHECK(verdict!='passed' OR (verifier IS NOT NULL AND verifier!=author AND state='ready')),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,author) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,verifier) REFERENCES employees(company_id,id)
);
CREATE TABLE artifact_staging (
 company_id text NOT NULL,
 id text NOT NULL,
 task_id text NOT NULL,
 digest text NOT NULL CHECK(digest ~ '^[0-9a-f]{64}$'),
 PRIMARY KEY(company_id,id),
 UNIQUE(company_id,task_id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id)
);
ALTER TABLE messages ADD FOREIGN KEY(company_id,evidence_id) REFERENCES artifacts(company_id,id);
CREATE TABLE obligations (
 company_id text NOT NULL,
 id text NOT NULL,
 task_id text NOT NULL,
 owner text NOT NULL,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','fulfilled')),
 evidence_id text,
 PRIMARY KEY(company_id,id),
 CHECK((state='fulfilled')=(evidence_id IS NOT NULL)),
 FOREIGN KEY(company_id,id) REFERENCES messages(company_id,id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,owner) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,evidence_id) REFERENCES artifacts(company_id,id)
);
CREATE TABLE receipts (
 company_id text NOT NULL REFERENCES companies(id),
 actor text NOT NULL,
 key text NOT NULL CHECK(octet_length(key) BETWEEN 1 AND 80),
 fingerprint text NOT NULL,
 result jsonb NOT NULL,
 PRIMARY KEY(company_id,actor,key)
);
-- Company sequence is allocated while holding the company guard through commit.
-- These events also form the durable local outbox; no external send path exists.
CREATE TABLE events (
 company_id text NOT NULL REFERENCES companies(id),
 company_seq bigint NOT NULL,
 kind text NOT NULL,
 payload jsonb NOT NULL,
 observed boolean NOT NULL DEFAULT false,
 PRIMARY KEY(company_id,company_seq)
);
CREATE INDEX tasks_mission ON tasks(company_id,mission_id);
CREATE INDEX messages_mission ON messages(company_id,mission_id);
CREATE INDEX obligations_task ON obligations(company_id,task_id);
CREATE INDEX events_pending ON events(company_id,company_seq) WHERE NOT observed;

-- +goose Down
DROP TABLE events, receipts, obligations;
ALTER TABLE messages DROP CONSTRAINT messages_company_id_evidence_id_fkey;
DROP TABLE artifact_staging, artifacts, messages, tasks, missions, employees, companies, runtime_control;
