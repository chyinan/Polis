-- +goose Up
-- A fixed Go-test receipt is the verifier for go_patch Artifacts; it is not
-- an Employee identity. Keep the historical independent-Employee rule for
-- every other passed Artifact.
ALTER TABLE artifacts DROP CONSTRAINT artifacts_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_passed_verifier_check CHECK(
 verdict!='passed' OR (
  state='ready' AND (
   (content_format='go_patch' AND verifier IS NULL) OR
   (verifier IS NOT NULL AND verifier!=author)
  )
 )
);

-- Do not let the format-specific verifier exception create a passed artifact
-- without the exact qualified Go Patch ledger row. The deferred check permits
-- the existing publication transaction to insert the Artifact before its
-- replay-verified go_patch_deliveries row.
-- +goose StatementBegin
CREATE FUNCTION enforce_passed_go_patch_delivery() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.content_format='go_patch' AND NEW.verdict='passed' AND NEW.state='ready'
    AND NOT EXISTS (
      SELECT 1 FROM go_patch_deliveries d
      WHERE d.company_id=NEW.company_id AND d.artifact_id=NEW.id
    ) THEN
   RAISE EXCEPTION 'passed Go Patch Artifact has no verified delivery record' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER artifacts_passed_go_patch_delivery_required
 AFTER INSERT OR UPDATE ON artifacts
 DEFERRABLE INITIALLY DEFERRED
 FOR EACH ROW EXECUTE FUNCTION enforce_passed_go_patch_delivery();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM artifacts WHERE content_format='go_patch' AND verdict='passed') THEN
  RAISE EXCEPTION 'cannot remove machine-verifier support while passed Go Patch Artifacts exist';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER artifacts_passed_go_patch_delivery_required ON artifacts;
DROP FUNCTION enforce_passed_go_patch_delivery();
ALTER TABLE artifacts DROP CONSTRAINT artifacts_passed_verifier_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_check CHECK(
 verdict!='passed' OR (verifier IS NOT NULL AND verifier!=author AND state='ready')
);
