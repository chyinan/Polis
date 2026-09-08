-- name: ListTasks :many
SELECT id, mission_id, owner, kind, state, generation FROM tasks
WHERE company_id=$1 AND mission_id=$2 ORDER BY id;

-- name: ListEmployees :many
SELECT id FROM employees WHERE company_id=$1 ORDER BY id;

-- name: ListArtifacts :many
SELECT a.id,a.digest,a.state,a.verdict FROM artifacts a JOIN tasks t
ON t.company_id=a.company_id AND t.id=a.task_id
WHERE a.company_id=$1 AND t.mission_id=$2 ORDER BY a.id;

-- name: ListObligations :many
SELECT o.id,o.state FROM obligations o JOIN tasks t
ON t.company_id=o.company_id AND t.id=o.task_id
WHERE o.company_id=$1 AND t.mission_id=$2 ORDER BY o.id;

-- name: ListMessages :many
SELECT id FROM messages WHERE company_id=$1 AND mission_id=$2 ORDER BY id;

-- name: LockCompany :one
SELECT id FROM companies WHERE id=$1 FOR UPDATE;
