-- +goose Up
-- Schema 125 accidentally removed the peer contract introduced by schema 3.
-- Restore that existing contract without changing Artifact kind/qualification.
ALTER TABLE artifacts DROP CONSTRAINT artifacts_contract_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_contract_check
 CHECK(contract IN ('r0-arithmetic@1','signed-zero@1','r03-api@1','operation-evidence@1'));

-- +goose Down
ALTER TABLE artifacts DROP CONSTRAINT artifacts_contract_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_contract_check
 CHECK(contract IN ('r0-arithmetic@1','operation-evidence@1'));
