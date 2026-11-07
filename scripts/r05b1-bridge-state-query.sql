SELECT json_build_object(
  'schema_version', (SELECT max(version_id) FROM goose_db_version WHERE is_applied),
  'missions', COALESCE((SELECT json_agg(json_build_object('id',id,'state',state,'title',title,'goal',goal) ORDER BY id) FROM missions), '[]'::json),
  'tasks', COALESCE((SELECT json_agg(json_build_object('id',id,'mission_id',mission_id,'owner',owner,'kind',kind,'state',state,'generation',generation) ORDER BY id) FROM tasks), '[]'::json),
  'worker_sessions', COALESCE((SELECT json_agg(json_build_object('id',id,'employee_id',employee_id,'task_id',task_id,'state',state,'profile',profile,'stop_receipt',stop_receipt) ORDER BY id) FROM worker_sessions), '[]'::json),
  'artifacts', COALESCE((SELECT json_agg(json_build_object('id',id,'task_id',task_id,'state',state,'verdict',verdict,'digest',digest) ORDER BY id) FROM artifacts), '[]'::json),
  'provider_terminal_observations', COALESCE((SELECT json_agg(data ORDER BY id) FROM worker_observations WHERE reason='provider_terminal'), '[]'::json),
  'events', COALESCE((SELECT json_agg(json_build_object('company_seq',company_seq,'kind',kind,'occurred_at',payload->>'occurred_at') ORDER BY company_seq) FROM events), '[]'::json),
  'live_worker_sessions', (SELECT count(*) FROM worker_sessions WHERE state!='stopped'),
  'provider_reservation', 0,
  'provider_egress', 0,
  'medium', 0,
  'high', 0
) AS r05b1_authoritative_state;
