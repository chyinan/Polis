SELECT jsonb_build_object(
  'record_type', 'polis-live-1-authoritative-state@1',
  'captured_at', clock_timestamp(),
  'database', current_database(),
  'postgres_version', current_setting('server_version'),
  'schema_version', (SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1),
  'company', (
    SELECT jsonb_build_object('id', id, 'company_seq', company_seq)
    FROM companies WHERE id = 'r05b-live-1'
  ),
  'employees', COALESCE((
    SELECT jsonb_agg(id ORDER BY id) FROM employees WHERE company_id = 'r05b-live-1'
  ), '[]'::jsonb),
  'mission', (
    SELECT jsonb_build_object(
      'mission_id', id,
      'title', title,
      'goal', goal,
      'state', state,
      'activation_id', activation_id,
      'acceptance_contract', acceptance_contract
    )
    FROM missions WHERE company_id = 'r05b-live-1'
  ),
  'counts', jsonb_build_object(
    'companies', (SELECT count(*) FROM companies),
    'missions', (SELECT count(*) FROM missions WHERE company_id = 'r05b-live-1'),
    'tasks', (SELECT count(*) FROM tasks WHERE company_id = 'r05b-live-1'),
    'task_validation_bindings', (SELECT count(*) FROM task_validation_bindings WHERE company_id = 'r05b-live-1'),
    'worker_workspaces', (SELECT count(*) FROM worker_workspaces WHERE company_id = 'r05b-live-1'),
    'worker_sessions', (SELECT count(*) FROM worker_sessions WHERE company_id = 'r05b-live-1'),
    'worker_checks', (SELECT count(*) FROM worker_checks WHERE company_id = 'r05b-live-1'),
    'worker_checkpoints', (SELECT count(*) FROM worker_checkpoints WHERE company_id = 'r05b-live-1'),
    'artifacts', (SELECT count(*) FROM artifacts WHERE company_id = 'r05b-live-1'),
    'artifact_qualifications', (SELECT count(*) FROM task_validation_artifact_qualifications WHERE company_id = 'r05b-live-1')
  ),
  'tasks', COALESCE((
    SELECT jsonb_agg(jsonb_build_object('id',id,'mission_id',mission_id,'owner',owner,'kind',kind,'state',state) ORDER BY id)
    FROM tasks WHERE company_id = 'r05b-live-1'
  ), '[]'::jsonb),
  'events', COALESCE((
    SELECT jsonb_agg(jsonb_build_object('company_seq',company_seq,'kind',kind,'payload',payload,'observed',observed) ORDER BY company_seq)
    FROM events WHERE company_id = 'r05b-live-1'
  ), '[]'::jsonb)
);
