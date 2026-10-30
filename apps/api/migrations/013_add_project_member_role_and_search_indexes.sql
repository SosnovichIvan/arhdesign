BEGIN;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'project_memberships_known_role_check'
          AND conrelid = 'project_memberships'::regclass
    ) THEN
        ALTER TABLE project_memberships
            ADD CONSTRAINT project_memberships_known_role_check
            CHECK (project_role IN ('customer', 'project_admin', 'executor')) NOT VALID;
    END IF;
END
$$;

ALTER TABLE project_memberships
    VALIDATE CONSTRAINT project_memberships_known_role_check;

CREATE INDEX IF NOT EXISTS users_active_login_prefix_idx
    ON users (login_normalized text_pattern_ops)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS users_active_email_prefix_idx
    ON users (email_normalized text_pattern_ops)
    WHERE status = 'active';

COMMIT;
