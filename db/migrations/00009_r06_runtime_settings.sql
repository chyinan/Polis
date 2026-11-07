-- +goose Up
ALTER TABLE companies
 ADD COLUMN provider text NOT NULL DEFAULT 'deterministic',
 ADD COLUMN model text NOT NULL DEFAULT 'deterministic/fake',
 ADD COLUMN effort text NOT NULL DEFAULT 'bounded',
 ADD COLUMN profile text NOT NULL DEFAULT 'deterministic/fake';
ALTER TABLE companies ADD CONSTRAINT companies_provider_nonempty CHECK(btrim(provider)!='');
ALTER TABLE companies ADD CONSTRAINT companies_model_nonempty CHECK(btrim(model)!='');
ALTER TABLE companies ADD CONSTRAINT companies_effort_nonempty CHECK(btrim(effort)!='');
ALTER TABLE companies ADD CONSTRAINT companies_profile_nonempty CHECK(btrim(profile)!='');

-- +goose Down
ALTER TABLE companies DROP CONSTRAINT companies_profile_nonempty;
ALTER TABLE companies DROP CONSTRAINT companies_effort_nonempty;
ALTER TABLE companies DROP CONSTRAINT companies_model_nonempty;
ALTER TABLE companies DROP CONSTRAINT companies_provider_nonempty;
ALTER TABLE companies DROP COLUMN profile, DROP COLUMN effort, DROP COLUMN model, DROP COLUMN provider;
