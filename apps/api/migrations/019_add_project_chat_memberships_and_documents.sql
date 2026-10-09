BEGIN;

DROP INDEX IF EXISTS chats_one_project_channel_idx;

ALTER TABLE chats DROP CONSTRAINT IF EXISTS chats_context_semantics;
ALTER TABLE chats DROP CONSTRAINT IF EXISTS chats_version_positive;
UPDATE chats SET name = 'Общий чат проекта' WHERE kind = 'project' AND name IS NULL;
ALTER TABLE chats ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE chats ADD CONSTRAINT chats_version_positive CHECK (version > 0);
ALTER TABLE chats ADD CONSTRAINT chats_context_semantics CHECK (
    (kind = 'project' AND context_type IS NULL AND context_id IS NULL AND name IS NOT NULL)
    OR
    (kind = 'context' AND context_type IS NOT NULL AND context_id IS NOT NULL AND name IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS chat_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL,
    project_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    added_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    removed_at TIMESTAMPTZ,
    removed_by_user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT chat_memberships_chat_project_fk FOREIGN KEY (chat_id, project_id)
        REFERENCES chats(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT chat_memberships_version_positive CHECK (version > 0),
    CONSTRAINT chat_memberships_remove_semantics CHECK ((removed_at IS NULL) = (removed_by_user_id IS NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS chat_memberships_active_user_idx
    ON chat_memberships (chat_id, user_id) WHERE removed_at IS NULL;
CREATE INDEX IF NOT EXISTS chat_memberships_user_project_idx
    ON chat_memberships (user_id, project_id, chat_id) WHERE removed_at IS NULL;

INSERT INTO chat_memberships (chat_id, project_id, user_id, added_by_user_id, joined_at)
SELECT chat.id, chat.project_id, chat.created_by_user_id, chat.created_by_user_id, chat.created_at
FROM chats chat
JOIN users ON users.id = chat.created_by_user_id AND users.status = 'active'
WHERE chat.archived_at IS NULL
ON CONFLICT (chat_id, user_id) WHERE removed_at IS NULL DO NOTHING;

INSERT INTO chat_memberships (chat_id, project_id, user_id, added_by_user_id, joined_at)
SELECT chat.id, chat.project_id, membership.user_id, chat.created_by_user_id,
       GREATEST(chat.created_at, membership.joined_at)
FROM chats chat
JOIN project_memberships membership ON membership.project_id = chat.project_id AND membership.revoked_at IS NULL
WHERE chat.archived_at IS NULL
ON CONFLICT (chat_id, user_id) WHERE removed_at IS NULL DO NOTHING;

CREATE TABLE IF NOT EXISTS project_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    media_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    content BYTEA NOT NULL,
    uploaded_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    deleted_by_user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT project_documents_name_length CHECK (char_length(btrim(name)) BETWEEN 1 AND 240),
    CONSTRAINT project_documents_media_type_length CHECK (char_length(media_type) BETWEEN 3 AND 127),
    CONSTRAINT project_documents_size CHECK (size_bytes BETWEEN 1 AND 10485760),
    CONSTRAINT project_documents_content_size CHECK (octet_length(content) = size_bytes),
    CONSTRAINT project_documents_delete_semantics CHECK ((deleted_at IS NULL) = (deleted_by_user_id IS NULL)),
    CONSTRAINT project_documents_version_positive CHECK (version > 0)
);

CREATE INDEX IF NOT EXISTS project_documents_project_created_idx
    ON project_documents (project_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;

COMMIT;
