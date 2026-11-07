\set ON_ERROR_STOP on
UPDATE runtime_control SET incarnation = 'restore-incarnation' WHERE singleton;
