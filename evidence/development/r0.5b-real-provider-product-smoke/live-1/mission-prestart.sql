SELECT json_build_object(
  'company_count', (SELECT count(*) FROM companies),
  'company_id', 'r05b-live-1',
  'employee_identity_count', (SELECT count(*) FROM employees WHERE company_id = 'r05b-live-1'),
  'mission_count', (SELECT count(*) FROM missions WHERE company_id = 'r05b-live-1'),
  'mission', (
    SELECT json_build_object(
      'mission_id', id,
      'title', title,
      'goal', goal,
      'state', state,
      'acceptance_contract', acceptance_contract
    )
    FROM missions WHERE company_id = 'r05b-live-1'
  ),
  'task_count', (SELECT count(*) FROM tasks WHERE company_id = 'r05b-live-1' AND kind = 'compat'),
  'worker_session_count', (SELECT count(*) FROM worker_sessions WHERE company_id = 'r05b-live-1'),
  'task_validation_binding_count', (SELECT count(*) FROM task_validation_bindings WHERE company_id = 'r05b-live-1')
);
