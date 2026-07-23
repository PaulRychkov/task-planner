ALTER TABLE task_occurrences ADD COLUMN progress_minutes integer NOT NULL DEFAULT 0 CHECK (progress_minutes >= 0);
