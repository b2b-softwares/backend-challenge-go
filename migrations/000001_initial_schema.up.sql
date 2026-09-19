CREATE TABLE wallets (
    id UUID PRIMARY KEY,
    player_id UUID NOT NULL,
    currency CHAR(3) NOT NULL,
    balance_minor BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT wallets_player_currency_uk
        UNIQUE (player_id, currency),

    CONSTRAINT wallets_balance_non_negative_ck
        CHECK (balance_minor >= 0),

    CONSTRAINT wallets_version_positive_ck
        CHECK (version >= 1)
);

CREATE TABLE wager_transactions (
    id UUID PRIMARY KEY,
    external_transaction_id TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    payload_hash TEXT NOT NULL,

    wallet_id UUID NOT NULL,
    player_id UUID NOT NULL,

    round_id TEXT NOT NULL DEFAULT '',
    game_id TEXT NOT NULL DEFAULT '',

    kind TEXT NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,

    reference_external_id TEXT NOT NULL DEFAULT '',
    reference_transaction_id UUID NULL,

    status TEXT NOT NULL,
    failure_code TEXT NOT NULL DEFAULT '',

    result_balance_minor BIGINT NULL,
    result_balance_currency CHAR(3) NULL,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT wager_transactions_provider_external_uk
        UNIQUE (provider_id, external_transaction_id),

    CONSTRAINT wager_transactions_provider_idempotency_uk
        UNIQUE (provider_id, idempotency_key),

    CONSTRAINT wager_transactions_wallet_fk
        FOREIGN KEY (wallet_id)
        REFERENCES wallets(id),

    CONSTRAINT wager_transactions_kind_ck
        CHECK (
            kind IN (
                'BET',
                'WIN',
                'LOSS',
                'REFUND',
                'ROLLBACK'
            )
        ),

    CONSTRAINT wager_transactions_status_ck
        CHECK (
            status IN (
                'PENDING',
                'PENDING_REFERENCE',
                'PROCESSED',
                'REJECTED',
                'FAILED'
            )
        ),

    CONSTRAINT wager_transactions_loss_zero_ck
        CHECK (
            kind <> 'LOSS'
            OR amount_minor = 0
        ),

    CONSTRAINT wager_transactions_non_loss_positive_ck
        CHECK (
            kind = 'LOSS'
            OR amount_minor > 0
        )
);

CREATE TABLE wallet_ledger (
    id UUID PRIMARY KEY,
    wallet_id UUID NOT NULL,
    transaction_id UUID NOT NULL,

    direction TEXT NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,

    balance_before_minor BIGINT NOT NULL,
    balance_after_minor BIGINT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT wallet_ledger_wallet_fk
        FOREIGN KEY (wallet_id)
        REFERENCES wallets(id),

    CONSTRAINT wallet_ledger_transaction_fk
        FOREIGN KEY (transaction_id)
        REFERENCES wager_transactions(id),

    CONSTRAINT wallet_ledger_wallet_transaction_uk
        UNIQUE (wallet_id, transaction_id),

    CONSTRAINT wallet_ledger_direction_ck
        CHECK (
            direction IN ('DEBIT', 'CREDIT')
        ),

    CONSTRAINT wallet_ledger_amount_positive_ck
        CHECK (amount_minor > 0),

    CONSTRAINT wallet_ledger_balance_before_non_negative_ck
        CHECK (balance_before_minor >= 0),

    CONSTRAINT wallet_ledger_balance_after_non_negative_ck
        CHECK (balance_after_minor >= 0),

    CONSTRAINT wallet_ledger_balance_relation_ck
        CHECK (
            (
                direction = 'DEBIT'
                AND balance_after_minor = balance_before_minor - amount_minor
            )
            OR
            (
                direction = 'CREDIT'
                AND balance_after_minor = balance_before_minor + amount_minor
            )
        )
);

CREATE TABLE idempotency_records (
    provider_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,

    payload_hash TEXT NOT NULL,
    transaction_id UUID NOT NULL,

    status TEXT NOT NULL,
    response_body JSONB NOT NULL,

    observed_balance_amount BIGINT NOT NULL,
    observed_balance_currency CHAR(3) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (provider_id, idempotency_key),

    CONSTRAINT idempotency_transaction_fk
        FOREIGN KEY (transaction_id)
        REFERENCES wager_transactions(id)
);

CREATE TABLE inbox_messages (
    message_id TEXT PRIMARY KEY,
    message_type TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ NULL
);

CREATE TABLE outbox_events (
    event_id UUID PRIMARY KEY,

    event_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,

    correlation_id TEXT NOT NULL,
    causation_id TEXT NOT NULL,

    occurred_at TIMESTAMPTZ NOT NULL,
    version INTEGER NOT NULL,

    payload JSONB NOT NULL,

    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL,

    claimed_by TEXT NULL,
    claimed_at TIMESTAMPTZ NULL,

    published_at TIMESTAMPTZ NULL,
    last_error TEXT NULL,

    CONSTRAINT outbox_attempts_non_negative_ck
        CHECK (attempts >= 0),

    CONSTRAINT outbox_version_positive_ck
        CHECK (version >= 1)
);

CREATE INDEX wager_transactions_wallet_idx
    ON wager_transactions(wallet_id);

CREATE INDEX wager_transactions_reference_idx
    ON wager_transactions(
        provider_id,
        reference_external_id
    );

CREATE INDEX wallet_ledger_wallet_created_idx
    ON wallet_ledger(wallet_id, created_at, id);

CREATE INDEX outbox_available_idx
    ON outbox_events(available_at, occurred_at, event_id)
    WHERE published_at IS NULL;

CREATE INDEX outbox_claim_idx
    ON outbox_events(claimed_by, claimed_at)
    WHERE published_at IS NULL;

CREATE INDEX inbox_processed_idx
    ON inbox_messages(processed_at);