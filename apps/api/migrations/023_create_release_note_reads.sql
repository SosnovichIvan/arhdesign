BEGIN;

CREATE TABLE IF NOT EXISTS release_note_reads (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version TEXT NOT NULL CHECK (version ~ '^[0-9]+\.[0-9]+\.[0-9]+$'),
    read_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, version)
);

COMMIT;
