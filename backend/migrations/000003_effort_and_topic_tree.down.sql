ALTER TABLE tasks DROP CONSTRAINT tasks_fixed_time_xor_effort;
ALTER TABLE tasks DROP COLUMN effort_minutes;
DROP INDEX topics_parent_idx;
ALTER TABLE topics DROP COLUMN parent_id;
