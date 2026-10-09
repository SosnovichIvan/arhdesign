BEGIN;

ALTER TABLE project_tasks
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

ALTER TABLE project_tasks
    DROP CONSTRAINT IF EXISTS project_tasks_actual_time_order;

ALTER TABLE project_tasks
    ADD CONSTRAINT project_tasks_actual_time_order
    CHECK (completed_at IS NULL OR (started_at IS NOT NULL AND completed_at >= started_at));

CREATE TABLE IF NOT EXISTS project_task_assignees (
    task_id UUID NOT NULL REFERENCES project_tasks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    assigned_by_user_id UUID NOT NULL REFERENCES users(id),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (task_id, user_id)
);

CREATE INDEX IF NOT EXISTS project_task_assignees_user_task_idx
    ON project_task_assignees (user_id, task_id);

COMMIT;
