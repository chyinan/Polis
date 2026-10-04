-- +goose Up
-- Installation-owner review can acknowledge an incomplete legacy inventory,
-- but it cannot turn that inventory into an exact snapshot or prove quiescence.
CREATE TABLE capability_revocation_owner_reviews (
 company_id text NOT NULL REFERENCES companies(id),
 revocation_id text NOT NULL,
 scope text NOT NULL CHECK(scope IN ('capability','employee')),
 capability_kind text NOT NULL CHECK(capability_kind IN ('skill','mcp')),
 capability_id text NOT NULL,
 version_digest text NOT NULL CHECK(version_digest ~ '^[a-f0-9]{64}$'),
 qualification_id text NOT NULL,
 employee_id text NOT NULL,
 disposition text NOT NULL CHECK(disposition='acknowledged_unresolved'),
 rationale text NOT NULL CHECK(length(btrim(rationale)) BETWEEN 1 AND 512),
 actor text NOT NULL CHECK(actor='installation-owner'),
 request_id text NOT NULL,
 reviewed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,revocation_id),
 UNIQUE(company_id,request_id),
 CHECK((scope='capability' AND employee_id='') OR (scope='employee' AND employee_id<>''))
);
CREATE TRIGGER capability_revocation_owner_reviews_immutable
 BEFORE UPDATE OR DELETE ON capability_revocation_owner_reviews
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER capability_revocation_owner_reviews_no_truncate
 BEFORE TRUNCATE ON capability_revocation_owner_reviews
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM capability_revocation_owner_reviews) THEN
  RAISE EXCEPTION 'cannot remove capability revocation owner-review history';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER capability_revocation_owner_reviews_no_truncate ON capability_revocation_owner_reviews;
DROP TRIGGER capability_revocation_owner_reviews_immutable ON capability_revocation_owner_reviews;
DROP TABLE capability_revocation_owner_reviews;
