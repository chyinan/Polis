-- +goose Up
ALTER TABLE missions DROP CONSTRAINT missions_contract_check;
ALTER TABLE missions ADD CHECK(contract IN ('r0-arithmetic@1','signed-zero@1','r03-api@1'));
ALTER TABLE tasks DROP CONSTRAINT tasks_kind_check;
ALTER TABLE tasks ADD CHECK(kind IN ('bootstrap_plan','compute','compat','review','peer_backend','peer_frontend','peer_review'));
ALTER TABLE messages DROP CONSTRAINT messages_kind_check;
ALTER TABLE messages ADD CHECK(kind IN ('request','response','fyi','contract_proposal','ack'));
ALTER TABLE messages ADD COLUMN delivery_state text NOT NULL DEFAULT 'persisted' CHECK(delivery_state IN ('persisted','delivered','observed','acknowledged','applied','resolved','superseded'));
ALTER TABLE messages ADD COLUMN contract_revision_id text;
ALTER TABLE messages ADD COLUMN task_revision bigint NOT NULL DEFAULT 0 CHECK(task_revision>=0);
ALTER TABLE obligations DROP CONSTRAINT obligations_state_check;
ALTER TABLE obligations ADD CHECK(state IN ('pending','observed','applied','fulfilled','declined','superseded'));
ALTER TABLE obligations ADD COLUMN evidence_ref text;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_contract_check;
ALTER TABLE artifacts ADD CHECK(contract IN ('r0-arithmetic@1','signed-zero@1','r03-api@1'));

CREATE TABLE contract_revisions (
 company_id text NOT NULL,
 id text NOT NULL,
 mission_id text NOT NULL,
 revision bigint NOT NULL CHECK(revision>0),
 base_revision bigint,
 endpoint text NOT NULL,
 schema jsonb NOT NULL,
 digest text NOT NULL CHECK(digest ~ '^[0-9a-f]{64}$'),
 state text NOT NULL CHECK(state IN ('proposed','accepted','superseded')),
 proposer text NOT NULL,
 accepter text,
 PRIMARY KEY(company_id,id),
 UNIQUE(company_id,mission_id,revision),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,proposer) REFERENCES employees(company_id,id),
 FOREIGN KEY(company_id,accepter) REFERENCES employees(company_id,id)
);
ALTER TABLE messages ADD FOREIGN KEY(company_id,contract_revision_id) REFERENCES contract_revisions(company_id,id);

CREATE TABLE peer_work_signals (
 company_id text NOT NULL,
 id text NOT NULL,
 message_id text NOT NULL,
 obligation_id text NOT NULL,
 recipient text NOT NULL,
 state text NOT NULL CHECK(state IN ('pending','observed','acknowledged','applied','resolved','superseded')),
 PRIMARY KEY(company_id,id),
 UNIQUE(company_id,message_id),
 FOREIGN KEY(company_id,message_id) REFERENCES messages(company_id,id),
 FOREIGN KEY(company_id,obligation_id) REFERENCES obligations(company_id,id),
 FOREIGN KEY(company_id,recipient) REFERENCES employees(company_id,id)
);

CREATE TABLE task_revisions (
 company_id text NOT NULL,
 id text NOT NULL,
 task_id text NOT NULL,
 revision bigint NOT NULL CHECK(revision>0),
 digest text NOT NULL CHECK(digest ~ '^[0-9a-f]{64}$'),
 contract_revision_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('working','candidate','fixed','superseded')),
 PRIMARY KEY(company_id,id),
 UNIQUE(company_id,task_id,revision),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,contract_revision_id) REFERENCES contract_revisions(company_id,id)
);

CREATE TABLE integration_candidates (
 company_id text NOT NULL,
 id text NOT NULL,
 mission_id text NOT NULL,
 backend_artifact_id text NOT NULL,
 frontend_artifact_id text NOT NULL,
 contract_revision_id text NOT NULL,
 base_revision text NOT NULL,
 verifier_revision text NOT NULL,
 state text NOT NULL CHECK(state IN ('candidate','passed','failed')),
 PRIMARY KEY(company_id,id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,backend_artifact_id) REFERENCES artifacts(company_id,id),
 FOREIGN KEY(company_id,frontend_artifact_id) REFERENCES artifacts(company_id,id),
 FOREIGN KEY(company_id,contract_revision_id) REFERENCES contract_revisions(company_id,id)
);
CREATE TABLE review_records (
 company_id text NOT NULL,
 id text NOT NULL,
 task_id text NOT NULL,
 integration_id text NOT NULL,
 verdict text NOT NULL CHECK(verdict IN ('passed','failed','inconclusive')),
 findings jsonb NOT NULL,
 evidence jsonb NOT NULL,
 confidence text NOT NULL CHECK(confidence IN ('low','medium','high')),
 limitations jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(company_id,id),
 UNIQUE(company_id,task_id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id),
 FOREIGN KEY(company_id,integration_id) REFERENCES integration_candidates(company_id,id)
);
CREATE INDEX peer_messages_recipient ON messages(company_id,recipient,delivery_state);
CREATE INDEX peer_obligations_owner ON obligations(company_id,owner,state);

-- +goose Down
DROP TABLE review_records,integration_candidates,task_revisions,peer_work_signals;
ALTER TABLE messages DROP CONSTRAINT messages_company_id_contract_revision_id_fkey;
DROP TABLE contract_revisions;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_contract_check;
ALTER TABLE artifacts ADD CHECK(contract IN ('r0-arithmetic@1','signed-zero@1'));
ALTER TABLE obligations DROP COLUMN evidence_ref;
ALTER TABLE obligations DROP CONSTRAINT obligations_state_check;
ALTER TABLE obligations ADD CHECK(state IN ('pending','fulfilled'));
ALTER TABLE messages DROP COLUMN task_revision;
ALTER TABLE messages DROP COLUMN contract_revision_id;
ALTER TABLE messages DROP COLUMN delivery_state;
ALTER TABLE messages DROP CONSTRAINT messages_kind_check;
ALTER TABLE messages ADD CHECK(kind IN ('request','response'));
ALTER TABLE tasks DROP CONSTRAINT tasks_kind_check;
ALTER TABLE tasks ADD CHECK(kind IN ('bootstrap_plan','compute','compat','review'));
ALTER TABLE missions DROP CONSTRAINT missions_contract_check;
ALTER TABLE missions ADD CHECK(contract IN ('r0-arithmetic@1','signed-zero@1'));
