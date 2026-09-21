ALTER TABLE wager_transactions
    DROP CONSTRAINT wager_transactions_non_loss_positive_ck;

ALTER TABLE wager_transactions
    ADD CONSTRAINT wager_transactions_non_loss_positive_ck
    CHECK (
        kind = 'LOSS'
        OR amount_minor > 0
        OR status = 'PENDING_REFERENCE'
    );