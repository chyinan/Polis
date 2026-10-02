-- CAS writers reserve the expected digest before publishing bytes. This keeps
-- the collection path safe across the existing CAS-first, DB-reference-second
-- operations, including calls that return a digest before a later event write.
CREATE TABLE cas_blob_write_claims (
 company_id text NOT NULL,
 digest text NOT NULL CHECK(digest ~ '^[0-9a-f]{64}$'),
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,digest),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 CHECK(expires_at>created_at)
);
CREATE INDEX cas_blob_write_claims_expiry ON cas_blob_write_claims(company_id,expires_at,digest);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 DELETE FROM cas_blob_write_claims WHERE expires_at<=clock_timestamp();
 IF EXISTS(SELECT 1 FROM cas_blob_write_claims) THEN
  RAISE EXCEPTION 'cannot roll back active CAS write claims';
 END IF;
END $$;
-- +goose StatementEnd
DROP INDEX cas_blob_write_claims_expiry;
DROP TABLE cas_blob_write_claims;
