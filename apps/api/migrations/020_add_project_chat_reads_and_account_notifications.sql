BEGIN;

ALTER TABLE chat_memberships
    ADD COLUMN IF NOT EXISTS last_read_at TIMESTAMPTZ;

UPDATE chat_memberships
SET last_read_at = joined_at
WHERE last_read_at IS NULL;

ALTER TABLE chat_memberships
    ALTER COLUMN last_read_at SET DEFAULT now(),
    ALTER COLUMN last_read_at SET NOT NULL;

CREATE TABLE IF NOT EXISTS account_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    actor_user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    event_type TEXT NOT NULL,
    event_id TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    href TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at TIMESTAMPTZ,
    CONSTRAINT account_notifications_event_type_length CHECK (char_length(event_type) BETWEEN 3 AND 120),
    CONSTRAINT account_notifications_event_id_length CHECK (char_length(event_id) BETWEEN 1 AND 200),
    CONSTRAINT account_notifications_title_length CHECK (char_length(title) BETWEEN 1 AND 200),
    CONSTRAINT account_notifications_body_length CHECK (char_length(body) BETWEEN 1 AND 5000),
    CONSTRAINT account_notifications_href_length CHECK (char_length(href) BETWEEN 1 AND 1000),
    CONSTRAINT account_notifications_recipient_event_unique UNIQUE (recipient_user_id, event_type, event_id)
);

CREATE INDEX IF NOT EXISTS account_notifications_recipient_unread_created_idx
    ON account_notifications (recipient_user_id, created_at DESC, id DESC)
    WHERE read_at IS NULL;

CREATE INDEX IF NOT EXISTS account_notifications_recipient_created_idx
    ON account_notifications (recipient_user_id, created_at DESC, id DESC);

COMMIT;
