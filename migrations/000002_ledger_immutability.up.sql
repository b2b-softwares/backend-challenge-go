CREATE OR REPLACE FUNCTION prevent_wallet_ledger_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION
        'wallet_ledger is immutable: % operation is not allowed',
        TG_OP
        USING ERRCODE = '55000';

    RETURN NULL;
END;
$$;

CREATE TRIGGER wallet_ledger_immutable_update
BEFORE UPDATE ON wallet_ledger
FOR EACH ROW
EXECUTE FUNCTION prevent_wallet_ledger_mutation();

CREATE TRIGGER wallet_ledger_immutable_delete
BEFORE DELETE ON wallet_ledger
FOR EACH ROW
EXECUTE FUNCTION prevent_wallet_ledger_mutation();