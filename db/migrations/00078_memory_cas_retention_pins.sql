-- CAS objects referenced by memory history must outlive the rows that first
-- introduced them. Pins are append-only; collectors must serialize on the
-- company row before checking references and deleting a CAS object.
CREATE TABLE memory_cas_retention_pins (
 company_id text NOT NULL,
 reference_kind text NOT NULL CHECK(reference_kind IN (
  'memory_revision_source','memory_correction_source','memory_dependency_target','memory_revalidation_workspace'
 )),
 reference_id text NOT NULL,
 reference_revision bigint NOT NULL CHECK(reference_revision>0),
 object_kind text NOT NULL CHECK(object_kind IN ('mission_input','artifact','worker_workspace')),
 object_id text NOT NULL,
 object_revision bigint NOT NULL CHECK(object_revision>0),
 digest text NOT NULL CHECK(digest ~ '^[0-9a-f]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,reference_kind,reference_id,reference_revision),
 FOREIGN KEY(company_id) REFERENCES companies(id),
 CHECK((reference_kind IN ('memory_revision_source','memory_correction_source','memory_dependency_target') AND object_kind IN ('mission_input','artifact')) OR
       (reference_kind='memory_revalidation_workspace' AND object_kind='worker_workspace')),
 CHECK(object_kind!='artifact' OR object_revision=1)
);
CREATE INDEX memory_cas_retention_pins_digest ON memory_cas_retention_pins(company_id,digest);
CREATE TRIGGER memory_cas_retention_pins_immutable
 BEFORE UPDATE OR DELETE ON memory_cas_retention_pins
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER memory_cas_retention_pins_no_truncate
 BEFORE TRUNCATE ON memory_cas_retention_pins
 FOR EACH STATEMENT EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- Preserve every durable memory reference, including rejected correction
-- evidence and historical Task workspaces. A single owner has one immutable
-- CAS target, enforced by the primary key above.
INSERT INTO memory_cas_retention_pins(company_id,reference_kind,reference_id,reference_revision,object_kind,object_id,object_revision,digest)
SELECT company_id,'memory_revision_source',record_id,revision,source_kind,source_id,source_revision,source_sha256
FROM memory_record_revisions WHERE source_kind IS NOT NULL;
INSERT INTO memory_cas_retention_pins(company_id,reference_kind,reference_id,reference_revision,object_kind,object_id,object_revision,digest)
SELECT company_id,'memory_correction_source',correction_id,1,source_kind,source_id,source_revision,source_sha256
FROM memory_correction_requests;
INSERT INTO memory_cas_retention_pins(company_id,reference_kind,reference_id,reference_revision,object_kind,object_id,object_revision,digest)
SELECT company_id,'memory_dependency_target',dependency_id,1,target_kind,target_id,target_revision,target_sha256
FROM memory_dependencies WHERE target_kind IN ('mission_input','artifact');
INSERT INTO memory_cas_retention_pins(company_id,reference_kind,reference_id,reference_revision,object_kind,object_id,object_revision,digest)
SELECT company_id,'memory_revalidation_workspace',revalidation_id,1,'worker_workspace',task_id,workspace_revision,workspace_digest
FROM memory_task_revalidation_events;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM memory_cas_retention_pins) THEN
  RAISE EXCEPTION 'cannot roll back memory CAS retention pins';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER memory_cas_retention_pins_no_truncate ON memory_cas_retention_pins;
DROP TRIGGER memory_cas_retention_pins_immutable ON memory_cas_retention_pins;
DROP INDEX memory_cas_retention_pins_digest;
DROP TABLE memory_cas_retention_pins;
