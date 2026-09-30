-- +goose Up
ALTER TABLE environment_executor_qualification_events
 ADD COLUMN host_fingerprint_sha256 text CHECK(host_fingerprint_sha256 IS NULL OR host_fingerprint_sha256 ~ '^[a-f0-9]{64}$'),
 ADD COLUMN isolation_policy_sha256 text CHECK(isolation_policy_sha256 IS NULL OR isolation_policy_sha256 ~ '^[a-f0-9]{64}$'),
 ADD COLUMN toolchain_sha256 text CHECK(toolchain_sha256 IS NULL OR toolchain_sha256 ~ '^[a-f0-9]{64}$'),
 ADD COLUMN evidence_input_id text,
 ADD COLUMN evidence_input_revision bigint,
 ADD CONSTRAINT environment_executor_qualification_identity_complete CHECK(
  (host_fingerprint_sha256 IS NULL AND isolation_policy_sha256 IS NULL AND toolchain_sha256 IS NULL AND evidence_input_id IS NULL AND evidence_input_revision IS NULL)
  OR (host_fingerprint_sha256 IS NOT NULL AND isolation_policy_sha256 IS NOT NULL AND toolchain_sha256 IS NOT NULL AND evidence_input_id IS NOT NULL AND evidence_input_revision>0)
 ),
 ADD CONSTRAINT environment_executor_qualification_evidence_input
  FOREIGN KEY(company_id,evidence_input_id,evidence_input_revision)
  REFERENCES mission_inputs(company_id,input_id,revision);

-- +goose Down
ALTER TABLE environment_executor_qualification_events
 DROP CONSTRAINT environment_executor_qualification_evidence_input,
 DROP CONSTRAINT environment_executor_qualification_identity_complete,
 DROP COLUMN evidence_input_revision,
 DROP COLUMN evidence_input_id,
 DROP COLUMN toolchain_sha256,
 DROP COLUMN isolation_policy_sha256,
 DROP COLUMN host_fingerprint_sha256;
