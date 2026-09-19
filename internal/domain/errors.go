package domain

import "errors"

var (
	ErrInvalidMoney               = errors.New("invalid money")
	ErrInvalidCurrency            = errors.New("invalid currency")
	ErrCurrencyMismatch           = errors.New("currency mismatch")
	ErrInsufficientBalance        = errors.New("insufficient balance")
	ErrInvalidWallet              = errors.New("invalid wallet")
	ErrInvalidTransaction         = errors.New("invalid transaction")
	ErrInvalidTransactionState    = errors.New("invalid transaction state")
	ErrTerminalTransaction        = errors.New("terminal transaction")
	ErrInvalidTransactionKind     = errors.New("invalid transaction kind")
	ErrInvalidLedgerEntry         = errors.New("invalid ledger entry")
	ErrDuplicateTransaction       = errors.New("duplicate transaction")
	ErrDuplicateIdempotency       = errors.New("duplicate idempotency")
	ErrDuplicateWagerIdempotency  = errors.New("duplicate wager idempotency")
	ErrDuplicateIdempotencyRecord = errors.New("duplicate idempotency record")
	ErrIdempotencyConflict        = errors.New("idempotency conflict")
	ErrReferenceNotFound          = errors.New("reference not found")
	ErrReferenceNotProcessed      = errors.New("reference not processed")
	ErrDuplicateReversal          = errors.New("duplicate reversal")
	ErrUnauthorizedProvider       = errors.New("unauthorized provider")
	ErrInvalidOpening             = errors.New("invalid opening")
	ErrInvalidExternalOperation   = errors.New("invalid external operation")
	ErrConcurrentModification     = errors.New("concurrent modification")
)
