ALTER TABLE topics ADD COLUMN parent_id uuid REFERENCES topics(id) ON DELETE SET NULL;
CREATE INDEX topics_parent_idx ON topics(parent_id);
ALTER TABLE tasks ADD COLUMN effort_minutes integer CHECK (effort_minutes > 0);
ALTER TABLE tasks ADD CONSTRAINT tasks_fixed_time_xor_effort
    CHECK (NOT (start_time_minutes IS NOT NULL AND effort_minutes IS NOT NULL));
