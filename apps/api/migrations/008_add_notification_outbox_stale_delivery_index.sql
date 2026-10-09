BEGIN;

CREATE INDEX IF NOT EXISTS notification_outbox_stale_delivery_idx
    ON notification_outbox (channel, locked_at, id)
    WHERE state = 'delivering';

COMMIT;
