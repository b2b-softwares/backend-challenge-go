ALTER TABLE inbox_messages
    ADD COLUMN event_id UUID,
    ADD COLUMN payload JSONB,
    ADD COLUMN status TEXT NOT NULL DEFAULT 'RECEIVED',
    ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN claimed_by TEXT,
    ADD COLUMN claimed_at TIMESTAMPTZ,
    ADD COLUMN last_error TEXT;

ALTER TABLE inbox_messages
    ADD CONSTRAINT inbox_status_ck
        CHECK (
            status IN (
                'RECEIVED',
                'PROCESSING',
                'PROCESSED',
                'FAILED'
            )
        );

ALTER TABLE inbox_messages
    ADD CONSTRAINT inbox_attempts_non_negative_ck
        CHECK (attempts >= 0);

ALTER TABLE inbox_messages
    ALTER COLUMN event_id SET NOT NULL,
    ALTER COLUMN payload SET NOT NULL;

CREATE INDEX inbox_available_idx
    ON inbox_messages (available_at, received_at, message_id)
    WHERE status IN ('RECEIVED', 'FAILED');

CREATE INDEX inbox_claim_idx
    ON inbox_messages (claimed_by, claimed_at)
    WHERE status = 'PROCESSING';

CREATE INDEX inbox_event_id_idx
    ON inbox_messages (event_id);