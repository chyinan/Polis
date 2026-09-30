-- +goose Up
CREATE TABLE human_interventions (
 company_id text NOT NULL,
 id text NOT NULL,
 mission_id text,
 task_id text,
 problem_key text NOT NULL CHECK(octet_length(problem_key) BETWEEN 1 AND 160),
 occurrence bigint NOT NULL CHECK(occurrence > 0),
 severity text NOT NULL CHECK(severity IN ('high','critical')),
 reason_code text NOT NULL CHECK(reason_code IN ('credential_unavailable','provider_quota_exhausted','external_outcome_unknown','capability_revoked','handover_required','environment_not_ready')),
 affected_scope text NOT NULL CHECK(octet_length(affected_scope) BETWEEN 1 AND 160),
 protection_action text NOT NULL CHECK(protection_action IN ('execution_paused','old_writer_fenced','readonly_mode','no_mutation')),
 required_action text NOT NULL CHECK(required_action IN ('reauthorize_in_workbench','review_unknown_outcome','resume_or_cancel','supply_missing_input')),
 evidence_refs jsonb NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(evidence_refs)='array'),
 state text NOT NULL DEFAULT 'open' CHECK(state IN ('open','acknowledged','resolved','obsolete')),
 state_revision bigint NOT NULL DEFAULT 1 CHECK(state_revision > 0),
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,id),
 UNIQUE(company_id,problem_key,occurrence),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 FOREIGN KEY(company_id,mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,task_id) REFERENCES tasks(company_id,id)
);
CREATE INDEX human_interventions_open_scope ON human_interventions(company_id,state,created_at DESC);

ALTER TABLE notification_routes DROP CONSTRAINT notification_routes_adapter_check;
ALTER TABLE notification_routes ADD CONSTRAINT notification_routes_adapter_check CHECK(adapter IN ('local','webhook','qq_official'));
ALTER TABLE notification_routes
 ADD COLUMN route_revision bigint NOT NULL DEFAULT 1 CHECK(route_revision > 0),
 ADD COLUMN qualification_status text NOT NULL DEFAULT 'unverified' CHECK(qualification_status IN ('unverified','qualified','expired','revoked','failed')),
 ADD COLUMN qualified_until timestamptz,
 ADD COLUMN safety_alias text NOT NULL DEFAULT '' CHECK(octet_length(safety_alias)<=160),
 ADD COLUMN credential_ref text NOT NULL DEFAULT '' CHECK(octet_length(credential_ref)<=128);

CREATE TABLE qq_channel_qualifications (
 company_id text NOT NULL,
 route_id text NOT NULL,
 route_revision bigint NOT NULL CHECK(route_revision > 0),
 target_openid text NOT NULL CHECK(octet_length(target_openid) BETWEEN 1 AND 128),
 account_fingerprint text NOT NULL CHECK(account_fingerprint ~ '^[a-f0-9]{64}$'),
 qualification_status text NOT NULL CHECK(qualification_status IN ('qualified','expired','failed')),
 evidence_digest text NOT NULL CHECK(evidence_digest ~ '^[a-f0-9]{64}$'),
 qualified_until timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,route_id,route_revision),
 FOREIGN KEY(company_id,route_id) REFERENCES notification_routes(company_id,id)
);
CREATE TRIGGER qq_channel_qualifications_immutable
 BEFORE UPDATE OR DELETE ON qq_channel_qualifications
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();

ALTER TABLE notification_intents DROP CONSTRAINT notification_intents_state_check;
ALTER TABLE notification_intents ADD CONSTRAINT notification_intents_state_check
 CHECK(state IN ('pending','delivered','failed','unknown','outcome_unknown','exhausted','superseded'));
ALTER TABLE notification_intents
 ADD COLUMN intervention_id text,
 ADD COLUMN route_id text,
 ADD COLUMN route_revision bigint CHECK(route_revision IS NULL OR route_revision > 0),
 ADD COLUMN dedupe_key text,
 ADD COLUMN message_digest text CHECK(message_digest IS NULL OR message_digest ~ '^[a-f0-9]{64}$'),
 ADD COLUMN expires_at timestamptz;
ALTER TABLE notification_intents ADD CONSTRAINT notification_intents_intervention_fk
 FOREIGN KEY(company_id,intervention_id) REFERENCES human_interventions(company_id,id);
ALTER TABLE notification_intents ADD CONSTRAINT notification_intents_route_fk
 FOREIGN KEY(company_id,route_id) REFERENCES notification_routes(company_id,id);
CREATE UNIQUE INDEX notification_intents_dedupe ON notification_intents(company_id,dedupe_key) WHERE dedupe_key IS NOT NULL;

ALTER TABLE notification_deliveries DROP CONSTRAINT notification_deliveries_state_check;
ALTER TABLE notification_deliveries ADD CONSTRAINT notification_deliveries_state_check
 CHECK(state IN ('accepted','delivered','failed','unknown','pending','sending','provider_accepted','retry_wait','rejected','outcome_unknown','exhausted','superseded'));
ALTER TABLE notification_deliveries
 ADD COLUMN route_revision bigint CHECK(route_revision IS NULL OR route_revision > 0),
 ADD COLUMN attempt_count integer NOT NULL DEFAULT 0 CHECK(attempt_count >= 0),
 ADD COLUMN retry_at timestamptz,
 ADD COLUMN expires_at timestamptz,
 ADD COLUMN message_digest text CHECK(message_digest IS NULL OR message_digest ~ '^[a-f0-9]{64}$'),
 ADD COLUMN remote_message_id text,
 ADD COLUMN sender_epoch text,
 ADD COLUMN permit_id text,
 ADD COLUMN provider_http_status integer CHECK(provider_http_status IS NULL OR provider_http_status BETWEEN 100 AND 599);

-- +goose Down
ALTER TABLE notification_deliveries DROP COLUMN provider_http_status,DROP COLUMN permit_id,DROP COLUMN sender_epoch,DROP COLUMN remote_message_id,DROP COLUMN message_digest,DROP COLUMN expires_at,DROP COLUMN retry_at,DROP COLUMN attempt_count,DROP COLUMN route_revision;
ALTER TABLE notification_deliveries DROP CONSTRAINT notification_deliveries_state_check;
ALTER TABLE notification_deliveries ADD CONSTRAINT notification_deliveries_state_check CHECK(state IN ('accepted','delivered','failed','unknown'));
DROP INDEX notification_intents_dedupe;
ALTER TABLE notification_intents DROP CONSTRAINT notification_intents_route_fk,DROP CONSTRAINT notification_intents_intervention_fk;
ALTER TABLE notification_intents DROP COLUMN expires_at,DROP COLUMN message_digest,DROP COLUMN dedupe_key,DROP COLUMN route_revision,DROP COLUMN route_id,DROP COLUMN intervention_id;
ALTER TABLE notification_intents DROP CONSTRAINT notification_intents_state_check;
ALTER TABLE notification_intents ADD CONSTRAINT notification_intents_state_check CHECK(state IN ('pending','delivered','failed','unknown'));
DROP TRIGGER qq_channel_qualifications_immutable ON qq_channel_qualifications;
DROP TABLE qq_channel_qualifications;
ALTER TABLE notification_routes DROP COLUMN credential_ref,DROP COLUMN safety_alias,DROP COLUMN qualified_until,DROP COLUMN qualification_status,DROP COLUMN route_revision;
ALTER TABLE notification_routes DROP CONSTRAINT notification_routes_adapter_check;
ALTER TABLE notification_routes ADD CONSTRAINT notification_routes_adapter_check CHECK(adapter IN ('local','webhook'));
DROP INDEX human_interventions_open_scope;
DROP TABLE human_interventions;
