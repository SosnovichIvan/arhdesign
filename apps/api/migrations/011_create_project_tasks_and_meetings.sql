BEGIN;

CREATE TABLE IF NOT EXISTS project_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    created_by_user_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'new',
    due_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT project_tasks_title_length CHECK (char_length(title) BETWEEN 2 AND 200),
    CONSTRAINT project_tasks_description_length CHECK (description IS NULL OR char_length(description) <= 5000),
    CONSTRAINT project_tasks_status_check CHECK (status IN ('new', 'in_progress', 'review', 'changes_requested', 'accepted')),
    CONSTRAINT project_tasks_version_positive CHECK (version > 0)
);

CREATE INDEX IF NOT EXISTS project_tasks_upcoming_idx
    ON project_tasks (project_id, due_at, id);

CREATE TABLE IF NOT EXISTS project_meetings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    created_by_user_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    description TEXT,
    location TEXT,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT project_meetings_title_length CHECK (char_length(title) BETWEEN 2 AND 200),
    CONSTRAINT project_meetings_description_length CHECK (description IS NULL OR char_length(description) <= 5000),
    CONSTRAINT project_meetings_location_length CHECK (location IS NULL OR char_length(location) <= 500),
    CONSTRAINT project_meetings_time_order CHECK (ends_at > starts_at),
    CONSTRAINT project_meetings_version_positive CHECK (version > 0)
);

CREATE INDEX IF NOT EXISTS project_meetings_upcoming_idx
    ON project_meetings (project_id, starts_at, id);

COMMIT;
