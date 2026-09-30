-- Disposable R0.4 Workbench integration fixture.
-- Run only against a dedicated polis_r0_* database after migrations 00001-00005.
BEGIN;

INSERT INTO companies(id, company_seq) VALUES ('r04-workbench-company', 8);
INSERT INTO employees(company_id, id, epoch) VALUES
  ('r04-workbench-company', 'emp-backend', 2),
  ('r04-workbench-company', 'emp-frontend', 2),
  ('r04-workbench-company', 'emp-planning', 1),
  ('r04-workbench-company', 'emp-review', 1);
INSERT INTO missions(company_id, id, state, activation_id, contract)
VALUES ('r04-workbench-company', 'r04-workbench-mission', 'active', 'r04-workbench-activation', 'r03-api@1');
INSERT INTO tasks(company_id, id, mission_id, owner, kind, state, generation, plan) VALUES
  ('r04-workbench-company', 'r04-backend-task', 'r04-workbench-mission', 'emp-backend', 'peer_backend', 'candidate', 3, '{"role":"backend"}'),
  ('r04-workbench-company', 'r04-frontend-task', 'r04-workbench-mission', 'emp-frontend', 'peer_frontend', 'working', 2, '{"role":"frontend"}');
INSERT INTO contract_revisions(company_id, id, mission_id, revision, base_revision, endpoint, schema, digest, state, proposer, accepter)
VALUES ('r04-workbench-company', 'r04-contract-rev-1', 'r04-workbench-mission', 1, NULL, 'GET /items?cursor={cursor}&limit={limit}', '{"items":[{"id":"string","name":"string"}],"next_cursor":"string|null"}', repeat('1',64), 'accepted', 'emp-backend', 'emp-backend');
INSERT INTO messages(company_id, id, mission_id, task_id, sender, recipient, kind, body, delivery_state, contract_revision_id, task_revision)
VALUES ('r04-workbench-company', 'r04-message-1', 'r04-workbench-mission', 'r04-frontend-task', 'emp-backend', 'emp-frontend', 'request', 'Consume the accepted cursor and limit contract.', 'observed', 'r04-contract-rev-1', 2);
INSERT INTO obligations(company_id, id, task_id, owner, state, evidence_ref)
VALUES ('r04-workbench-company', 'r04-message-1', 'r04-frontend-task', 'emp-frontend', 'pending', NULL);
INSERT INTO peer_work_signals(company_id, id, message_id, obligation_id, recipient, state)
VALUES ('r04-workbench-company', 'r04-signal-1', 'r04-message-1', 'r04-message-1', 'emp-frontend', 'observed');
INSERT INTO task_revisions(company_id, id, task_id, revision, digest, contract_revision_id, state) VALUES
  ('r04-workbench-company', 'r04-task-rev-backend', 'r04-backend-task', 3, repeat('2',64), 'r04-contract-rev-1', 'candidate'),
  ('r04-workbench-company', 'r04-task-rev-frontend', 'r04-frontend-task', 2, repeat('3',64), 'r04-contract-rev-1', 'working');
INSERT INTO worker_workspaces(company_id, task_id, digest, revision) VALUES
  ('r04-workbench-company', 'r04-backend-task', repeat('2',64), 3),
  ('r04-workbench-company', 'r04-frontend-task', repeat('3',64), 2);
INSERT INTO worker_sessions(company_id, id, employee_id, task_id, generation, epoch, incarnation, profile, state, tool_call_limit, tool_calls_used, stop_receipt)
VALUES
  ('r04-workbench-company', 'r04-backend-session', 'emp-backend', 'r04-backend-task', 3, 2, 'r04-incarnation', 'gpt-5.6-luna/medium', 'active', 48, 17, NULL),
  ('r04-workbench-company', 'r04-frontend-session', 'emp-frontend', 'r04-frontend-task', 2, 2, 'r04-incarnation', 'gpt-5.6-luna/medium', 'stopped', 48, 12, 'r04-stop-receipt');
INSERT INTO worker_checks(company_id, id, session_id, digest, phase, passed, report)
VALUES ('r04-workbench-company', 'r04-check-1', 'r04-backend-session', repeat('2',64), 'full', true, '{"acceptance_checker_revision":"r04-checker"}');
INSERT INTO worker_checkpoints(company_id, id, session_id, digest, data)
VALUES ('r04-workbench-company', 'r04-checkpoint-1', 'r04-backend-session', repeat('2',64), '{"kind":"qualified","workspace_revision":3,"contract_revision_id":"r04-contract-rev-1"}');
INSERT INTO artifacts(company_id, id, task_id, author, digest, bytes, state, verdict, contract)
VALUES ('r04-workbench-company', 'r04-artifact-1', 'r04-backend-task', 'emp-backend', repeat('2',64), 128, 'ready', 'candidate', 'r03-api@1');
INSERT INTO artifact_qualifications(company_id, artifact_id, checkpoint_id, workspace_revision, workspace_digest, contract_revision_id, acceptance_checker_revision, checkpoint_policy_revision, artifact_eligibility_policy_revision, contract_supersession_policy_revision)
VALUES ('r04-workbench-company', 'r04-artifact-1', 'r04-checkpoint-1', 3, repeat('2',64), 'r04-contract-rev-1', 'r04-checker', 'r04-checkpoint-policy', 'r04-artifact-policy', 'r04-contract-policy');
INSERT INTO events(company_id, company_seq, kind, payload, observed) VALUES
  ('r04-workbench-company', 1, 'mission.created', '{"id":"r04-workbench-mission"}', true),
  ('r04-workbench-company', 2, 'task.claim', '{"id":"r04-backend-task"}', true),
  ('r04-workbench-company', 3, 'contract.propose', '{"id":"r04-contract-rev-1"}', true),
  ('r04-workbench-company', 4, 'contract.accept', '{"id":"r04-contract-rev-1"}', true),
  ('r04-workbench-company', 5, 'collab.send', '{"id":"r04-message-1"}', true),
  ('r04-workbench-company', 6, 'work.checkpoint', '{"id":"r04-checkpoint-1"}', true),
  ('r04-workbench-company', 7, 'artifact.submit', '{"id":"r04-artifact-1"}', true),
  ('r04-workbench-company', 8, 'workspace.replace', '{"id":"r04-backend-task"}', true);

COMMIT;
