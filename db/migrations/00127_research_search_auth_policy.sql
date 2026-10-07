-- +goose Up
-- Search auth/ranking metadata contains no secret. Credential material is
-- resolved only by an explicitly injected provider at execution time.
ALTER TABLE research_source_bindings ADD COLUMN search_credential_ref text;
ALTER TABLE research_source_bindings ADD COLUMN search_ranking_revision text;
UPDATE research_source_bindings SET search_ranking_revision='research-ranking@1' WHERE search_endpoint IS NOT NULL;
ALTER TABLE research_source_bindings ADD CONSTRAINT research_source_search_auth_policy_check CHECK (
 (search_endpoint IS NULL AND search_credential_ref IS NULL AND search_ranking_revision IS NULL) OR
 (search_endpoint IS NOT NULL AND (search_credential_ref IS NULL OR search_credential_ref ~ '^[A-Za-z0-9_-]{1,80}$') AND search_ranking_revision='research-ranking@1'));

-- +goose Down
ALTER TABLE research_source_bindings DROP CONSTRAINT research_source_search_auth_policy_check;
ALTER TABLE research_source_bindings DROP COLUMN search_ranking_revision;
ALTER TABLE research_source_bindings DROP COLUMN search_credential_ref;
