SELECT
  (SELECT count(*) FROM companies WHERE id='r05b-live-2') || '|' ||
  (SELECT count(*) FROM employees WHERE company_id='r05b-live-2') || '|' ||
  (SELECT count(*) FROM missions WHERE company_id='r05b-live-2') || '|' ||
  COALESCE((SELECT state FROM missions WHERE company_id='r05b-live-2' AND id=:'mission'),'missing') || '|' ||
  (SELECT count(*) FROM tasks WHERE company_id='r05b-live-2' AND mission_id=:'mission') || '|' ||
  (SELECT count(*) FROM worker_sessions WHERE company_id='r05b-live-2');
