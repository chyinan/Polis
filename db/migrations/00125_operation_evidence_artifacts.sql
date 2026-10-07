-- +goose Up
-- Operation evidence is an immutable, Task-scoped CAS artifact. It is not a
-- deliverable and never advances a Task or Mission by itself.
ALTER TABLE artifacts DROP CONSTRAINT artifacts_kind_check;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_bytes_check;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_verdict_check;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_workspace_snapshot_binding_check;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_contract_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_kind_check CHECK (artifact_kind IN ('deliverable','workspace_snapshot','operation_evidence'));
ALTER TABLE artifacts ADD CONSTRAINT artifacts_bytes_check CHECK (
 (artifact_kind='deliverable' AND bytes BETWEEN 1 AND 4096) OR
 (artifact_kind='workspace_snapshot' AND bytes BETWEEN 1 AND 8388608) OR
 (artifact_kind='operation_evidence' AND bytes BETWEEN 1 AND 8388608));
ALTER TABLE artifacts ADD CONSTRAINT artifacts_verdict_check CHECK(verdict IN ('candidate','passed','failed','invalidated','draft_not_accepted'));
ALTER TABLE artifacts ADD CONSTRAINT artifacts_contract_check CHECK(contract IN ('r0-arithmetic@1','operation-evidence@1'));
ALTER TABLE artifacts ADD CONSTRAINT artifacts_workspace_snapshot_binding_check CHECK (
 (artifact_kind='deliverable' AND workspace_id IS NULL AND workspace_revision IS NULL AND verdict!='draft_not_accepted') OR
 (artifact_kind='workspace_snapshot' AND workspace_id IS NOT NULL AND workspace_revision IS NOT NULL AND workspace_revision>0 AND verdict='draft_not_accepted') OR
 (artifact_kind='operation_evidence' AND workspace_id IS NULL AND workspace_revision IS NULL AND verdict IN ('candidate','passed')));

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM artifacts WHERE artifact_kind='operation_evidence') THEN
  RAISE EXCEPTION 'cannot discard operation evidence artifacts';
 END IF;
END
$$;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_workspace_snapshot_binding_check;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_contract_check;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_verdict_check;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_bytes_check;
ALTER TABLE artifacts DROP CONSTRAINT artifacts_kind_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_kind_check CHECK (artifact_kind IN ('deliverable','workspace_snapshot'));
ALTER TABLE artifacts ADD CONSTRAINT artifacts_bytes_check CHECK (
 (artifact_kind='deliverable' AND bytes BETWEEN 1 AND 4096) OR
 (artifact_kind='workspace_snapshot' AND bytes BETWEEN 1 AND 8388608));
ALTER TABLE artifacts ADD CONSTRAINT artifacts_verdict_check CHECK(verdict IN ('candidate','passed','failed','invalidated','draft_not_accepted'));
ALTER TABLE artifacts ADD CONSTRAINT artifacts_contract_check CHECK(contract='r0-arithmetic@1');
ALTER TABLE artifacts ADD CONSTRAINT artifacts_workspace_snapshot_binding_check CHECK (
 (artifact_kind='deliverable' AND workspace_id IS NULL AND workspace_revision IS NULL AND verdict!='draft_not_accepted') OR
 (artifact_kind='workspace_snapshot' AND workspace_id IS NOT NULL AND workspace_revision IS NOT NULL AND workspace_revision>0 AND verdict='draft_not_accepted'));
