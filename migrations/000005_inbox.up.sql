ALTER TABLE inbox_messages
    DROP CONSTRAINT IF EXISTS inbox_messages_pkey;

ALTER TABLE inbox_messages
    DROP CONSTRAINT IF EXISTS inbox_messages_consumer_message_pkey;

ALTER TABLE inbox_messages
    ADD COLUMN IF NOT EXISTS consumer_name TEXT NOT NULL DEFAULT 'wagering-consumer',
    ADD COLUMN IF NOT EXISTS message_hash TEXT NOT NULL DEFAULT 'legacy';

ALTER TABLE inbox_messages
    ALTER COLUMN event_id DROP NOT NULL;

ALTER TABLE inbox_messages
    ADD CONSTRAINT inbox_consumer_name_ck
        CHECK (length(btrim(consumer_name)) > 0);

ALTER TABLE inbox_messages
    ADD CONSTRAINT inbox_message_hash_ck
        CHECK (length(btrim(message_hash)) > 0);

ALTER TABLE inbox_messages
    ADD CONSTRAINT inbox_messages_consumer_message_pkey
        PRIMARY KEY (consumer_name, message_id);

ALTER TABLE inbox_messages
    ALTER COLUMN consumer_name DROP DEFAULT,
    ALTER COLUMN message_hash DROP DEFAULT;

CREATE INDEX IF NOT EXISTS inbox_message_id_idx
    ON inbox_messages (message_id);

CREATE INDEX IF NOT EXISTS inbox_consumer_status_idx
    ON inbox_messages (
        consumer_name,
        status,
        available_at,
        received_at,
        message_id
    );

CREATE INDEX IF NOT EXISTS inbox_consumer_claim_idx
    ON inbox_messages (
        consumer_name,
        claimed_by,
        claimed_at
    )
    WHERE status = 'PROCESSING';