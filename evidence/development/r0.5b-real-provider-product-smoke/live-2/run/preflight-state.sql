SELECT jsonb_pretty(jsonb_build_object(
  'postgres_version', current_setting('server_version'),
  'schema_version', (SELECT max(version_id) FROM goose_db_version WHERE is_applied),
  'companies', (SELECT count(*) FROM companies WHERE id='r05b-live-2'),
  'employees', (SELECT count(*) FROM employees WHERE company_id='r05b-live-2'),
  'missions', (SELECT count(*) FROM missions WHERE company_id='r05b-live-2'),
  'tasks', (SELECT count(*) FROM tasks WHERE company_id='r05b-live-2'),
  'worker_sessions', (SELECT count(*) FROM worker_sessions WHERE company_id='r05b-live-2'),
  'provider_allowance_present', false
));
