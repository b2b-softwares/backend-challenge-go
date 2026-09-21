DROP INDEX IF EXISTS inbox_event_id_idx;

DROP INDEX IF EXISTS inbox_claim_idx;

DROP INDEX IF EXISTS inbox_available_idx;

ALTER TABLE inbox_messages
    DROP CONSTRAINT IF EXISTS inbox_attempts_non_negative_ck;

ALTER TABLE inbox_messages
    DROP CONSTRAINT IF EXISTS inbox_status_ck;

ALTER TABLE inbox_messages
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS claimed_at,
    DROP COLUMN IF EXISTS claimed_by,
    DROP COLUMN IF EXISTS available_at,
    DROP COLUMN IF EXISTS attempts,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS payload,
    DROP COLUMN IF EXISTS event_id;