DROP INDEX IF EXISTS wager_transactions_pending_reference_idx;

ALTER TABLE wager_transactions
    DROP CONSTRAINT IF EXISTS wager_transactions_reference_attempts_non_negative_ck;

ALTER TABLE wager_transactions
    DROP COLUMN IF EXISTS reference_attempts,
    DROP COLUMN IF EXISTS reference_available_at,
    DROP COLUMN IF EXISTS reference_expires_at;

ALTER TABLE inbox_messages
    DROP CONSTRAINT IF EXISTS inbox_messages_consumer_message_pkey;

ALTER TABLE inbox_messages
    DROP COLUMN IF EXISTS consumer_name;

ALTER TABLE inbox_messages
    ADD CONSTRAINT inbox_messages_pkey
        PRIMARY KEY (message_id);