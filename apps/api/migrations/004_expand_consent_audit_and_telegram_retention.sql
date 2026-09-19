ALTER TABLE contact_consents
    ADD COLUMN IF NOT EXISTS document_sha256 TEXT NOT NULL DEFAULT 'legacy',
    ADD COLUMN IF NOT EXISTS consent_granted BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS consent_method TEXT NOT NULL DEFAULT 'legacy',
    ADD COLUMN IF NOT EXISTS consent_text TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_url TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS telegram_notification_receipts (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    message_id BIGINT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delete_after TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT telegram_notification_receipts_chat_message_unique UNIQUE (chat_id, message_id)
);

CREATE INDEX IF NOT EXISTS telegram_notification_receipts_due_idx
    ON telegram_notification_receipts (delete_after)
    WHERE deleted_at IS NULL;
