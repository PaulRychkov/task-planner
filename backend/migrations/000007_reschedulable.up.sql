ALTER TABLE tasks ADD COLUMN reschedulable boolean NOT NULL DEFAULT false;

UPDATE tasks SET reschedulable = true
WHERE recurrence_kind IN ('once', 'spaced_repetition');
