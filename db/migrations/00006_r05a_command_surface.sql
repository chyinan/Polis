-- +goose Up
ALTER TABLE missions ADD COLUMN title text NOT NULL DEFAULT 'untitled';
ALTER TABLE missions ADD COLUMN goal text NOT NULL DEFAULT 'goal unavailable';
UPDATE missions SET title=id WHERE title='untitled';
ALTER TABLE missions DROP CONSTRAINT missions_state_check;
ALTER TABLE missions ADD CONSTRAINT missions_state_check CHECK(state IN ('draft','active','paused','succeeded','cancelled'));
ALTER TABLE missions ADD CONSTRAINT missions_title_nonempty CHECK(btrim(title)!='');
ALTER TABLE missions ADD CONSTRAINT missions_goal_nonempty CHECK(btrim(goal)!='');

ALTER TABLE tasks DROP CONSTRAINT tasks_state_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_state_check CHECK(state IN ('ready','working','candidate','completed','cancelled'));

-- +goose Down
ALTER TABLE tasks DROP CONSTRAINT tasks_state_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_state_check CHECK(state IN ('ready','working','candidate','completed'));
ALTER TABLE missions DROP CONSTRAINT missions_goal_nonempty;
ALTER TABLE missions DROP CONSTRAINT missions_title_nonempty;
ALTER TABLE missions DROP CONSTRAINT missions_state_check;
ALTER TABLE missions ADD CONSTRAINT missions_state_check CHECK(state IN ('draft','active','paused','succeeded'));
ALTER TABLE missions DROP COLUMN goal;
ALTER TABLE missions DROP COLUMN title;
