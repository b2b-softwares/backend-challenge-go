ALTER TABLE wager_transactions
    ADD COLUMN reference_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN reference_available_at TIMESTAMPTZ NULL,
    ADD COLUMN reference_expires_at TIMESTAMPTZ NULL;

ALTER TABLE wager_transactions
    ADD CONSTRAINT wager_transactions_reference_attempts_non_negative_ck
        CHECK (reference_attempts >= 0);

CREATE INDEX wager_transactions_pending_reference_idx
    ON wager_transactions(
        reference_available_at,
        updated_at,
        id
    )
    WHERE status = 'PENDING_REFERENCE';


ALTER TABLE inbox_messages
    DROP CONSTRAINT inbox_messages_pkey;

ALTER TABLE inbox_messages
    ADD COLUMN consumer_name TEXT;

UPDATE inbox_messages
SET consumer_name = 'legacy'
WHERE consumer_name IS NULL;

ALTER TABLE inbox_messages
    ALTER COLUMN consumer_name SET NOT NULL;

ALTER TABLE inbox_messages
    ADD CONSTRAINT inbox_messages_consumer_message_pkey
        PRIMARY KEY (consumer_name, message_id);