BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS professional_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT professional_roles_code_format CHECK (code ~ '^[a-z][a-z0-9_]{1,63}$'),
    CONSTRAINT professional_roles_code_unique UNIQUE (code)
);

INSERT INTO professional_roles (code, name)
VALUES
    ('customer', 'Заказчик'),
    ('designer', 'Дизайнер'),
    ('foreman', 'Прораб'),
    ('architect', 'Архитектор')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    login TEXT NOT NULL,
    login_normalized TEXT NOT NULL,
    email TEXT NOT NULL,
    email_normalized TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending_verification',
    global_role TEXT,
    email_verified_at TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,
    security_version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT users_login_length CHECK (char_length(login) BETWEEN 3 AND 64),
    CONSTRAINT users_email_length CHECK (char_length(email) BETWEEN 3 AND 254),
    CONSTRAINT users_status_check CHECK (status IN ('pending_verification', 'active', 'disabled')),
    CONSTRAINT users_global_role_check CHECK (global_role IS NULL OR global_role = 'super_admin'),
    CONSTRAINT users_email_verification_state CHECK (
        (status = 'pending_verification' AND email_verified_at IS NULL)
        OR (status = 'active' AND email_verified_at IS NOT NULL)
        OR status = 'disabled'
    ),
    CONSTRAINT users_security_version_positive CHECK (security_version > 0),
    CONSTRAINT users_version_positive CHECK (version > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS users_login_normalized_uidx
    ON users (login_normalized);
CREATE UNIQUE INDEX IF NOT EXISTS users_email_normalized_uidx
    ON users (email_normalized);
CREATE INDEX IF NOT EXISTS users_status_created_at_idx
    ON users (status, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS users_active_super_admin_idx
    ON users (id)
    WHERE status = 'active' AND global_role = 'super_admin';

CREATE TABLE IF NOT EXISTS profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    professional_role_id UUID NOT NULL REFERENCES professional_roles(id),
    first_name TEXT NOT NULL,
    last_name TEXT,
    middle_name TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT profiles_first_name_length CHECK (char_length(first_name) BETWEEN 1 AND 100),
    CONSTRAINT profiles_last_name_length CHECK (last_name IS NULL OR char_length(last_name) BETWEEN 1 AND 100),
    CONSTRAINT profiles_middle_name_length CHECK (middle_name IS NULL OR char_length(middle_name) BETWEEN 1 AND 100)
);

CREATE TABLE IF NOT EXISTS credentials (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    algorithm TEXT NOT NULL,
    parameters JSONB NOT NULL,
    hash_version INTEGER NOT NULL,
    password_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT credentials_algorithm_check CHECK (algorithm = 'argon2id'),
    CONSTRAINT credentials_hash_version_positive CHECK (hash_version > 0),
    CONSTRAINT credentials_parameters_object CHECK (jsonb_typeof(parameters) = 'object')
);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_id UUID NOT NULL,
    access_token_hash BYTEA NOT NULL,
    refresh_token_hash BYTEA NOT NULL,
    csrf_token_hash BYTEA NOT NULL,
    hash_key_version INTEGER NOT NULL,
    captured_security_version BIGINT NOT NULL,
    replaced_by_session_id UUID REFERENCES sessions(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    idle_expires_at TIMESTAMPTZ NOT NULL,
    absolute_expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT,
    CONSTRAINT sessions_access_token_hash_unique UNIQUE (access_token_hash),
    CONSTRAINT sessions_refresh_token_hash_unique UNIQUE (refresh_token_hash),
    CONSTRAINT sessions_replaced_by_unique UNIQUE (replaced_by_session_id),
    CONSTRAINT sessions_hash_key_version_positive CHECK (hash_key_version > 0),
    CONSTRAINT sessions_security_version_positive CHECK (captured_security_version > 0),
    CONSTRAINT sessions_expiry_order CHECK (idle_expires_at <= absolute_expires_at),
    CONSTRAINT sessions_revocation_pair CHECK (
        (revoked_at IS NULL AND revoke_reason IS NULL)
        OR (revoked_at IS NOT NULL AND revoke_reason IS NOT NULL)
    ),
    CONSTRAINT sessions_not_self_replaced CHECK (replaced_by_session_id IS NULL OR replaced_by_session_id <> id)
);

CREATE INDEX IF NOT EXISTS sessions_active_user_idx
    ON sessions (user_id, absolute_expires_at)
    WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS sessions_family_idx
    ON sessions (family_id, created_at DESC);

CREATE TABLE IF NOT EXISTS email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    hash_key_version INTEGER NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT email_verification_tokens_hash_unique UNIQUE (token_hash),
    CONSTRAINT email_verification_tokens_key_version_positive CHECK (hash_key_version > 0),
    CONSTRAINT email_verification_tokens_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT email_verification_tokens_terminal_state CHECK (consumed_at IS NULL OR revoked_at IS NULL),
    CONSTRAINT email_verification_tokens_revocation_pair CHECK (
        (revoked_at IS NULL AND revoke_reason IS NULL)
        OR (revoked_at IS NOT NULL AND revoke_reason IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS email_verification_tokens_one_active_per_user_uidx
    ON email_verification_tokens (user_id)
    WHERE consumed_at IS NULL AND revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS email_verification_tokens_expiry_idx
    ON email_verification_tokens (expires_at)
    WHERE consumed_at IS NULL AND revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    hash_key_version INTEGER NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT password_reset_tokens_hash_unique UNIQUE (token_hash),
    CONSTRAINT password_reset_tokens_key_version_positive CHECK (hash_key_version > 0),
    CONSTRAINT password_reset_tokens_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT password_reset_tokens_terminal_state CHECK (consumed_at IS NULL OR revoked_at IS NULL),
    CONSTRAINT password_reset_tokens_revocation_pair CHECK (
        (revoked_at IS NULL AND revoke_reason IS NULL)
        OR (revoked_at IS NOT NULL AND revoke_reason IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS password_reset_tokens_one_active_per_user_uidx
    ON password_reset_tokens (user_id)
    WHERE consumed_at IS NULL AND revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS password_reset_tokens_expiry_idx
    ON password_reset_tokens (expires_at)
    WHERE consumed_at IS NULL AND revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS auth_rate_limit_buckets (
    policy_code TEXT NOT NULL,
    subject_hash BYTEA NOT NULL,
    hash_key_version INTEGER NOT NULL,
    tokens NUMERIC(12,6) NOT NULL,
    last_refill_at TIMESTAMPTZ NOT NULL,
    blocked_until TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (policy_code, subject_hash),
    CONSTRAINT auth_rate_limit_policy_format CHECK (policy_code ~ '^[a-z][a-z0-9_.]{2,63}$'),
    CONSTRAINT auth_rate_limit_key_version_positive CHECK (hash_key_version > 0),
    CONSTRAINT auth_rate_limit_tokens_nonnegative CHECK (tokens >= 0),
    CONSTRAINT auth_rate_limit_expiry_check CHECK (expires_at > last_refill_at)
);

CREATE INDEX IF NOT EXISTS auth_rate_limit_buckets_expiry_idx
    ON auth_rate_limit_buckets (expires_at);

CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_by_user_id UUID NOT NULL REFERENCES users(id),
    customer_user_id UUID REFERENCES users(id),
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    address TEXT,
    status TEXT NOT NULL DEFAULT 'draft',
    planned_start_on DATE,
    planned_finish_on DATE,
    currency_code TEXT NOT NULL DEFAULT 'RUB',
    description TEXT,
    auto_approve_expenses BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT projects_name_length CHECK (char_length(name) BETWEEN 2 AND 200),
    CONSTRAINT projects_type_length CHECK (char_length(type) BETWEEN 2 AND 100),
    CONSTRAINT projects_address_length CHECK (address IS NULL OR char_length(address) <= 500),
    CONSTRAINT projects_status_check CHECK (status IN ('draft', 'active', 'paused', 'completed', 'pending_deletion', 'archived')),
    CONSTRAINT projects_date_order CHECK (planned_finish_on IS NULL OR planned_start_on IS NULL OR planned_finish_on >= planned_start_on),
    CONSTRAINT projects_currency_code_check CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT projects_description_length CHECK (description IS NULL OR char_length(description) <= 5000),
    CONSTRAINT projects_archive_state CHECK ((status = 'archived') = (archived_at IS NOT NULL)),
    CONSTRAINT projects_version_positive CHECK (version > 0)
);

CREATE INDEX IF NOT EXISTS projects_created_by_idx
    ON projects (created_by_user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS projects_customer_idx
    ON projects (customer_user_id, created_at DESC, id DESC)
    WHERE customer_user_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS project_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    project_role TEXT NOT NULL,
    privileges JSONB NOT NULL DEFAULT '{}'::jsonb,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT project_memberships_role_format CHECK (project_role ~ '^[a-z][a-z0-9_]{1,63}$'),
    CONSTRAINT project_memberships_privileges_object CHECK (jsonb_typeof(privileges) = 'object'),
    CONSTRAINT project_memberships_version_positive CHECK (version > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS project_memberships_one_active_role_per_user_uidx
    ON project_memberships (project_id, user_id, project_role)
    WHERE revoked_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS project_memberships_one_active_customer_uidx
    ON project_memberships (project_id)
    WHERE project_role = 'customer' AND revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS project_memberships_active_user_projects_idx
    ON project_memberships (user_id, project_id)
    WHERE revoked_at IS NULL;

CREATE OR REPLACE FUNCTION assert_project_customer_membership_consistent(checked_project_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    expected_customer_id UUID;
    actual_customer_id UUID;
BEGIN
    SELECT customer_user_id
      INTO expected_customer_id
      FROM projects
     WHERE id = checked_project_id;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    SELECT user_id
      INTO actual_customer_id
      FROM project_memberships
     WHERE project_id = checked_project_id
       AND project_role = 'customer'
       AND revoked_at IS NULL;

    IF expected_customer_id IS DISTINCT FROM actual_customer_id THEN
        RAISE EXCEPTION 'project customer and active customer membership are inconsistent'
            USING ERRCODE = '23514';
    END IF;

    RETURN;
END;
$$;

CREATE OR REPLACE FUNCTION enforce_project_customer_membership()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_TABLE_NAME = 'projects' THEN
        PERFORM assert_project_customer_membership_consistent(COALESCE(NEW.id, OLD.id));
        RETURN NULL;
    END IF;

    IF TG_OP <> 'INSERT' THEN
        PERFORM assert_project_customer_membership_consistent(OLD.project_id);
    END IF;

    IF TG_OP <> 'DELETE'
       AND (TG_OP = 'INSERT' OR NEW.project_id IS DISTINCT FROM OLD.project_id) THEN
        PERFORM assert_project_customer_membership_consistent(NEW.project_id);
    END IF;

    RETURN NULL;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM pg_trigger
         WHERE tgname = 'projects_customer_membership_consistency'
           AND tgrelid = 'projects'::regclass
    ) THEN
        CREATE CONSTRAINT TRIGGER projects_customer_membership_consistency
        AFTER INSERT OR UPDATE OF customer_user_id ON projects
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION enforce_project_customer_membership();
    END IF;

    IF NOT EXISTS (
        SELECT 1
          FROM pg_trigger
         WHERE tgname = 'project_memberships_customer_consistency'
           AND tgrelid = 'project_memberships'::regclass
    ) THEN
        CREATE CONSTRAINT TRIGGER project_memberships_customer_consistency
        AFTER INSERT OR UPDATE OF project_id, user_id, project_role, revoked_at OR DELETE ON project_memberships
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION enforce_project_customer_membership();
    END IF;
END;
$$;

ALTER TABLE telegram_subscribers
    ADD COLUMN IF NOT EXISTS user_id UUID,
    ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS chat_type TEXT,
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conname = 'telegram_subscribers_user_id_fkey'
           AND conrelid = 'telegram_subscribers'::regclass
    ) THEN
        ALTER TABLE telegram_subscribers
            ADD CONSTRAINT telegram_subscribers_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conname = 'telegram_subscribers_chat_type_check'
           AND conrelid = 'telegram_subscribers'::regclass
    ) THEN
        ALTER TABLE telegram_subscribers
            ADD CONSTRAINT telegram_subscribers_chat_type_check
            CHECK (chat_type IS NULL OR chat_type = 'private') NOT VALID;
        ALTER TABLE telegram_subscribers
            VALIDATE CONSTRAINT telegram_subscribers_chat_type_check;
    END IF;

    IF NOT EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conname = 'telegram_subscribers_version_positive'
           AND conrelid = 'telegram_subscribers'::regclass
    ) THEN
        ALTER TABLE telegram_subscribers
            ADD CONSTRAINT telegram_subscribers_version_positive
            CHECK (version > 0) NOT VALID;
        ALTER TABLE telegram_subscribers
            VALIDATE CONSTRAINT telegram_subscribers_version_positive;
    END IF;
END;
$$;

CREATE UNIQUE INDEX IF NOT EXISTS telegram_subscribers_active_user_uidx
    ON telegram_subscribers (user_id)
    WHERE user_id IS NOT NULL AND unsubscribed_at IS NULL;

COMMIT;
