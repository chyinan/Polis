SELECT json_build_object(
  'schema_version', (SELECT max(version_id) FROM goose_db_version WHERE is_applied),
  'missions', COALESCE((SELECT json_agg(json_build_object('id',id,'state',state,'title',title,'goal',goal,'acceptance_contract',acceptance_contract) ORDER BY id) FROM missions WHERE company_id='r05b3-offline-company'), '[]'::json),
  'tasks', COALESCE((SELECT json_agg(json_build_object('id',t.id,'mission_id',t.mission_id,'owner',t.owner,'kind',t.kind,'state',t.state,'generation',t.generation,'validation_binding_digest',v.configuration_digest) ORDER BY t.id) FROM tasks t LEFT JOIN task_validation_bindings v ON v.company_id=t.company_id AND v.task_id=t.id WHERE t.company_id='r05b3-offline-company'), '[]'::json),
  'task_validation_bindings', COALESCE((SELECT json_agg(json_build_object('task_id',task_id,'mission_id',mission_id,'acceptance_revision',acceptance_revision,'runner_kind',runner_kind,'runner_revision',runner_revision,'configuration_digest',configuration_digest,'contract',contract) ORDER BY task_id) FROM task_validation_bindings WHERE company_id='r05b3-offline-company'), '[]'::json),
  'worker_sessions', COALESCE((SELECT json_agg(json_build_object('id',id,'employee_id',employee_id,'task_id',task_id,'state',state,'profile',profile,'stop_receipt',stop_receipt) ORDER BY id) FROM worker_sessions WHERE company_id='r05b3-offline-company'), '[]'::json),
  'worker_checks', COALESCE((SELECT json_agg(json_build_object('id',id,'session_id',session_id,'digest',digest,'phase',phase,'passed',passed,'report',report) ORDER BY id) FROM worker_checks WHERE company_id='r05b3-offline-company'), '[]'::json),
  'checkpoints', COALESCE((SELECT json_agg(json_build_object('id',id,'session_id',session_id,'digest',digest,'data',data) ORDER BY id) FROM worker_checkpoints WHERE company_id='r05b3-offline-company'), '[]'::json),
  'artifacts', COALESCE((SELECT json_agg(json_build_object('id',id,'task_id',task_id,'author',author,'state',state,'verdict',verdict,'digest',digest,'bytes',bytes) ORDER BY id) FROM artifacts WHERE company_id='r05b3-offline-company'), '[]'::json),
  'artifact_qualifications', COALESCE((SELECT json_agg(json_build_object('task_id',task_id,'artifact_id',artifact_id,'checkpoint_id',checkpoint_id,'check_id',check_id,'session_id',session_id,'validation_binding_digest',validation_binding_digest,'workspace_digest',workspace_digest,'workspace_revision',workspace_revision,'runner_revision',runner_revision) ORDER BY task_id) FROM task_validation_artifact_qualifications WHERE company_id='r05b3-offline-company'), '[]'::json),
  'provider_terminal_observations', COALESCE((SELECT json_agg(data ORDER BY id) FROM worker_observations WHERE company_id='r05b3-offline-company' AND reason='provider_terminal'), '[]'::json),
  'events', COALESCE((SELECT json_agg(json_build_object('company_seq',company_seq,'kind',kind,'payload',payload) ORDER BY company_seq) FROM events WHERE company_id='r05b3-offline-company'), '[]'::json),
  'live_worker_sessions', (SELECT count(*) FROM worker_sessions WHERE company_id='r05b3-offline-company' AND state!='stopped'),
  'provider_reservations', 0,
  'provider_egress', 0,
  'medium', 0,
  'high', 0
) AS r05b3_authoritative_state;
