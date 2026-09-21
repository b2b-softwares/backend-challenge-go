DROP INDEX IF EXISTS inbox_consumer_claim_idx;

DROP INDEX IF EXISTS inbox_consumer_status_idx;

DROP INDEX IF EXISTS inbox_message_id_idx;

ALTER TABLE inbox_messages
    DROP CONSTRAINT IF EXISTS inbox_message_hash_ck;

ALTER TABLE inbox_messages
    DROP CONSTRAINT IF EXISTS inbox_consumer_name_ck;

ALTER TABLE inbox_messages
    DROP CONSTRAINT IF EXISTS inbox_messages_consumer_message_pkey;

ALTER TABLE inbox_messages
    DROP COLUMN IF EXISTS message_hash;

ALTER TABLE inbox_messages
    DROP COLUMN IF EXISTS consumer_name;

ALTER TABLE inbox_messages
    ADD CONSTRAINT inbox_messages_pkey
        PRIMARY KEY (message_id);

ALTER TABLE inbox_messages
    ALTER COLUMN event_id SET NOT NULL;