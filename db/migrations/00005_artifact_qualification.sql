-- +goose Up
CREATE TABLE artifact_qualifications (
 company_id text NOT NULL,
 artifact_id text NOT NULL,
 checkpoint_id text NOT NULL,
 workspace_revision bigint NOT NULL CHECK(workspace_revision>0),
 workspace_digest text NOT NULL,
 contract_revision_id text NOT NULL,
 acceptance_checker_revision text NOT NULL,
 checkpoint_policy_revision text NOT NULL,
 artifact_eligibility_policy_revision text NOT NULL,
 contract_supersession_policy_revision text NOT NULL,
 PRIMARY KEY(company_id,artifact_id),
 FOREIGN KEY(company_id,artifact_id) REFERENCES artifacts(company_id,id),
 FOREIGN KEY(company_id,checkpoint_id) REFERENCES worker_checkpoints(company_id,id),
 FOREIGN KEY(company_id,contract_revision_id) REFERENCES contract_revisions(company_id,id)
);
-- +goose Down
DROP TABLE artifact_qualifications;
