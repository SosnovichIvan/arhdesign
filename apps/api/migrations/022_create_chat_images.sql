BEGIN;

CREATE TABLE IF NOT EXISTS chat_images (
    id UUID PRIMARY KEY,
    scope TEXT NOT NULL CHECK (scope IN ('global', 'project')),
    chat_id UUID NOT NULL,
    global_chat_id UUID GENERATED ALWAYS AS (CASE WHEN scope = 'global' THEN chat_id END) STORED
        REFERENCES global_chats(id) ON DELETE CASCADE,
    project_chat_id UUID GENERATED ALWAYS AS (CASE WHEN scope = 'project' THEN chat_id END) STORED
        REFERENCES chats(id) ON DELETE CASCADE,
    uploader_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 240),
    media_type TEXT NOT NULL CHECK (media_type IN ('image/jpeg', 'image/png', 'image/gif')),
    size_bytes BIGINT NOT NULL CHECK (size_bytes BETWEEN 1 AND 10485760),
    width INTEGER NOT NULL CHECK (width BETWEEN 1 AND 12000),
    height INTEGER NOT NULL CHECK (height BETWEEN 1 AND 12000),
    content_hash BYTEA NOT NULL CHECK (octet_length(content_hash) = 32),
    content BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    pinned BOOLEAN NOT NULL DEFAULT false,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chat_images_content_state CHECK (
        (deleted_at IS NOT NULL AND content IS NULL AND NOT pinned)
        OR (deleted_at IS NULL AND content IS NOT NULL AND octet_length(content) = size_bytes)
    ),
    CONSTRAINT chat_images_pixel_count CHECK (width * height <= 40000000)
);

CREATE INDEX IF NOT EXISTS chat_images_chat_order_idx
    ON chat_images (scope, chat_id, pinned DESC, created_at DESC, id DESC);

COMMIT;
