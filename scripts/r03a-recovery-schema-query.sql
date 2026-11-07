\set ON_ERROR_STOP on
SELECT max(version_id) FROM goose_db_version WHERE is_applied;
