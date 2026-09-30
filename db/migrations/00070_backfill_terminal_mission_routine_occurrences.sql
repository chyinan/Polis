-- +goose Up
-- Schema 69 handles future terminal transitions. Retire pending occurrences
-- whose Mission was already terminal before that trigger existed.
UPDATE routine_occurrences o
SET state='cancelled'
FROM routines r JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
WHERE o.company_id=r.company_id AND o.routine_id=r.id
 AND m.state IN ('cancelled','succeeded') AND o.state='pending';

-- +goose Down
-- Do not resurrect occurrences whose Mission is terminal.
