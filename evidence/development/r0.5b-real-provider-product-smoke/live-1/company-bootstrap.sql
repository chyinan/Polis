BEGIN;
INSERT INTO companies(id) VALUES ('r05b-live-1');
INSERT INTO employees(company_id, id) VALUES
  ('r05b-live-1', 'emp-planning'),
  ('r05b-live-1', 'emp-backend'),
  ('r05b-live-1', 'emp-frontend'),
  ('r05b-live-1', 'emp-review');
COMMIT;
