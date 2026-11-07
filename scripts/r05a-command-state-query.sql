SELECT json_build_object(
  'schema_version', (SELECT max(version_id) FROM goose_db_version WHERE is_applied),
  'mission', (SELECT json_build_object('id',id,'title',title,'goal',goal,'state',state) FROM missions WHERE company_id='r05a-e2e-company'),
  'worker_sessions', COALESCE((SELECT json_agg(json_build_object('id',id,'employee_id',employee_id,'task_id',task_id,'state',state,'stop_receipt',stop_receipt) ORDER BY id) FROM worker_sessions WHERE company_id='r05a-e2e-company'), '[]'::json),
  'tasks', COALESCE((SELECT json_agg(json_build_object('id',id,'kind',kind,'state',state,'generation',generation) ORDER BY id) FROM tasks WHERE company_id='r05a-e2e-company'), '[]'::json),
  'events', COALESCE((SELECT json_agg(json_build_object('company_seq',company_seq,'kind',kind,'target_id',payload->>'id','occurred_at',payload->>'occurred_at') ORDER BY company_seq) FROM events WHERE company_id='r05a-e2e-company'), '[]'::json),
  'provider_egress', 0,
  'medium', 0,
  'high', 0
) AS r05a_authoritative_state;
