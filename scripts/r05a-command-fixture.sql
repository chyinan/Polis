-- Disposable R0.5A command integration fixture. It creates only the existing Company scope and fixed employees.
INSERT INTO companies(id) VALUES ('r05a-e2e-company');
INSERT INTO employees(company_id,id) VALUES
 ('r05a-e2e-company','emp-planning'),
 ('r05a-e2e-company','emp-backend'),
 ('r05a-e2e-company','emp-frontend'),
 ('r05a-e2e-company','emp-review');
