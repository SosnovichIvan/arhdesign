CREATE TABLE IF NOT EXISTS contact_consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    submission_id UUID NOT NULL REFERENCES contact_submissions(id) ON DELETE CASCADE,
    consent_version TEXT NOT NULL,
    document_path TEXT NOT NULL,
    consented_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT contact_consents_submission_id_unique UNIQUE (submission_id)
);

CREATE INDEX IF NOT EXISTS contact_consents_consented_at_idx ON contact_consents (consented_at);
