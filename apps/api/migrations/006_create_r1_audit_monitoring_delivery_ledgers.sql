BEGIN;

CREATE TABLE IF NOT EXISTS audit_events (
    id BIGSERIAL PRIMARY KEY,
    event_type TEXT NOT NULL,
    result TEXT NOT NULL,
    reason_code TEXT,
    request_id TEXT NOT NULL,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    subject_type TEXT,
    subject_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    session_family_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retention_until TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '365 days'),
    legal_hold_until TIMESTAMPTZ,
    CONSTRAINT audit_events_event_type_format CHECK (event_type ~ '^[a-z][a-z0-9_.]{2,127}$'),
    CONSTRAINT audit_events_result_format CHECK (result ~ '^[a-z][a-z0-9_]{1,63}$'),
    CONSTRAINT audit_events_reason_code_format CHECK (reason_code IS NULL OR reason_code ~ '^[a-z][a-z0-9_]{1,63}$'),
    CONSTRAINT audit_events_request_id_length CHECK (char_length(request_id) BETWEEN 8 AND 100),
    CONSTRAINT audit_events_subject_type_format CHECK (subject_type IS NULL OR subject_type ~ '^[a-z][a-z0-9_]{1,63}$'),
    CONSTRAINT audit_events_metadata_object CHECK (jsonb_typeof(metadata) = 'object'),
    CONSTRAINT audit_events_retention_after_event CHECK (retention_until > occurred_at),
    CONSTRAINT audit_events_legal_hold_after_event CHECK (legal_hold_until IS NULL OR legal_hold_until > occurred_at)
);

CREATE INDEX IF NOT EXISTS audit_events_type_result_occurred_at_idx
    ON audit_events (event_type, result, occurred_at DESC);
CREATE INDEX IF NOT EXISTS audit_events_actor_occurred_at_idx
    ON audit_events (actor_user_id, occurred_at DESC)
    WHERE actor_user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS audit_events_retention_idx
    ON audit_events (retention_until, id)
    WHERE legal_hold_until IS NULL;

CREATE OR REPLACE FUNCTION enforce_audit_event_immutability()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF NEW.id IS DISTINCT FROM OLD.id
           OR NEW.event_type IS DISTINCT FROM OLD.event_type
           OR NEW.result IS DISTINCT FROM OLD.result
           OR NEW.reason_code IS DISTINCT FROM OLD.reason_code
           OR NEW.request_id IS DISTINCT FROM OLD.request_id
           OR NEW.actor_user_id IS DISTINCT FROM OLD.actor_user_id
           OR NEW.subject_type IS DISTINCT FROM OLD.subject_type
           OR NEW.subject_user_id IS DISTINCT FROM OLD.subject_user_id
           OR NEW.project_id IS DISTINCT FROM OLD.project_id
           OR NEW.session_family_id IS DISTINCT FROM OLD.session_family_id
           OR NEW.metadata IS DISTINCT FROM OLD.metadata
           OR NEW.occurred_at IS DISTINCT FROM OLD.occurred_at
           OR NEW.retention_until IS DISTINCT FROM OLD.retention_until THEN
            RAISE EXCEPTION 'audit events are append-only'
                USING ERRCODE = '55000';
        END IF;
        RETURN NEW;
    END IF;

    IF OLD.retention_until > NOW()
       OR (OLD.legal_hold_until IS NOT NULL AND OLD.legal_hold_until > NOW()) THEN
        RAISE EXCEPTION 'audit event retention has not expired'
            USING ERRCODE = '55000';
    END IF;

    RETURN OLD;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM pg_trigger
         WHERE tgname = 'audit_events_immutable_until_retention'
           AND tgrelid = 'audit_events'::regclass
    ) THEN
        CREATE TRIGGER audit_events_immutable_until_retention
        BEFORE UPDATE OR DELETE ON audit_events
        FOR EACH ROW EXECUTE FUNCTION enforce_audit_event_immutability();
    END IF;
END;
$$;

CREATE TABLE IF NOT EXISTS notification_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    channel TEXT NOT NULL,
    message_type TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    payload_ciphertext BYTEA NOT NULL,
    payload_key_version INTEGER NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    state TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    terminal_at TIMESTAMPTZ,
    last_error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT notification_outbox_idempotency_key_unique UNIQUE (idempotency_key),
    CONSTRAINT notification_outbox_channel_check CHECK (channel IN ('email', 'telegram')),
    CONSTRAINT notification_outbox_message_type_format CHECK (message_type ~ '^[a-z][a-z0-9_.]{2,127}$'),
    CONSTRAINT notification_outbox_idempotency_length CHECK (char_length(idempotency_key) BETWEEN 8 AND 300),
    CONSTRAINT notification_outbox_payload_nonempty CHECK (octet_length(payload_ciphertext) > 0),
    CONSTRAINT notification_outbox_key_version_positive CHECK (payload_key_version > 0),
    CONSTRAINT notification_outbox_metadata_object CHECK (jsonb_typeof(metadata) = 'object'),
    CONSTRAINT notification_outbox_state_check CHECK (state IN ('pending', 'delivering', 'retry', 'delivered', 'terminal')),
    CONSTRAINT notification_outbox_attempts_nonnegative CHECK (attempts >= 0),
    CONSTRAINT notification_outbox_state_timestamps CHECK (
        (state = 'delivered' AND delivered_at IS NOT NULL AND terminal_at IS NULL)
        OR (state = 'terminal' AND terminal_at IS NOT NULL AND delivered_at IS NULL)
        OR (state IN ('pending', 'delivering', 'retry') AND delivered_at IS NULL AND terminal_at IS NULL)
    ),
    CONSTRAINT notification_outbox_error_code_format CHECK (last_error_code IS NULL OR last_error_code ~ '^[a-z][a-z0-9_]{1,63}$')
);

CREATE INDEX IF NOT EXISTS notification_outbox_due_idx
    ON notification_outbox (available_at, created_at, id)
    WHERE state IN ('pending', 'retry');
CREATE INDEX IF NOT EXISTS notification_outbox_recipient_idx
    ON notification_outbox (recipient_user_id, created_at DESC)
    WHERE recipient_user_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS monitoring_samples (
    id BIGSERIAL PRIMARY KEY,
    sample_slot TIMESTAMPTZ NOT NULL,
    collected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    collector_status TEXT NOT NULL,
    release_version TEXT,
    metrics JSONB NOT NULL,
    CONSTRAINT monitoring_samples_slot_unique UNIQUE (sample_slot),
    CONSTRAINT monitoring_samples_quarter_hour CHECK (
        EXTRACT(SECOND FROM sample_slot) = 0
        AND MOD(EXTRACT(MINUTE FROM sample_slot)::INTEGER, 15) = 0
    ),
    CONSTRAINT monitoring_samples_status_check CHECK (collector_status IN ('complete', 'partial', 'failed')),
    CONSTRAINT monitoring_samples_metrics_object CHECK (jsonb_typeof(metrics) = 'object')
);

CREATE INDEX IF NOT EXISTS monitoring_samples_collected_at_idx
    ON monitoring_samples (sample_slot DESC);

CREATE TABLE IF NOT EXISTS monitoring_incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_type TEXT NOT NULL,
    resource_key TEXT NOT NULL,
    state TEXT NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    activated_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    peak JSONB NOT NULL DEFAULT '{}'::jsonb,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT monitoring_incidents_type_format CHECK (incident_type ~ '^[a-z][a-z0-9_.]{2,127}$'),
    CONSTRAINT monitoring_incidents_resource_length CHECK (char_length(resource_key) BETWEEN 1 AND 128),
    CONSTRAINT monitoring_incidents_state_check CHECK (state IN ('pending', 'active', 'recovering', 'resolved')),
    CONSTRAINT monitoring_incidents_time_order CHECK (last_seen_at >= first_seen_at),
    CONSTRAINT monitoring_incidents_resolution_state CHECK (
        (state = 'resolved' AND resolved_at IS NOT NULL)
        OR (state <> 'resolved' AND resolved_at IS NULL)
    ),
    CONSTRAINT monitoring_incidents_peak_object CHECK (jsonb_typeof(peak) = 'object'),
    CONSTRAINT monitoring_incidents_version_positive CHECK (version > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS monitoring_incidents_one_open_uidx
    ON monitoring_incidents (incident_type, resource_key)
    WHERE resolved_at IS NULL;
CREATE INDEX IF NOT EXISTS monitoring_incidents_state_last_seen_idx
    ON monitoring_incidents (state, last_seen_at DESC);

CREATE TABLE IF NOT EXISTS report_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_type TEXT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    recipient_id UUID NOT NULL REFERENCES users(id),
    outbox_id UUID UNIQUE REFERENCES notification_outbox(id),
    aggregate_payload JSONB NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    telegram_message_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ,
    terminal_at TIMESTAMPTZ,
    CONSTRAINT report_deliveries_logical_unique UNIQUE (report_type, period_start, period_end, recipient_id),
    CONSTRAINT report_deliveries_type_format CHECK (report_type ~ '^[a-z][a-z0-9_.]{2,127}$'),
    CONSTRAINT report_deliveries_period_order CHECK (period_end > period_start),
    CONSTRAINT report_deliveries_payload_object CHECK (jsonb_typeof(aggregate_payload) = 'object'),
    CONSTRAINT report_deliveries_state_check CHECK (state IN ('pending', 'delivered', 'terminal', 'recipient_inactive')),
    CONSTRAINT report_deliveries_state_timestamps CHECK (
        (state = 'delivered' AND delivered_at IS NOT NULL AND terminal_at IS NULL)
        OR (state IN ('terminal', 'recipient_inactive') AND terminal_at IS NOT NULL AND delivered_at IS NULL)
        OR (state = 'pending' AND delivered_at IS NULL AND terminal_at IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS report_deliveries_period_idx
    ON report_deliveries (report_type, period_start DESC, period_end DESC);
CREATE INDEX IF NOT EXISTS report_deliveries_recipient_idx
    ON report_deliveries (recipient_id, period_start DESC);

COMMIT;
