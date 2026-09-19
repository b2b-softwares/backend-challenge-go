package domain

import (
	"time"

	"github.com/google/uuid"
)

type Wallet struct {
	id        uuid.UUID
	playerID  uuid.UUID
	currency  Currency
	balance   Money
	version   int64
	createdAt time.Time
	updatedAt time.Time
}

// NewWallet creates a new wallet.
//
// The initial version is always 1.
//
// A zero initial balance is valid and does not itself imply an OPENING
// transaction. Creation of the corresponding financial transaction
// is handled by the application layer.
func NewWallet(
	id uuid.UUID,
	playerID uuid.UUID,
	initialBalance Money,
	now time.Time,
) (Wallet, error) {
	if id == uuid.Nil {
		return Wallet{}, ErrInvalidWallet
	}

	if playerID == uuid.Nil {
		return Wallet{}, ErrInvalidWallet
	}

	if initialBalance.IsNegative() {
		return Wallet{}, ErrInvalidWallet
	}

	if now.IsZero() {
		return Wallet{}, ErrInvalidWallet
	}

	return Wallet{
		id:        id,
		playerID:  playerID,
		currency:  initialBalance.Currency(),
		balance:   initialBalance,
		version:   1,
		createdAt: now.UTC(),
		updatedAt: now.UTC(),
	}, nil
}

// RehydrateWallet recreates a Wallet from persistent state.
//
// Rehydration does not execute financial transitions and does not
// generate events.
func RehydrateWallet(
	id uuid.UUID,
	playerID uuid.UUID,
	currency Currency,
	balance Money,
	version int64,
	createdAt time.Time,
	updatedAt time.Time,
) (Wallet, error) {
	if id == uuid.Nil || playerID == uuid.Nil {
		return Wallet{}, ErrInvalidWallet
	}

	if version < 1 {
		return Wallet{}, ErrInvalidWallet
	}

	if createdAt.IsZero() || updatedAt.IsZero() {
		return Wallet{}, ErrInvalidWallet
	}

	if balance.Currency() != currency {
		return Wallet{}, ErrCurrencyMismatch
	}

	if balance.IsNegative() {
		return Wallet{}, ErrInvalidWallet
	}

	return Wallet{
		id:        id,
		playerID:  playerID,
		currency:  currency,
		balance:   balance,
		version:   version,
		createdAt: createdAt.UTC(),
		updatedAt: updatedAt.UTC(),
	}, nil
}

func (w Wallet) ID() uuid.UUID {
	return w.id
}

func (w Wallet) PlayerID() uuid.UUID {
	return w.playerID
}

func (w Wallet) Currency() Currency {
	return w.currency
}

func (w Wallet) Balance() Money {
	return w.balance
}

func (w Wallet) Version() int64 {
	return w.version
}

func (w Wallet) CreatedAt() time.Time {
	return w.createdAt
}

func (w Wallet) UpdatedAt() time.Time {
	return w.updatedAt
}

// Debit removes money from the wallet.
//
// The aggregate enforces currency compatibility and the
// non-negative-balance invariant.
func (w *Wallet) Debit(money Money, now time.Time) error {
	if w == nil {
		return ErrInvalidWallet
	}

	if money.Currency() != w.currency {
		return ErrCurrencyMismatch
	}

	if !money.IsPositive() {
		return ErrInvalidMoney
	}

	comparison, err := w.balance.Compare(money)
	if err != nil {
		return err
	}

	if comparison < 0 {
		return ErrInsufficientBalance
	}

	newBalance, err := w.balance.Sub(money)
	if err != nil {
		return err
	}

	if newBalance.IsNegative() {
		return ErrInvalidWallet
	}

	return w.applyBalance(newBalance, now)
}

// Credit adds money to the wallet.
func (w *Wallet) Credit(money Money, now time.Time) error {
	if w == nil {
		return ErrInvalidWallet
	}

	if money.Currency() != w.currency {
		return ErrCurrencyMismatch
	}

	if !money.IsPositive() {
		return ErrInvalidMoney
	}

	newBalance, err := w.balance.Add(money)
	if err != nil {
		return err
	}

	if newBalance.IsNegative() {
		return ErrInvalidWallet
	}

	return w.applyBalance(newBalance, now)
}

func (w *Wallet) applyBalance(balance Money, now time.Time) error {
	if now.IsZero() {
		return ErrInvalidWallet
	}

	if balance.Currency() != w.currency {
		return ErrCurrencyMismatch
	}

	if balance.IsNegative() {
		return ErrInvalidWallet
	}

	if w.version == int64(^uint64(0)>>1) {
		return ErrInvalidWallet
	}

	w.balance = balance
	w.version++
	w.updatedAt = now.UTC()

	return nil
}
