package domain

import (
	"time"

	"github.com/google/uuid"
)

type TransactionKind string

const (
	TransactionKindOpening  TransactionKind = "OPENING"
	TransactionKindBet      TransactionKind = "BET"
	TransactionKindWin      TransactionKind = "WIN"
	TransactionKindLoss     TransactionKind = "LOSS"
	TransactionKindRefund   TransactionKind = "REFUND"
	TransactionKindRollback TransactionKind = "ROLLBACK"
)

type TransactionStatus string

const (
	TransactionStatusPending          TransactionStatus = "PENDING"
	TransactionStatusPendingReference TransactionStatus = "PENDING_REFERENCE"
	TransactionStatusProcessed        TransactionStatus = "PROCESSED"
	TransactionStatusRejected         TransactionStatus = "REJECTED"
	TransactionStatusFailed           TransactionStatus = "FAILED"
)

type WagerTransaction struct {
	id                     uuid.UUID
	externalTransactionID  string
	providerID             string
	idempotencyKey         string
	payloadHash            string
	walletID               uuid.UUID
	playerID               uuid.UUID
	roundID                string
	gameID                 string
	kind                   TransactionKind
	money                  Money
	referenceExternalID    string
	referenceTransactionID *uuid.UUID
	status                 TransactionStatus
	failureCode            string
	resultBalance          *Money
	createdAt              time.Time
	updatedAt              time.Time
}

// NewExternalTransaction creates a transaction originating from a provider.
// OPENING is deliberately rejected here because it is an internal wallet-opening operation.
func NewExternalTransaction(
	id uuid.UUID,
	externalTransactionID string,
	providerID string,
	idempotencyKey string,
	payloadHash string,
	walletID uuid.UUID,
	playerID uuid.UUID,
	roundID string,
	gameID string,
	kind TransactionKind,
	money Money,
	referenceExternalID string,
	now time.Time,
) (WagerTransaction, error) {
	if id == uuid.Nil ||
		externalTransactionID == "" ||
		providerID == "" ||
		idempotencyKey == "" ||
		payloadHash == "" ||
		walletID == uuid.Nil ||
		playerID == uuid.Nil ||
		now.IsZero() {
		return WagerTransaction{}, ErrInvalidTransaction
	}

	if kind == TransactionKindOpening {
		return WagerTransaction{}, ErrInvalidOpening
	}

	if !isExternalTransactionKind(kind) {
		return WagerTransaction{}, ErrInvalidTransactionKind
	}

	if err := validateExternalMoney(kind, money); err != nil {
		return WagerTransaction{}, err
	}

	if (kind == TransactionKindRefund || kind == TransactionKindRollback) &&
		referenceExternalID == "" {
		return WagerTransaction{}, ErrInvalidTransaction
	}

	return WagerTransaction{
		id:                    id,
		externalTransactionID: externalTransactionID,
		providerID:            providerID,
		idempotencyKey:        idempotencyKey,
		payloadHash:           payloadHash,
		walletID:              walletID,
		playerID:              playerID,
		roundID:               roundID,
		gameID:                gameID,
		kind:                  kind,
		money:                 money,
		referenceExternalID:   referenceExternalID,
		status:                TransactionStatusPending,
		createdAt:             now.UTC(),
		updatedAt:             now.UTC(),
	}, nil
}

// NewPendingReferenceTransaction creates a reversal transaction whose referenced
// transaction has not been processed/resolved yet.
func NewPendingReferenceTransaction(
	id uuid.UUID,
	externalTransactionID string,
	providerID string,
	idempotencyKey string,
	payloadHash string,
	walletID uuid.UUID,
	playerID uuid.UUID,
	roundID string,
	gameID string,
	kind TransactionKind,
	currency Currency,
	referenceExternalID string,
	now time.Time,
) (WagerTransaction, error) {
	if id == uuid.Nil ||
		externalTransactionID == "" ||
		providerID == "" ||
		idempotencyKey == "" ||
		payloadHash == "" ||
		walletID == uuid.Nil ||
		playerID == uuid.Nil ||
		referenceExternalID == "" ||
		now.IsZero() {
		return WagerTransaction{}, ErrInvalidTransaction
	}

	if !isReversalKind(kind) {
		return WagerTransaction{}, ErrInvalidTransactionKind
	}

	money, err := NewMoney("0.00", currency)
	if err != nil {
		return WagerTransaction{}, err
	}

	return WagerTransaction{
		id:                    id,
		externalTransactionID: externalTransactionID,
		providerID:            providerID,
		idempotencyKey:        idempotencyKey,
		payloadHash:           payloadHash,
		walletID:              walletID,
		playerID:              playerID,
		roundID:               roundID,
		gameID:                gameID,
		kind:                  kind,
		money:                 money,
		referenceExternalID:   referenceExternalID,
		status:                TransactionStatusPendingReference,
		createdAt:             now.UTC(),
		updatedAt:             now.UTC(),
	}, nil
}

func RehydrateTransaction(
	id uuid.UUID,
	externalTransactionID string,
	providerID string,
	idempotencyKey string,
	payloadHash string,
	walletID uuid.UUID,
	playerID uuid.UUID,
	roundID string,
	gameID string,
	kind TransactionKind,
	money Money,
	referenceExternalID string,
	referenceTransactionID *uuid.UUID,
	status TransactionStatus,
	failureCode string,
	resultBalance *Money,
	createdAt time.Time,
	updatedAt time.Time,
) (WagerTransaction, error) {
	if id == uuid.Nil ||
		externalTransactionID == "" ||
		providerID == "" ||
		idempotencyKey == "" ||
		payloadHash == "" ||
		walletID == uuid.Nil ||
		playerID == uuid.Nil ||
		createdAt.IsZero() ||
		updatedAt.IsZero() {
		return WagerTransaction{}, ErrInvalidTransaction
	}

	if !isValidTransactionStatus(status) {
		return WagerTransaction{}, ErrInvalidTransactionState
	}

	if kind == TransactionKindOpening {
		return WagerTransaction{}, ErrInvalidOpening
	}

	if !isExternalTransactionKind(kind) {
		return WagerTransaction{}, ErrInvalidTransactionKind
	}

	if status == TransactionStatusPendingReference && !isReversalKind(kind) {
		return WagerTransaction{}, ErrInvalidTransactionState
	}

	if resultBalance != nil && resultBalance.Currency() != money.Currency() {
		return WagerTransaction{}, ErrCurrencyMismatch
	}

	return WagerTransaction{
		id:                     id,
		externalTransactionID:  externalTransactionID,
		providerID:             providerID,
		idempotencyKey:         idempotencyKey,
		payloadHash:            payloadHash,
		walletID:               walletID,
		playerID:               playerID,
		roundID:                roundID,
		gameID:                 gameID,
		kind:                   kind,
		money:                  money,
		referenceExternalID:    referenceExternalID,
		referenceTransactionID: referenceTransactionID,
		status:                 status,
		failureCode:            failureCode,
		resultBalance:          resultBalance,
		createdAt:              createdAt.UTC(),
		updatedAt:              updatedAt.UTC(),
	}, nil
}

func (t WagerTransaction) ID() uuid.UUID {
	return t.id
}

func (t WagerTransaction) ExternalTransactionID() string {
	return t.externalTransactionID
}

func (t WagerTransaction) ProviderID() string {
	return t.providerID
}

func (t WagerTransaction) IdempotencyKey() string {
	return t.idempotencyKey
}

func (t WagerTransaction) PayloadHash() string {
	return t.payloadHash
}

func (t WagerTransaction) WalletID() uuid.UUID {
	return t.walletID
}

func (t WagerTransaction) PlayerID() uuid.UUID {
	return t.playerID
}

func (t WagerTransaction) RoundID() string {
	return t.roundID
}

func (t WagerTransaction) GameID() string {
	return t.gameID
}

func (t WagerTransaction) Kind() TransactionKind {
	return t.kind
}

func (t WagerTransaction) Money() Money {
	return t.money
}

func (t WagerTransaction) ReferenceExternalID() string {
	return t.referenceExternalID
}

func (t WagerTransaction) ReferenceTransactionID() *uuid.UUID {
	return t.referenceTransactionID
}

func (t WagerTransaction) Status() TransactionStatus {
	return t.status
}

func (t WagerTransaction) FailureCode() string {
	return t.failureCode
}

func (t WagerTransaction) ResultBalance() *Money {
	return t.resultBalance
}

func (t WagerTransaction) CreatedAt() time.Time {
	return t.createdAt
}

func (t WagerTransaction) UpdatedAt() time.Time {
	return t.updatedAt
}

func (t *WagerTransaction) MarkPendingReference(now time.Time) error {
	if t == nil {
		return ErrInvalidTransaction
	}

	if isTerminalStatus(t.status) {
		return ErrTerminalTransaction
	}

	if t.status != TransactionStatusPending {
		return ErrInvalidTransactionState
	}

	if !isReversalKind(t.kind) {
		return ErrInvalidTransactionKind
	}

	if now.IsZero() {
		return ErrInvalidTransaction
	}

	t.status = TransactionStatusPendingReference
	t.updatedAt = now.UTC()

	return nil
}

func (t *WagerTransaction) MarkProcessed(balance Money, now time.Time) error {
	if t == nil {
		return ErrInvalidTransaction
	}

	if isTerminalStatus(t.status) {
		return ErrTerminalTransaction
	}

	if t.status != TransactionStatusPending {
		return ErrInvalidTransactionState
	}

	if now.IsZero() {
		return ErrInvalidTransaction
	}

	if !t.money.IsPositive() {
		return ErrInvalidMoney
	}

	if balance.Currency() != t.money.Currency() {
		return ErrCurrencyMismatch
	}

	t.status = TransactionStatusProcessed
	t.resultBalance = &balance
	t.failureCode = ""
	t.updatedAt = now.UTC()

	return nil
}

func (t *WagerTransaction) MarkRejected(
	failureCode string,
	now time.Time,
) error {
	if t == nil {
		return ErrInvalidTransaction
	}

	if isTerminalStatus(t.status) {
		return ErrTerminalTransaction
	}

	if failureCode == "" || now.IsZero() {
		return ErrInvalidTransaction
	}

	t.status = TransactionStatusRejected
	t.failureCode = failureCode
	t.updatedAt = now.UTC()

	return nil
}

func (t *WagerTransaction) MarkFailed(
	failureCode string,
	now time.Time,
) error {
	if t == nil {
		return ErrInvalidTransaction
	}

	if isTerminalStatus(t.status) {
		return ErrTerminalTransaction
	}

	if failureCode == "" || now.IsZero() {
		return ErrInvalidTransaction
	}

	t.status = TransactionStatusFailed
	t.failureCode = failureCode
	t.updatedAt = now.UTC()

	return nil
}

// ResolveReference preserves the original domain API used by existing callers
// and tests. It resolves the reference and moves the transaction back to PENDING.
// The referenced amount is supplied separately through ResolveReferenceWithMoney
// when the reversal must inherit the reference amount.
func (t *WagerTransaction) ResolveReference(
	referenceID uuid.UUID,
	now time.Time,
) error {
	if t == nil {
		return ErrInvalidTransaction
	}

	if isTerminalStatus(t.status) {
		return ErrTerminalTransaction
	}

	if t.status != TransactionStatusPendingReference {
		return ErrInvalidTransactionState
	}

	if !isReversalKind(t.kind) {
		return ErrInvalidTransactionKind
	}

	if referenceID == uuid.Nil || now.IsZero() {
		return ErrInvalidTransaction
	}

	t.referenceTransactionID = &referenceID
	t.status = TransactionStatusPending
	t.updatedAt = now.UTC()

	return nil
}

// ResolveReferenceWithMoney resolves a pending reversal and assigns the amount
// inherited from the referenced transaction.
func (t *WagerTransaction) ResolveReferenceWithMoney(
	referenceID uuid.UUID,
	referenceMoney Money,
	now time.Time,
) error {
	if t == nil {
		return ErrInvalidTransaction
	}

	if isTerminalStatus(t.status) {
		return ErrTerminalTransaction
	}

	if t.status != TransactionStatusPendingReference {
		return ErrInvalidTransactionState
	}

	if !isReversalKind(t.kind) {
		return ErrInvalidTransactionKind
	}

	if referenceID == uuid.Nil ||
		now.IsZero() ||
		!referenceMoney.IsPositive() {
		return ErrInvalidTransaction
	}

	if t.money.Currency() != referenceMoney.Currency() {
		return ErrCurrencyMismatch
	}

	t.referenceTransactionID = &referenceID
	t.money = referenceMoney
	t.status = TransactionStatusPending
	t.updatedAt = now.UTC()

	return nil
}

func validateExternalMoney(kind TransactionKind, money Money) error {
	switch kind {
	case TransactionKindLoss:
		if !money.IsZero() {
			return ErrInvalidMoney
		}

		return nil

	case TransactionKindBet,
		TransactionKindWin,
		TransactionKindRefund,
		TransactionKindRollback:
		if !money.IsPositive() {
			return ErrInvalidMoney
		}

		return nil

	default:
		return ErrInvalidTransactionKind
	}
}

func isExternalTransactionKind(kind TransactionKind) bool {
	switch kind {
	case TransactionKindBet,
		TransactionKindWin,
		TransactionKindLoss,
		TransactionKindRefund,
		TransactionKindRollback:
		return true

	default:
		return false
	}
}

func isReversalKind(kind TransactionKind) bool {
	return kind == TransactionKindRefund || kind == TransactionKindRollback
}

func isValidTransactionStatus(status TransactionStatus) bool {
	switch status {
	case TransactionStatusPending,
		TransactionStatusPendingReference,
		TransactionStatusProcessed,
		TransactionStatusRejected,
		TransactionStatusFailed:
		return true

	default:
		return false
	}
}

func isTerminalStatus(status TransactionStatus) bool {
	return status == TransactionStatusProcessed ||
		status == TransactionStatusRejected ||
		status == TransactionStatusFailed
}
