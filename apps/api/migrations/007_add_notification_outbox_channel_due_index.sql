BEGIN;

CREATE INDEX IF NOT EXISTS notification_outbox_channel_due_idx
    ON notification_outbox (channel, available_at, created_at, id)
    WHERE state IN ('pending', 'retry');

COMMIT;
