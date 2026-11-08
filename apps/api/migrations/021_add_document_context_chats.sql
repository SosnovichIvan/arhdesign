BEGIN;

ALTER TABLE chats DROP CONSTRAINT IF EXISTS chats_context_type_check;
ALTER TABLE chats ADD CONSTRAINT chats_context_type_check
    CHECK (context_type IS NULL OR context_type IN ('task', 'material', 'expense', 'event', 'document'));

COMMIT;
