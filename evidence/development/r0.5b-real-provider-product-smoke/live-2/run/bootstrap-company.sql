BEGIN;
INSERT INTO companies(id) VALUES ('r05b-live-2');
INSERT INTO employees(company_id,id) VALUES
  ('r05b-live-2','emp-planning'),
  ('r05b-live-2','emp-backend'),
  ('r05b-live-2','emp-frontend'),
  ('r05b-live-2','emp-review');
COMMIT;
