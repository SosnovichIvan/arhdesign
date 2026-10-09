CREATE TABLE IF NOT EXISTS project_expense_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    created_by_user_id UUID NOT NULL REFERENCES users(id),
    amount NUMERIC(14,2) NOT NULL,
    currency_code TEXT NOT NULL,
    category TEXT NOT NULL,
    description TEXT NOT NULL,
    vendor_name TEXT,
    planned_payment_on DATE,
    status TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_hash BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT project_expense_requests_amount_check CHECK (amount > 0 AND scale(amount) <= 2),
    CONSTRAINT project_expense_requests_currency_check CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT project_expense_requests_category_check CHECK (category IN ('materials','furniture','contractor','delivery','installation','design','other')),
    CONSTRAINT project_expense_requests_description_length CHECK (char_length(description) BETWEEN 2 AND 2000),
    CONSTRAINT project_expense_requests_vendor_length CHECK (vendor_name IS NULL OR char_length(vendor_name) <= 200),
    CONSTRAINT project_expense_requests_status_check CHECK (status IN ('draft','pending_approval','auto_approved','approved','rejected','awaiting_payment','paid','cancelled')),
    CONSTRAINT project_expense_requests_idempotency_length CHECK (char_length(idempotency_key) BETWEEN 8 AND 128),
    CONSTRAINT project_expense_requests_hash_length CHECK (octet_length(request_hash) = 32),
    CONSTRAINT project_expense_requests_version_positive CHECK (version > 0),
    UNIQUE (project_id, created_by_user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS project_expense_requests_project_created_idx
    ON project_expense_requests (project_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS project_expense_requests_project_open_idx
    ON project_expense_requests (project_id, status, created_at DESC)
    WHERE status IN ('pending_approval','auto_approved','approved','awaiting_payment');

CREATE TABLE IF NOT EXISTS project_ledger_operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    source_expense_request_id UUID REFERENCES project_expense_requests(id) ON DELETE RESTRICT,
    created_by_user_id UUID NOT NULL REFERENCES users(id),
    operation_type TEXT NOT NULL,
    direction TEXT NOT NULL,
    amount NUMERIC(14,2) NOT NULL,
    currency_code TEXT NOT NULL,
    description TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_ledger_operations_type_check CHECK (operation_type IN ('income','expense','adjustment','refund')),
    CONSTRAINT project_ledger_operations_direction_check CHECK (direction IN ('credit','debit')),
    CONSTRAINT project_ledger_operations_direction_semantics CHECK (
        (operation_type = 'income' AND direction = 'credit')
        OR (operation_type = 'expense' AND direction = 'debit')
        OR (operation_type = 'refund' AND direction = 'credit')
        OR operation_type = 'adjustment'
    ),
    CONSTRAINT project_ledger_operations_amount_check CHECK (amount > 0 AND scale(amount) <= 2),
    CONSTRAINT project_ledger_operations_currency_check CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT project_ledger_operations_description_length CHECK (char_length(description) BETWEEN 2 AND 2000),
    CONSTRAINT project_ledger_operations_expense_source_check CHECK ((operation_type = 'expense') = (source_expense_request_id IS NOT NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS project_ledger_operations_expense_source_uidx
    ON project_ledger_operations (source_expense_request_id)
    WHERE source_expense_request_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS project_ledger_operations_project_occurred_idx
    ON project_ledger_operations (project_id, occurred_at DESC, id DESC);

CREATE OR REPLACE FUNCTION reject_project_ledger_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'project ledger operations are immutable' USING ERRCODE = '23514';
END;
$$;

DROP TRIGGER IF EXISTS project_ledger_operations_immutable ON project_ledger_operations;
CREATE TRIGGER project_ledger_operations_immutable
BEFORE UPDATE OR DELETE ON project_ledger_operations
FOR EACH ROW EXECUTE FUNCTION reject_project_ledger_mutation();
