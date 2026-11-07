\set ON_ERROR_STOP on
BEGIN;

UPDATE runtime_control SET incarnation = 'source-incarnation' WHERE singleton;
INSERT INTO companies(id, company_seq) VALUES ('recovery-company', 22);
INSERT INTO employees(company_id, id, epoch) VALUES
  ('recovery-company', 'emp-backend', 9),
  ('recovery-company', 'emp-frontend', 4),
  ('recovery-company', 'emp-planning', 1),
  ('recovery-company', 'emp-review', 1);
INSERT INTO missions(company_id, id, state, activation_id, contract)
VALUES ('recovery-company', 'recovery-mission', 'active', 'recovery-activation', 'r03-api@1');
INSERT INTO tasks(company_id, id, mission_id, owner, kind, state, generation, plan) VALUES
  ('recovery-company', 'recovery-backend-task', 'recovery-mission', 'emp-backend', 'peer_backend', 'candidate', 10, '{"role":"backend","contract":"api-pagination@4"}'),
  ('recovery-company', 'recovery-frontend-task', 'recovery-mission', 'emp-frontend', 'peer_frontend', 'ready', 0, '{"role":"frontend","contract":"api-pagination@4"}'),
  ('recovery-company', 'recovery-planner-task', 'recovery-mission', 'emp-planning', 'bootstrap_plan', 'completed', 0, '{"fixture":"recovery"}');
INSERT INTO contract_revisions(company_id, id, mission_id, revision, base_revision, endpoint, schema, digest, state, proposer, accepter) VALUES
  ('recovery-company', 'recovery-contract-v1', 'recovery-mission', 1, NULL, 'GET /items', '{"items":["id","name"]}', repeat('1',64), 'superseded', 'emp-backend', 'emp-backend'),
  ('recovery-company', 'recovery-contract-v2', 'recovery-mission', 2, 1, 'GET /items?cursor={cursor}', '{"items":["id","name"],"next_cursor":"string|null"}', repeat('2',64), 'superseded', 'emp-backend', 'emp-backend'),
  ('recovery-company', 'recovery-contract-v3', 'recovery-mission', 3, 2, 'GET /items?cursor={cursor}', '{"items":["id","name"],"next_cursor":"string"}', repeat('3',64), 'superseded', 'emp-backend', 'emp-backend'),
  ('recovery-company', 'recovery-contract-v4', 'recovery-mission', 4, 3, 'GET /items?cursor={cursor}', '{"items":["id","name"],"next_cursor":"string"}', repeat('4',64), 'accepted', 'emp-backend', 'emp-backend');
INSERT INTO messages(company_id, id, mission_id, task_id, sender, recipient, kind, body, delivery_state, contract_revision_id, task_revision)
VALUES ('recovery-company', 'recovery-peer-message', 'recovery-mission', 'recovery-frontend-task', 'emp-backend', 'emp-frontend', 'request', 'Apply the accepted cursor contract and update the frontend pagination state.', 'persisted', 'recovery-contract-v4', 10);
INSERT INTO obligations(company_id, id, task_id, owner, state, evidence_ref)
VALUES ('recovery-company', 'recovery-peer-message', 'recovery-frontend-task', 'emp-frontend', 'pending', NULL);
INSERT INTO peer_work_signals(company_id, id, message_id, obligation_id, recipient, state)
VALUES ('recovery-company', 'recovery-peer-signal', 'recovery-peer-message', 'recovery-peer-message', 'emp-frontend', 'pending');
INSERT INTO worker_workspaces(company_id, task_id, digest, revision) VALUES
  ('recovery-company', 'recovery-backend-task', :'backend_digest', 10),
  ('recovery-company', 'recovery-frontend-task', :'frontend_digest', 1);
INSERT INTO worker_sessions(company_id, id, employee_id, task_id, generation, epoch, incarnation, profile, state, tool_call_limit, tool_calls_used, stop_receipt)
VALUES ('recovery-company', 'recovery-backend-session', 'emp-backend', 'recovery-backend-task', 11, 9, 'source-incarnation', 'gpt-5.6-luna/medium', 'stopped', 48, 41, 'windows-process-handle:recovery:waited');
INSERT INTO worker_checks(company_id, id, session_id, digest, phase, passed, report)
VALUES ('recovery-company', 'recovery-check-pass', 'recovery-backend-session', :'backend_digest', 'full', true,
  jsonb_build_object('acceptance_checker_revision','peer-semantic-checker@2','passed',true,'relevant_contract_revision','recovery-contract-v4','relevant_workspace_revision',10,'digest',:'backend_digest'));
INSERT INTO worker_checkpoints(company_id, id, session_id, digest, data)
VALUES ('recovery-company', 'recovery-qualified-checkpoint', 'recovery-backend-session', :'backend_digest',
  jsonb_build_object('kind','qualified','summary','Backend candidate is qualified before handover','facts',jsonb_build_array('final rev4 is accepted','workspace revision 10 is checked'),'decisions',jsonb_build_array('send the current contract to emp-frontend'),'rejected',jsonb_build_array('stale rev3 responsibility'),'evidence',jsonb_build_array('recovery-check-pass'),'failed_checks',jsonb_build_array(),'next_action','Frontend observes and applies rev4','workspace_digest',:'backend_digest','workspace_revision',10,'contract_revision_id','recovery-contract-v4','acceptance_checker_revision','peer-semantic-checker@2','checkpoint_policy_revision','checkpoint-effective-contract@2','artifact_eligibility_policy_revision','artifact-commit-qualification@2','contract_supersession_policy_revision','peer-contract-supersession@2','finalization_state','current','pending_message_id','recovery-peer-message','pending_message_state','persisted','pending_obligation_id','recovery-peer-message','pending_obligation_state','pending','session_id','recovery-backend-session','epoch',9));
INSERT INTO task_revisions(company_id, id, task_id, revision, digest, contract_revision_id, state)
VALUES ('recovery-company', 'recovery-task-revision-10', 'recovery-backend-task', 10, :'backend_digest', 'recovery-contract-v4', 'fixed');
INSERT INTO artifacts(company_id, id, task_id, author, digest, bytes, state, verdict, verifier, contract)
VALUES ('recovery-company', 'recovery-backend-artifact', 'recovery-backend-task', 'emp-backend', :'backend_digest', :backend_bytes, 'ready', 'candidate', NULL, 'r03-api@1');
INSERT INTO artifact_staging(company_id, id, task_id, digest)
VALUES ('recovery-company', 'recovery-backend-artifact-stage', 'recovery-backend-task', :'backend_digest');
INSERT INTO artifact_qualifications(company_id, artifact_id, checkpoint_id, workspace_revision, workspace_digest, contract_revision_id, acceptance_checker_revision, checkpoint_policy_revision, artifact_eligibility_policy_revision, contract_supersession_policy_revision)
VALUES ('recovery-company', 'recovery-backend-artifact', 'recovery-qualified-checkpoint', 10, :'backend_digest', 'recovery-contract-v4', 'peer-semantic-checker@2', 'checkpoint-effective-contract@2', 'artifact-commit-qualification@2', 'peer-contract-supersession@2');
INSERT INTO receipts(company_id, actor, key, fingerprint, result) VALUES
  ('recovery-company', 'emp-backend', 'recovery-work-current', repeat('a',64), '{"status":"persisted"}'),
  ('recovery-company', 'emp-backend', 'recovery-collab-send', repeat('b',64), '{"id":"recovery-peer-message","status":"persisted"}'),
  ('recovery-company', 'emp-backend', 'recovery-checkpoint', repeat('c',64), '{"id":"recovery-qualified-checkpoint","status":"persisted"}'),
  ('recovery-company', 'emp-backend', 'recovery-artifact-submit', repeat('d',64), '{"id":"recovery-backend-artifact","status":"candidate"}');
INSERT INTO events(company_id, company_seq, kind, payload, observed)
SELECT 'recovery-company', n,
  CASE n WHEN 1 THEN 'mission.created' WHEN 2 THEN 'task.backend.created' WHEN 3 THEN 'task.frontend.created' WHEN 4 THEN 'contract.rev1.accepted' WHEN 5 THEN 'contract.rev2.accepted' WHEN 6 THEN 'contract.rev3.accepted' WHEN 7 THEN 'contract.rev4.accepted' WHEN 8 THEN 'workspace.revision.2' WHEN 9 THEN 'workspace.revision.3' WHEN 10 THEN 'workspace.revision.4' WHEN 11 THEN 'workspace.revision.5' WHEN 12 THEN 'workspace.revision.6' WHEN 13 THEN 'workspace.revision.7' WHEN 14 THEN 'workspace.revision.8' WHEN 15 THEN 'workspace.revision.9' WHEN 16 THEN 'workspace.revision.10' WHEN 17 THEN 'workspace.check.failed' WHEN 18 THEN 'workspace.check.passed' WHEN 19 THEN 'checkpoint.qualified' WHEN 20 THEN 'collab.message.persisted' WHEN 21 THEN 'obligation.pending' ELSE 'artifact.candidate' END,
  jsonb_build_object('fixture','r0.3a-authoritative-recovery','sequence',n), true
FROM generate_series(1,22) AS n;
COMMIT;
