CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE recurrence_kind AS ENUM (
    'once', 'daily', 'weekdays', 'weekends', 'days_of_week',
    'every_n_days', 'every_n_weeks', 'monthly', 'spaced_repetition'
);

CREATE TYPE task_progress AS ENUM ('needs_action', 'in_process', 'completed', 'cancelled');

CREATE TYPE occurrence_status AS ENUM ('pending', 'completed', 'missed', 'rescheduled', 'skipped');

CREATE TYPE actor_kind AS ENUM ('app', 'bot', 'agent');

CREATE TABLE topics (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    description text NULL,
    is_archived boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id uuid NULL REFERENCES topics(id) ON DELETE SET NULL,
    title text NOT NULL,
    description text NULL,
    source text NULL,
    external_id text NULL,
    recurrence_kind recurrence_kind NOT NULL,
    recurrence_params jsonb NULL,
    start_date date NOT NULL,
    due date NULL,
    start_time_minutes integer NULL,
    estimated_duration_minutes integer NULL,
    all_day boolean NOT NULL DEFAULT false,
    priority integer NOT NULL DEFAULT 0,
    progress task_progress NOT NULL DEFAULT 'needs_action',
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
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
    CONSTRAINT tasks_priority_chk CHECK (priority >= 0 AND priority <= 9)
);

CREATE UNIQUE INDEX tasks_source_external_uq ON tasks (source, external_id) WHERE source IS NOT NULL;
CREATE INDEX tasks_topic_id_idx ON tasks (topic_id);
CREATE INDEX tasks_start_date_active_idx ON tasks (start_date)
    WHERE is_active AND progress IN ('needs_action', 'in_process');
CREATE INDEX tasks_due_open_idx ON tasks (due)
    WHERE due IS NOT NULL AND progress IN ('needs_action', 'in_process');

CREATE TABLE task_occurrences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    date date NOT NULL,
    status occurrence_status NOT NULL DEFAULT 'pending',
    series_step integer NULL,
    completed_at timestamptz NULL,
    rescheduled_to date NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
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
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    date date NOT NULL UNIQUE,
    committed_at timestamptz NULL,
    committed_by actor_kind NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT day_plans_committed_by_chk CHECK (committed_at IS NULL OR committed_by IS NOT NULL)
);

CREATE TABLE day_plan_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id uuid NOT NULL REFERENCES day_plans(id) ON DELETE CASCADE,
    occurrence_id uuid NOT NULL REFERENCES task_occurrences(id) ON DELETE CASCADE,
    planned_start_minutes integer NULL,
    position integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT plan_items_start_chk CHECK (
        planned_start_minutes IS NULL OR (planned_start_minutes >= 0 AND planned_start_minutes <= 1439)
    ),
    CONSTRAINT plan_items_plan_occurrence_uq UNIQUE (plan_id, occurrence_id)
);

CREATE INDEX plan_items_plan_position_idx ON day_plan_items (plan_id, position);

CREATE TABLE events_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type text NOT NULL,
    aggregate_type text NOT NULL,
    aggregate_id uuid NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz NULL,
    attempts integer NOT NULL DEFAULT 0,
    last_error text NULL
);

CREATE INDEX events_outbox_unpublished_idx ON events_outbox (created_at) WHERE published_at IS NULL;
