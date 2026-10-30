CREATE TABLE IF NOT EXISTS chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    kind TEXT NOT NULL,
    context_type TEXT,
    context_id UUID,
    name TEXT,
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT chats_id_project_unique UNIQUE (id, project_id),
    CONSTRAINT chats_kind_check CHECK (kind IN ('project', 'context')),
    CONSTRAINT chats_context_semantics CHECK (
        (kind = 'project' AND context_type IS NULL AND context_id IS NULL AND name IS NULL)
        OR
        (kind = 'context' AND context_type IS NOT NULL AND context_id IS NOT NULL AND name IS NOT NULL)
    ),
    CONSTRAINT chats_context_type_check CHECK (context_type IS NULL OR context_type IN ('task', 'material', 'expense', 'event')),
    CONSTRAINT chats_name_length CHECK (name IS NULL OR char_length(name) BETWEEN 2 AND 200)
);

DO $$
BEGIN
    IF to_regclass('chat_memberships') IS NULL THEN
        CREATE UNIQUE INDEX IF NOT EXISTS chats_one_project_channel_idx
            ON chats (project_id)
            WHERE kind = 'project' AND archived_at IS NULL;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS chats_one_context_channel_idx
    ON chats (project_id, context_type, context_id)
    WHERE kind = 'context' AND archived_at IS NULL;

CREATE INDEX IF NOT EXISTS chats_project_created_idx
    ON chats (project_id, created_at, id)
    WHERE archived_at IS NULL;

DO $$
BEGIN
    IF to_regclass('chat_memberships') IS NULL THEN
        INSERT INTO chats (project_id, kind, name, created_by_user_id, created_at)
        SELECT project.id, 'project',
               CASE WHEN EXISTS (
                   SELECT 1 FROM information_schema.columns
                   WHERE table_schema = current_schema() AND table_name = 'chats' AND column_name = 'version'
               ) THEN 'Общий чат проекта' ELSE NULL END,
               project.created_by_user_id, project.created_at
        FROM projects project
        WHERE project.archived_at IS NULL
        ON CONFLICT (project_id) WHERE kind = 'project' AND archived_at IS NULL DO NOTHING;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL,
    project_id UUID NOT NULL,
    author_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    client_message_id UUID NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    edited_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    deleted_by_user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT chat_messages_chat_project_fk FOREIGN KEY (chat_id, project_id)
        REFERENCES chats(id, project_id) ON DELETE RESTRICT,
    CONSTRAINT chat_messages_client_id_unique UNIQUE (chat_id, client_message_id),
    CONSTRAINT chat_messages_body_length CHECK (char_length(btrim(body)) BETWEEN 1 AND 5000),
    CONSTRAINT chat_messages_version_positive CHECK (version > 0),
    CONSTRAINT chat_messages_edit_semantics CHECK (edited_at IS NULL OR edited_at >= created_at),
    CONSTRAINT chat_messages_delete_semantics CHECK ((deleted_at IS NULL) = (deleted_by_user_id IS NULL))
);

CREATE INDEX IF NOT EXISTS chat_messages_page_idx
    ON chat_messages (chat_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS chat_messages_project_created_idx
    ON chat_messages (project_id, created_at DESC, id DESC);
