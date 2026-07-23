CREATE TABLE topics (
    id text PRIMARY KEY,
    name text NOT NULL UNIQUE,
    description text NULL,
    is_archived boolean NOT NULL DEFAULT 0,
    parent_id text NULL REFERENCES topics(id) ON DELETE SET NULL,
    created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX topics_parent_idx ON topics(parent_id);

CREATE TABLE tasks (
    id text PRIMARY KEY,
    topic_id text NULL REFERENCES topics(id) ON DELETE SET NULL,
    title text NOT NULL,
    description text NULL,
    source text NULL,
    external_id text NULL,
    recurrence_kind text NOT NULL CHECK (recurrence_kind IN (
        'once', 'daily', 'weekdays', 'weekends', 'days_of_week',
        'every_n_days', 'every_n_weeks', 'monthly', 'spaced_repetition'
    )),
    recurrence_params text NULL,
    start_date text NOT NULL,
    due text NULL,
    start_time_minutes integer NULL,
    estimated_duration_minutes integer NULL,
    effort_minutes integer NULL CHECK (effort_minutes IS NULL OR effort_minutes > 0),
    all_day boolean NOT NULL DEFAULT 0,
    priority integer NOT NULL DEFAULT 0,
    progress text NOT NULL DEFAULT 'needs_action' CHECK (progress IN ('needs_action', 'in_process', 'completed', 'cancelled')),
    requires_pomodoro boolean NOT NULL DEFAULT 1,
    is_active boolean NOT NULL DEFAULT 1,
    created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT tasks_source_pair_chk CHECK ((source IS NULL) = (external_id IS NULL)),
    CONSTRAINT tasks_recurrence_params_chk CHECK (
        (recurrence_params IS NULL) = (recurrence_kind IN ('once', 'daily', 'weekdays', 'weekends'))
    ),
    CONSTRAINT tasks_due_chk CHECK (due IS NULL OR recurrence_kind = 'once'),
    CONSTRAINT tasks_start_time_chk CHECK (
        start_time_minutes IS NULL OR (start_time_minutes >= 0 AND start_time_minutes <= 1439)
    ),
    CONSTRAINT tasks_duration_chk CHECK (
        estimated_duration_minutes IS NULL OR estimated_duration_minutes > 0
    ),
    CONSTRAINT tasks_priority_chk CHECK (priority >= 0 AND priority <= 9),
    CONSTRAINT tasks_fixed_time_xor_effort CHECK (
        NOT (start_time_minutes IS NOT NULL AND effort_minutes IS NOT NULL)
    )
);

CREATE UNIQUE INDEX tasks_source_external_uq ON tasks (source, external_id) WHERE source IS NOT NULL;
CREATE INDEX tasks_topic_id_idx ON tasks (topic_id);
CREATE INDEX tasks_start_date_active_idx ON tasks (start_date)
    WHERE is_active AND progress IN ('needs_action', 'in_process');
CREATE INDEX tasks_due_open_idx ON tasks (due)
    WHERE due IS NOT NULL AND progress IN ('needs_action', 'in_process');

CREATE TABLE task_occurrences (
    id text PRIMARY KEY,
    task_id text NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    date text NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'missed', 'rescheduled', 'skipped')),
    series_step integer NULL,
    progress_minutes integer NOT NULL DEFAULT 0 CHECK (progress_minutes >= 0),
    completed_at datetime NULL,
    rescheduled_to text NULL,
    created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT occurrences_completed_at_chk CHECK (
        status <> 'completed' OR completed_at IS NOT NULL
    )
);

CREATE UNIQUE INDEX occurrences_task_date_uq ON task_occurrences (task_id, date);
CREATE UNIQUE INDEX occurrences_task_step_completed_uq ON task_occurrences (task_id, series_step)
    WHERE status = 'completed';
CREATE INDEX occurrences_date_status_idx ON task_occurrences (date, status);
CREATE INDEX occurrences_task_status_idx ON task_occurrences (task_id, status);

CREATE TABLE day_plans (
    id text PRIMARY KEY,
    date text NOT NULL UNIQUE,
    committed_at datetime NULL,
    committed_by text NULL CHECK (committed_by IS NULL OR committed_by IN ('app', 'bot', 'agent')),
    created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT day_plans_committed_by_chk CHECK (committed_at IS NULL OR committed_by IS NOT NULL)
);

CREATE TABLE day_plan_items (
    id text PRIMARY KEY,
    plan_id text NOT NULL REFERENCES day_plans(id) ON DELETE CASCADE,
    occurrence_id text NOT NULL REFERENCES task_occurrences(id) ON DELETE CASCADE,
    planned_start_minutes integer NULL,
    position integer NOT NULL,
    created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT plan_items_start_chk CHECK (
        planned_start_minutes IS NULL OR (planned_start_minutes >= 0 AND planned_start_minutes <= 1439)
    ),
    CONSTRAINT plan_items_plan_occurrence_uq UNIQUE (plan_id, occurrence_id)
);

CREATE INDEX plan_items_plan_position_idx ON day_plan_items (plan_id, position);

CREATE TABLE events_outbox (
    id text PRIMARY KEY,
    event_type text NOT NULL,
    aggregate_type text NOT NULL,
    aggregate_id text NOT NULL,
    payload text NOT NULL,
    created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at datetime NULL,
    attempts integer NOT NULL DEFAULT 0,
    last_error text NULL
);

CREATE INDEX events_outbox_unpublished_idx ON events_outbox (created_at) WHERE published_at IS NULL;
