BEGIN;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_global_role_check;
ALTER TABLE users
    ADD CONSTRAINT users_global_role_check
    CHECK (global_role IS NULL OR global_role IN ('super_admin', 'technical_admin'));

CREATE UNIQUE INDEX IF NOT EXISTS users_one_active_technical_admin_uidx
    ON users (global_role)
    WHERE status = 'active' AND global_role = 'technical_admin';

CREATE TABLE IF NOT EXISTS global_chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL,
    name TEXT,
    direct_key TEXT,
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT global_chats_kind_check CHECK (kind IN ('direct', 'group')),
    CONSTRAINT global_chats_shape_check CHECK (
        (kind = 'direct' AND name IS NULL AND direct_key IS NOT NULL)
        OR (kind = 'group' AND name IS NOT NULL AND direct_key IS NULL)
    ),
    CONSTRAINT global_chats_name_length CHECK (name IS NULL OR char_length(btrim(name)) BETWEEN 2 AND 200),
    CONSTRAINT global_chats_direct_key_format CHECK (direct_key IS NULL OR direct_key ~ '^[0-9a-f-]{36}:[0-9a-f-]{36}$'),
    CONSTRAINT global_chats_version_positive CHECK (version > 0),
    CONSTRAINT global_chats_activity_order CHECK (last_activity_at >= created_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS global_chats_direct_pair_uidx
    ON global_chats (direct_key)
    WHERE kind = 'direct' AND archived_at IS NULL;
CREATE INDEX IF NOT EXISTS global_chats_activity_idx
    ON global_chats (last_activity_at DESC, id DESC)
    WHERE archived_at IS NULL;

CREATE TABLE IF NOT EXISTS global_chat_members (
    chat_id UUID NOT NULL REFERENCES global_chats(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    added_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    last_read_at TIMESTAMPTZ,
    PRIMARY KEY (chat_id, user_id),
    CONSTRAINT global_chat_members_leave_order CHECK (left_at IS NULL OR left_at >= joined_at),
    CONSTRAINT global_chat_members_read_order CHECK (last_read_at IS NULL OR last_read_at >= joined_at)
);

CREATE INDEX IF NOT EXISTS global_chat_members_user_inbox_idx
    ON global_chat_members (user_id, chat_id)
    WHERE left_at IS NULL;
CREATE INDEX IF NOT EXISTS global_chat_members_chat_active_idx
    ON global_chat_members (chat_id, user_id)
    WHERE left_at IS NULL;

CREATE TABLE IF NOT EXISTS global_chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL REFERENCES global_chats(id) ON DELETE RESTRICT,
    author_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    client_message_id UUID NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    edited_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    deleted_by_user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT global_chat_messages_client_id_unique UNIQUE (chat_id, client_message_id),
    CONSTRAINT global_chat_messages_body_length CHECK (char_length(btrim(body)) BETWEEN 1 AND 5000),
    CONSTRAINT global_chat_messages_version_positive CHECK (version > 0),
    CONSTRAINT global_chat_messages_edit_order CHECK (edited_at IS NULL OR edited_at >= created_at),
    CONSTRAINT global_chat_messages_delete_semantics CHECK ((deleted_at IS NULL) = (deleted_by_user_id IS NULL))
);

CREATE INDEX IF NOT EXISTS global_chat_messages_page_idx
    ON global_chat_messages (chat_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS technical_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL,
    category TEXT,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    request_id TEXT NOT NULL,
    method TEXT,
    route TEXT NOT NULL,
    status_code INTEGER,
    error_code TEXT,
    fingerprint TEXT NOT NULL,
    summary TEXT NOT NULL,
    user_message TEXT,
    occurrence_count INTEGER NOT NULL DEFAULT 1,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retention_until TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '90 days'),
    CONSTRAINT technical_reports_kind_check CHECK (kind IN ('backend_error', 'frontend_error', 'feedback')),
    CONSTRAINT technical_reports_category_check CHECK (
        (kind = 'feedback' AND category IN ('complaint', 'suggestion'))
        OR (kind <> 'feedback' AND category IS NULL)
    ),
    CONSTRAINT technical_reports_request_id_length CHECK (char_length(request_id) BETWEEN 8 AND 100),
    CONSTRAINT technical_reports_method_format CHECK (method IS NULL OR method ~ '^[A-Z]{3,10}$'),
    CONSTRAINT technical_reports_route_length CHECK (char_length(route) BETWEEN 1 AND 300),
    CONSTRAINT technical_reports_status_range CHECK (status_code IS NULL OR status_code BETWEEN 400 AND 599),
    CONSTRAINT technical_reports_error_code_format CHECK (error_code IS NULL OR error_code ~ '^[a-z][a-z0-9_]{1,63}$'),
    CONSTRAINT technical_reports_fingerprint_length CHECK (char_length(fingerprint) BETWEEN 16 AND 128),
    CONSTRAINT technical_reports_summary_length CHECK (char_length(summary) BETWEEN 1 AND 500),
    CONSTRAINT technical_reports_user_message_length CHECK (user_message IS NULL OR char_length(btrim(user_message)) BETWEEN 5 AND 2000),
    CONSTRAINT technical_reports_count_positive CHECK (occurrence_count > 0),
    CONSTRAINT technical_reports_seen_order CHECK (last_seen_at >= first_seen_at),
    CONSTRAINT technical_reports_retention_order CHECK (retention_until > first_seen_at)
);

CREATE INDEX IF NOT EXISTS technical_reports_fingerprint_recent_idx
    ON technical_reports (fingerprint, last_seen_at DESC)
    WHERE kind <> 'feedback';
CREATE INDEX IF NOT EXISTS technical_reports_retention_idx
    ON technical_reports (retention_until, id);
CREATE INDEX IF NOT EXISTS technical_reports_actor_created_idx
    ON technical_reports (actor_user_id, first_seen_at DESC)
    WHERE actor_user_id IS NOT NULL;

COMMIT;
