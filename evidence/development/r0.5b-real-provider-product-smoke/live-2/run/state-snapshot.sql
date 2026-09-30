\set ON_ERROR_STOP on
SELECT jsonb_pretty(jsonb_build_object(
  'company', jsonb_build_object(
    'id', c.id,
    'company_seq', c.company_seq,
    'employees', COALESCE((SELECT jsonb_agg(jsonb_build_object('id', e.id, 'epoch', e.epoch) ORDER BY e.id)
      FROM employees e WHERE e.company_id=c.id), '[]'::jsonb)
  ),
  'mission', (SELECT jsonb_build_object('id',m.id,'title',m.title,'goal',m.goal,'state',m.state,'contract',m.contract,'acceptance_contract',m.acceptance_contract)
    FROM missions m WHERE m.company_id=c.id AND m.id=(SELECT id FROM missions WHERE company_id=c.id)),
  'tasks', COALESCE((SELECT jsonb_agg(jsonb_build_object('id',t.id,'mission_id',t.mission_id,'kind',t.kind,'owner',t.owner,'state',t.state,'generation',t.generation)
    ORDER BY t.kind,t.id) FROM tasks t WHERE t.company_id=c.id AND t.mission_id=(SELECT id FROM missions WHERE company_id=c.id)), '[]'::jsonb),
  'task_validation_bindings', COALESCE((SELECT jsonb_agg(jsonb_build_object('task_id',v.task_id,'mission_id',v.mission_id,'acceptance_revision',v.acceptance_revision,'runner_kind',v.runner_kind,'runner_revision',v.runner_revision,'configuration_digest',v.configuration_digest,'contract',v.contract)
    ORDER BY v.task_id) FROM task_validation_bindings v WHERE v.company_id=c.id AND v.mission_id=(SELECT id FROM missions WHERE company_id=c.id)), '[]'::jsonb),
  'workspaces', COALESCE((SELECT jsonb_agg(jsonb_build_object('task_id',w.task_id,'digest',w.digest,'revision',w.revision) ORDER BY w.task_id)
    FROM worker_workspaces w JOIN tasks t ON t.company_id=w.company_id AND t.id=w.task_id
    WHERE w.company_id=c.id AND t.mission_id=(SELECT id FROM missions WHERE company_id=c.id)), '[]'::jsonb),
  'worker_sessions', COALESCE((SELECT jsonb_agg(jsonb_build_object('id',s.id,'task_id',s.task_id,'employee_id',s.employee_id,'profile',s.profile,'state',s.state,'epoch',s.epoch,'incarnation',s.incarnation,'process_pid',s.process_pid,'stop_receipt',s.stop_receipt,'tool_call_limit',s.tool_call_limit,'tool_calls_used',s.tool_calls_used)
    ORDER BY s.id) FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
    WHERE s.company_id=c.id AND t.mission_id=(SELECT id FROM missions WHERE company_id=c.id)), '[]'::jsonb),
  'checks', COALESCE((SELECT jsonb_agg(jsonb_build_object('id',q.id,'session_id',q.session_id,'digest',q.digest,'phase',q.phase,'passed',q.passed,'report',q.report) ORDER BY q.id)
    FROM worker_checks q JOIN worker_sessions s ON s.company_id=q.company_id AND s.id=q.session_id
    JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
    WHERE q.company_id=c.id AND t.mission_id=(SELECT id FROM missions WHERE company_id=c.id)), '[]'::jsonb),
  'checkpoints', COALESCE((SELECT jsonb_agg(jsonb_build_object('id',q.id,'session_id',q.session_id,'digest',q.digest,'data',q.data) ORDER BY q.id)
    FROM worker_checkpoints q JOIN worker_sessions s ON s.company_id=q.company_id AND s.id=q.session_id
    JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
    WHERE q.company_id=c.id AND t.mission_id=(SELECT id FROM missions WHERE company_id=c.id)), '[]'::jsonb),
  'artifacts', COALESCE((SELECT jsonb_agg(jsonb_build_object('id',a.id,'task_id',a.task_id,'digest',a.digest,'state',a.state,'verdict',a.verdict,'contract',a.contract) ORDER BY a.id)
    FROM artifacts a WHERE a.company_id=c.id AND a.task_id IN (SELECT t.id FROM tasks t WHERE t.company_id=c.id AND t.mission_id=(SELECT id FROM missions WHERE company_id=c.id))), '[]'::jsonb),
  'artifact_qualifications', COALESCE((SELECT jsonb_agg(jsonb_build_object('task_id',q.task_id,'artifact_id',q.artifact_id,'checkpoint_id',q.checkpoint_id,'check_id',q.check_id,'session_id',q.session_id,'validation_binding_digest',q.validation_binding_digest,'workspace_digest',q.workspace_digest,'workspace_revision',q.workspace_revision,'runner_revision',q.runner_revision) ORDER BY q.task_id)
    FROM task_validation_artifact_qualifications q WHERE q.company_id=c.id AND q.task_id IN (SELECT t.id FROM tasks t WHERE t.company_id=c.id AND t.mission_id=(SELECT id FROM missions WHERE company_id=c.id))), '[]'::jsonb),
  'provider_terminal_observations', COALESCE((SELECT jsonb_agg(jsonb_build_object('session_id',s.id,'data',o.data) ORDER BY o.id)
    FROM worker_observations o JOIN worker_sessions s ON s.company_id=o.company_id AND s.id=o.session_id
    JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
    WHERE o.company_id=c.id AND o.reason='provider_terminal' AND t.mission_id=(SELECT id FROM missions WHERE company_id=c.id)), '[]'::jsonb),
  'activity', COALESCE((SELECT jsonb_agg(jsonb_build_object('company_seq',e.company_seq,'kind',e.kind,'payload',e.payload) ORDER BY e.company_seq)
    FROM events e WHERE e.company_id=c.id), '[]'::jsonb)
))
FROM companies c WHERE c.id='r05b-live-2';
