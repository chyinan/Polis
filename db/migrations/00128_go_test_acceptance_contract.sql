-- +goose Up
ALTER TABLE missions DROP CONSTRAINT missions_acceptance_contract_shape;
ALTER TABLE missions ADD CONSTRAINT missions_acceptance_contract_shape CHECK (
 acceptance_contract IS NULL OR CASE
  WHEN jsonb_typeof(acceptance_contract)='object'
   AND jsonb_typeof(acceptance_contract->'required_text')='array'
  THEN CASE WHEN jsonb_array_length(acceptance_contract->'required_text') BETWEEN 1 AND 8
   THEN (acceptance_contract->>'revision'='text-acceptance@1' AND NOT (acceptance_contract ? 'go_test_profile_revision')) OR
        (acceptance_contract->>'revision'='text-acceptance@2' AND acceptance_contract->>'go_test_profile_revision'='polis-go-test-fixture@1')
   ELSE FALSE END
  ELSE FALSE
 END
);

-- The original CHECK was declared inline; its generated name can vary with
-- PostgreSQL's identifier rules. Locate the exact legacy contract by its
-- semantics and fail closed if the expected constraint is not present.
-- +goose StatementBegin
DO $$
DECLARE
 existing_constraint text;
BEGIN
 SELECT conname INTO existing_constraint
 FROM pg_constraint
 WHERE conrelid='task_validation_bindings'::regclass
  AND contype='c'
  AND pg_get_constraintdef(oid) LIKE '%acceptance_revision%'
  AND pg_get_constraintdef(oid) LIKE '%required_text%';
 IF existing_constraint IS NULL THEN
  RAISE EXCEPTION 'legacy task validation contract constraint is missing';
 END IF;
 EXECUTE format('ALTER TABLE task_validation_bindings DROP CONSTRAINT %I', existing_constraint);
END;
$$;
-- +goose StatementEnd

ALTER TABLE task_validation_bindings ADD CONSTRAINT task_validation_bindings_contract_check CHECK (
 jsonb_typeof(contract)='object' AND contract->>'revision'=acceptance_revision AND
 jsonb_typeof(contract->'required_text')='array' AND
 ((acceptance_revision='text-acceptance@1' AND NOT (contract ? 'go_test_profile_revision')) OR
  (acceptance_revision='text-acceptance@2' AND contract->>'go_test_profile_revision'='polis-go-test-fixture@1'))
);

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM missions WHERE acceptance_contract->>'revision'='text-acceptance@2') OR
    EXISTS (SELECT 1 FROM task_validation_bindings WHERE acceptance_revision='text-acceptance@2') THEN
  RAISE EXCEPTION 'cannot remove fixed Go-test acceptance contract while bindings exist';
 END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE task_validation_bindings DROP CONSTRAINT task_validation_bindings_contract_check;
ALTER TABLE task_validation_bindings ADD CONSTRAINT task_validation_bindings_contract_check CHECK (
 jsonb_typeof(contract)='object' AND contract->>'revision'=acceptance_revision AND
 jsonb_typeof(contract->'required_text')='array'
);
ALTER TABLE missions DROP CONSTRAINT missions_acceptance_contract_shape;
ALTER TABLE missions ADD CONSTRAINT missions_acceptance_contract_shape CHECK (
 acceptance_contract IS NULL OR CASE
  WHEN jsonb_typeof(acceptance_contract)='object' AND acceptance_contract->>'revision'='text-acceptance@1'
   AND jsonb_typeof(acceptance_contract->'required_text')='array'
  THEN jsonb_array_length(acceptance_contract->'required_text') BETWEEN 1 AND 8
  ELSE FALSE
 END
);
