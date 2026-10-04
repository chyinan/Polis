-- +goose Up
ALTER TABLE task_takeover_leases
 ADD COLUMN base_tree_root_id text,
 ADD COLUMN base_tree_revision bigint,
 ADD COLUMN base_tree_manifest_sha256 text,
 ADD COLUMN base_tree_file_count integer,
 ADD COLUMN base_tree_bytes bigint;

ALTER TABLE task_takeover_leases
 ADD CONSTRAINT task_takeover_tree_binding_shape_check CHECK (
  (base_tree_root_id IS NULL AND base_tree_revision IS NULL AND base_tree_manifest_sha256 IS NULL AND base_tree_file_count IS NULL AND base_tree_bytes IS NULL) OR
  (base_tree_root_id IS NOT NULL AND base_tree_revision IS NOT NULL AND base_tree_revision > 0 AND
   base_tree_manifest_sha256 IS NOT NULL AND base_tree_manifest_sha256 ~ '^[a-f0-9]{64}$' AND
   base_tree_file_count IS NOT NULL AND base_tree_file_count BETWEEN 1 AND 512 AND
   base_tree_bytes IS NOT NULL AND base_tree_bytes BETWEEN 1 AND 16777216)
 ),
 ADD CONSTRAINT task_takeover_tree_binding_root_fk FOREIGN KEY(company_id,base_tree_root_id) REFERENCES worker_workspace_roots(company_id,id);

-- +goose Down
-- A lease is immutable provenance; once a lease pins a tree, dropping this
-- binding would make its original baseline unverifiable.
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM task_takeover_leases WHERE base_tree_root_id IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot roll back pinned Task takeover workspace trees';
 END IF;
END;
$$;
-- +goose StatementEnd
ALTER TABLE task_takeover_leases DROP CONSTRAINT task_takeover_tree_binding_root_fk;
ALTER TABLE task_takeover_leases DROP CONSTRAINT task_takeover_tree_binding_shape_check;
ALTER TABLE task_takeover_leases DROP COLUMN base_tree_bytes,
 DROP COLUMN base_tree_file_count,
 DROP COLUMN base_tree_manifest_sha256,
 DROP COLUMN base_tree_revision,
 DROP COLUMN base_tree_root_id;
