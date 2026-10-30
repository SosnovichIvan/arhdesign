CREATE UNIQUE INDEX IF NOT EXISTS project_expense_requests_id_project_uidx
    ON project_expense_requests (id, project_id);

CREATE TABLE IF NOT EXISTS project_materials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    created_by_user_id UUID NOT NULL REFERENCES users(id),
    category TEXT NOT NULL,
    name TEXT NOT NULL,
    supplier_name TEXT NOT NULL,
    contact_info TEXT,
    contract_reference TEXT,
    contract_amount NUMERIC(14,2) NOT NULL,
    currency_code TEXT NOT NULL,
    linked_expense_request_id UUID,
    payment_status TEXT NOT NULL DEFAULT 'unpaid',
    delivery_status TEXT NOT NULL DEFAULT 'expected',
    planned_delivery_on DATE,
    actual_delivery_on DATE,
    installation_on DATE,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    deleted_by_user_id UUID REFERENCES users(id),
    delete_reason TEXT,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT project_materials_category_check CHECK (category IN ('materials','furniture','delivery','installation','other')),
    CONSTRAINT project_materials_name_length CHECK (char_length(name) BETWEEN 2 AND 300),
    CONSTRAINT project_materials_supplier_length CHECK (char_length(supplier_name) BETWEEN 2 AND 200),
    CONSTRAINT project_materials_contact_length CHECK (contact_info IS NULL OR char_length(contact_info) <= 500),
    CONSTRAINT project_materials_contract_reference_length CHECK (contract_reference IS NULL OR char_length(contract_reference) <= 200),
    CONSTRAINT project_materials_amount_check CHECK (contract_amount > 0 AND scale(contract_amount) <= 2),
    CONSTRAINT project_materials_currency_check CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT project_materials_payment_status_check CHECK (payment_status IN ('unpaid','partial','paid')),
    CONSTRAINT project_materials_delivery_status_check CHECK (delivery_status IN ('expected','partially_delivered','delivered','installed','cancelled')),
    CONSTRAINT project_materials_actual_delivery_semantics CHECK (actual_delivery_on IS NULL OR delivery_status IN ('delivered','installed')),
    CONSTRAINT project_materials_notes_length CHECK (notes IS NULL OR char_length(notes) <= 5000),
    CONSTRAINT project_materials_delete_audit_check CHECK ((deleted_at IS NULL) = (deleted_by_user_id IS NULL) AND (deleted_at IS NULL) = (delete_reason IS NULL)),
    CONSTRAINT project_materials_delete_reason_length CHECK (delete_reason IS NULL OR char_length(delete_reason) BETWEEN 2 AND 500),
    CONSTRAINT project_materials_version_positive CHECK (version > 0),
    CONSTRAINT project_materials_expense_project_fk FOREIGN KEY (linked_expense_request_id, project_id)
        REFERENCES project_expense_requests (id, project_id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS project_materials_project_active_idx
    ON project_materials (project_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS project_materials_project_delivery_idx
    ON project_materials (project_id, planned_delivery_on, id)
    WHERE deleted_at IS NULL AND delivery_status NOT IN ('installed','cancelled');
CREATE INDEX IF NOT EXISTS project_materials_expense_idx
    ON project_materials (linked_expense_request_id)
    WHERE linked_expense_request_id IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS project_material_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    material_id UUID NOT NULL REFERENCES project_materials(id) ON DELETE RESTRICT,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version BIGINT NOT NULL,
    changed_by_user_id UUID NOT NULL REFERENCES users(id),
    change_type TEXT NOT NULL,
    snapshot JSONB NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_material_versions_version_positive CHECK (version > 0),
    CONSTRAINT project_material_versions_change_type_check CHECK (change_type IN ('created','updated','deleted')),
    CONSTRAINT project_material_versions_snapshot_object CHECK (jsonb_typeof(snapshot) = 'object'),
    UNIQUE (material_id, version)
);

CREATE INDEX IF NOT EXISTS project_material_versions_project_changed_idx
    ON project_material_versions (project_id, changed_at DESC, id DESC);
