-- +goose Up
ALTER TABLE companies
 ADD COLUMN name text NOT NULL DEFAULT '',
 ADD COLUMN workspace_root text NOT NULL DEFAULT '.',
 ADD COLUMN state text NOT NULL DEFAULT 'active' CHECK(state IN ('active','archived'));
UPDATE companies SET name=id WHERE name='';
ALTER TABLE companies ADD CONSTRAINT companies_name_nonempty CHECK(btrim(name)!='');
ALTER TABLE companies ADD CONSTRAINT companies_workspace_root_nonempty CHECK(btrim(workspace_root)!='');

ALTER TABLE employees
 ADD COLUMN display_name text NOT NULL DEFAULT '',
 ADD COLUMN role_name text NOT NULL DEFAULT '',
 ADD COLUMN model_profile text NOT NULL DEFAULT 'deterministic/fake',
 ADD COLUMN enabled boolean NOT NULL DEFAULT true;
UPDATE employees SET
 display_name = CASE id
   WHEN 'emp-planning' THEN '规划工程师'
   WHEN 'emp-backend' THEN '后端工程师'
   WHEN 'emp-frontend' THEN '前端工程师'
   WHEN 'emp-review' THEN '独立验收员'
   ELSE id
 END,
 role_name = CASE id
   WHEN 'emp-planning' THEN 'planning'
   WHEN 'emp-backend' THEN 'backend'
   WHEN 'emp-frontend' THEN 'frontend'
   WHEN 'emp-review' THEN 'review'
   ELSE 'unknown'
 END
WHERE display_name='' OR role_name='';
ALTER TABLE employees ADD CONSTRAINT employees_display_name_nonempty CHECK(btrim(display_name)!='');
ALTER TABLE employees ADD CONSTRAINT employees_role_name_nonempty CHECK(btrim(role_name)!='');
ALTER TABLE employees ADD CONSTRAINT employees_model_profile_nonempty CHECK(btrim(model_profile)!='');

CREATE INDEX companies_state ON companies(state,id);

-- +goose Down
DROP INDEX companies_state;
ALTER TABLE employees DROP CONSTRAINT employees_model_profile_nonempty;
ALTER TABLE employees DROP CONSTRAINT employees_role_name_nonempty;
ALTER TABLE employees DROP CONSTRAINT employees_display_name_nonempty;
ALTER TABLE employees DROP COLUMN enabled, DROP COLUMN model_profile, DROP COLUMN role_name, DROP COLUMN display_name;
ALTER TABLE companies DROP CONSTRAINT companies_workspace_root_nonempty;
ALTER TABLE companies DROP CONSTRAINT companies_name_nonempty;
ALTER TABLE companies DROP COLUMN state, DROP COLUMN workspace_root, DROP COLUMN name;
