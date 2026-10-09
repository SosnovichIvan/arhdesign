BEGIN;

-- Telegram confirmation activates an account without claiming that its email
-- address was verified. Pending accounts still cannot carry a verification
-- timestamp, while active and disabled accounts preserve the channel-specific
-- verification state.
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_email_verification_state;

ALTER TABLE users
    ADD CONSTRAINT users_email_verification_state CHECK (
        status <> 'pending_verification' OR email_verified_at IS NULL
    );

CREATE TABLE IF NOT EXISTS telegram_bot_auth_states (
    chat_id BIGINT PRIMARY KEY,
    flow TEXT NOT NULL,
    step TEXT NOT NULL,
    candidate_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    request_id TEXT NOT NULL,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT telegram_bot_auth_states_flow_check CHECK (
        flow IN ('registration_confirmation', 'account_binding', 'password_recovery')
    ),
    CONSTRAINT telegram_bot_auth_states_step_check CHECK (
        step IN ('awaiting_login', 'awaiting_password')
    ),
    CONSTRAINT telegram_bot_auth_states_request_id_length CHECK (
        char_length(request_id) BETWEEN 8 AND 100
    ),
    CONSTRAINT telegram_bot_auth_states_attempts_check CHECK (
        failed_attempts BETWEEN 0 AND 5
    ),
    CONSTRAINT telegram_bot_auth_states_expiry_check CHECK (
        expires_at > started_at AND expires_at <= started_at + INTERVAL '5 minutes'
    ),
    CONSTRAINT telegram_bot_auth_states_candidate_step_check CHECK (
        step <> 'awaiting_login' OR candidate_user_id IS NULL
    )
);

CREATE INDEX IF NOT EXISTS telegram_bot_auth_states_expiry_idx
    ON telegram_bot_auth_states (expires_at, chat_id);

CREATE TABLE IF NOT EXISTS telegram_account_bindings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id BIGINT NOT NULL,
    chat_username TEXT,
    verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT telegram_account_bindings_username_length CHECK (
        chat_username IS NULL OR char_length(chat_username) BETWEEN 1 AND 64
    ),
    CONSTRAINT telegram_account_bindings_revoke_pair CHECK (
        (revoked_at IS NULL AND revoke_reason IS NULL)
        OR (revoked_at IS NOT NULL AND revoke_reason IS NOT NULL)
    ),
    CONSTRAINT telegram_account_bindings_revoke_order CHECK (
        revoked_at IS NULL OR revoked_at >= verified_at
    ),
    CONSTRAINT telegram_account_bindings_revoke_reason_format CHECK (
        revoke_reason IS NULL OR revoke_reason ~ '^[a-z][a-z0-9_]{1,63}$'
    ),
    CONSTRAINT telegram_account_bindings_version_positive CHECK (version > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS telegram_account_bindings_active_user_uidx
    ON telegram_account_bindings (user_id)
    WHERE revoked_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS telegram_account_bindings_active_chat_uidx
    ON telegram_account_bindings (chat_id)
    WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS telegram_account_bindings_active_chat_lookup_idx
    ON telegram_account_bindings (chat_id, user_id)
    WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS account_notification_preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    verification_channel TEXT NOT NULL DEFAULT 'email',
    recovery_channel TEXT NOT NULL DEFAULT 'email',
    telegram_events_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT account_notification_preferences_verification_channel_check CHECK (
        verification_channel IN ('email', 'telegram')
    ),
    CONSTRAINT account_notification_preferences_recovery_channel_check CHECK (
        recovery_channel IN ('email', 'telegram')
    )
);

COMMIT;
