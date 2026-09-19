package domain

import (
	"time"

	"github.com/google/uuid"
)

type LedgerDirection string

const (
	LedgerDirectionDebit  LedgerDirection = "DEBIT"
	LedgerDirectionCredit LedgerDirection = "CREDIT"
)

type WalletLedgerEntry struct {
	id            uuid.UUID
	walletID      uuid.UUID
	transactionID uuid.UUID
	direction     LedgerDirection
	money         Money
	balanceBefore Money
	balanceAfter  Money
	createdAt     time.Time
}

// NewLedgerEntry creates an immutable ledger entry and validates the
// resulting balance mathematically.
//
// The caller is responsible for ensuring the corresponding wallet
// transition occurs in the same database transaction.
func NewLedgerEntry(
	id uuid.UUID,
	walletID uuid.UUID,
	transactionID uuid.UUID,
	direction LedgerDirection,
	money Money,
	balanceBefore Money,
	balanceAfter Money,
	createdAt time.Time,
) (WalletLedgerEntry, error) {
	if id == uuid.Nil ||
		walletID == uuid.Nil ||
		transactionID == uuid.Nil ||
		createdAt.IsZero() {
		return WalletLedgerEntry{}, ErrInvalidLedgerEntry
	}

	if !money.IsPositive() {
		return WalletLedgerEntry{}, ErrInvalidLedgerEntry
	}

	if balanceBefore.Currency() != money.Currency() ||
		balanceAfter.Currency() != money.Currency() {
		return WalletLedgerEntry{}, ErrCurrencyMismatch
	}

	if balanceBefore.IsNegative() || balanceAfter.IsNegative() {
		return WalletLedgerEntry{}, ErrInvalidLedgerEntry
	}

	switch direction {
	case LedgerDirectionDebit:
		expected, err := balanceBefore.Sub(money)
		if err != nil {
			return WalletLedgerEntry{}, err
		}

		if !expected.Equal(balanceAfter) {
			return WalletLedgerEntry{}, ErrInvalidLedgerEntry
		}

	case LedgerDirectionCredit:
		expected, err := balanceBefore.Add(money)
		if err != nil {
			return WalletLedgerEntry{}, err
		}

		if !expected.Equal(balanceAfter) {
			return WalletLedgerEntry{}, ErrInvalidLedgerEntry
		}

	default:
		return WalletLedgerEntry{}, ErrInvalidLedgerEntry
	}

	return WalletLedgerEntry{
		id:            id,
		walletID:      walletID,
		transactionID: transactionID,
		direction:     direction,
		money:         money,
		balanceBefore: balanceBefore,
		balanceAfter:  balanceAfter,
		createdAt:     createdAt.UTC(),
	}, nil
}

func (e WalletLedgerEntry) ID() uuid.UUID {
	return e.id
}

func (e WalletLedgerEntry) WalletID() uuid.UUID {
	return e.walletID
}

func (e WalletLedgerEntry) TransactionID() uuid.UUID {
	return e.transactionID
}

func (e WalletLedgerEntry) Direction() LedgerDirection {
	return e.direction
}

func (e WalletLedgerEntry) Money() Money {
	return e.money
}

func (e WalletLedgerEntry) BalanceBefore() Money {
	return e.balanceBefore
}

func (e WalletLedgerEntry) BalanceAfter() Money {
	return e.balanceAfter
}

func (e WalletLedgerEntry) CreatedAt() time.Time {
	return e.createdAt
}
