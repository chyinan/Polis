-- +goose Up
ALTER TABLE task_takeover_lease_events DROP CONSTRAINT task_takeover_lease_events_snapshot_bytes_check;
ALTER TABLE task_takeover_lease_events ADD CONSTRAINT task_takeover_lease_events_snapshot_bytes_check
 CHECK(snapshot_bytes IS NULL OR snapshot_bytes BETWEEN 1 AND 8388608);

-- +goose Down
-- Preserve return receipts whose canonical directory archive exceeds the
-- original single-text-file receipt bound.
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM task_takeover_lease_events WHERE snapshot_bytes > 4096) THEN
  RAISE EXCEPTION 'cannot roll back Task takeover directory snapshot receipts';
 END IF;
END;
$$;
-- +goose StatementEnd
ALTER TABLE task_takeover_lease_events DROP CONSTRAINT task_takeover_lease_events_snapshot_bytes_check;
ALTER TABLE task_takeover_lease_events ADD CONSTRAINT task_takeover_lease_events_snapshot_bytes_check
 CHECK(snapshot_bytes IS NULL OR snapshot_bytes BETWEEN 1 AND 4096);
