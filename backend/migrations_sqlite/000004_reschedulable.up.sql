ALTER TABLE tasks ADD COLUMN reschedulable boolean NOT NULL DEFAULT 0;

UPDATE tasks SET reschedulable = 1
WHERE recurrence_kind IN ('once', 'spaced_repetition');
