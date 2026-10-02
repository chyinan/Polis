-- +goose Up
ALTER TABLE tasks
 ADD COLUMN parent_task_id text,
 ADD COLUMN problem_key text,
 ADD CONSTRAINT tasks_parent_same_mission_fk
  FOREIGN KEY(company_id,parent_task_id,mission_id) REFERENCES tasks(company_id,id,mission_id);

-- Reconstruct only unambiguous legacy parent links from fixed task factories.
UPDATE tasks child SET parent_task_id=bootstrap.id
FROM tasks bootstrap
WHERE child.company_id=bootstrap.company_id AND child.mission_id=bootstrap.mission_id
 AND child.kind='compat' AND bootstrap.kind='bootstrap_plan';

UPDATE tasks child SET parent_task_id=bootstrap.id
FROM tasks bootstrap
WHERE child.company_id=bootstrap.company_id AND child.mission_id=bootstrap.mission_id
 AND child.kind='compute' AND bootstrap.kind='bootstrap_plan'
 AND COALESCE(child.plan->>'template','')<>'daily-routine-task@1'
 AND EXISTS(SELECT 1 FROM messages m WHERE m.company_id=child.company_id AND m.task_id=child.id AND m.sender='emp-planning' AND m.kind='request');

UPDATE tasks child SET parent_task_id=a.task_id
FROM artifacts a
WHERE child.company_id=a.company_id AND child.mission_id=(SELECT mission_id FROM tasks parent WHERE parent.company_id=a.company_id AND parent.id=a.task_id)
 AND child.kind='review' AND child.plan->>'review_of'=a.id;

WITH RECURSIVE task_roots AS (
 SELECT company_id,id,parent_task_id,('problem:'||id)::text AS root_key
 FROM tasks WHERE parent_task_id IS NULL
 UNION ALL
 SELECT child.company_id,child.id,child.parent_task_id,parent.root_key
 FROM tasks child JOIN task_roots parent
  ON child.company_id=parent.company_id AND child.parent_task_id=parent.id
)
UPDATE tasks t SET problem_key=roots.root_key
FROM task_roots roots
WHERE roots.company_id=t.company_id AND roots.id=t.id;

ALTER TABLE tasks ALTER COLUMN problem_key SET NOT NULL;
CREATE INDEX tasks_problem_lineage ON tasks(company_id,problem_key,mission_id,id);

-- Task factories cannot choose or rewrite a ProblemKey. New roots get a stable
-- opaque key; children inherit the existing parent's key at insert time.
-- +goose StatementBegin
CREATE FUNCTION assign_task_problem_lineage() RETURNS trigger AS $$
DECLARE parent_problem_key text;
BEGIN
 IF NEW.parent_task_id IS NULL THEN
  NEW.problem_key := 'problem:'||NEW.id;
  RETURN NEW;
 END IF;
 SELECT problem_key INTO parent_problem_key
 FROM tasks
 WHERE company_id=NEW.company_id AND id=NEW.parent_task_id AND mission_id=NEW.mission_id;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'Task parent is absent or belongs to a different Mission' USING ERRCODE='23503';
 END IF;
 NEW.problem_key := parent_problem_key;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE FUNCTION reject_task_problem_lineage_change() RETURNS trigger AS $$
BEGIN
 IF NEW.parent_task_id IS DISTINCT FROM OLD.parent_task_id OR NEW.problem_key IS DISTINCT FROM OLD.problem_key THEN
  RAISE EXCEPTION 'Task problem lineage is immutable' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER tasks_assign_problem_lineage
 BEFORE INSERT ON tasks FOR EACH ROW EXECUTE FUNCTION assign_task_problem_lineage();
CREATE TRIGGER tasks_problem_lineage_immutable
 BEFORE UPDATE ON tasks FOR EACH ROW EXECUTE FUNCTION reject_task_problem_lineage_change();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM tasks) THEN
  RAISE EXCEPTION 'cannot remove persisted Task ProblemKey lineage';
 END IF;
END $$;
-- +goose StatementEnd
DROP TRIGGER tasks_problem_lineage_immutable ON tasks;
DROP TRIGGER tasks_assign_problem_lineage ON tasks;
DROP FUNCTION reject_task_problem_lineage_change();
DROP FUNCTION assign_task_problem_lineage();
DROP INDEX tasks_problem_lineage;
ALTER TABLE tasks
 DROP CONSTRAINT tasks_parent_same_mission_fk,
 DROP COLUMN problem_key,
 DROP COLUMN parent_task_id;
