package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

type PendingReferenceTransaction struct {
	Transaction domain.WagerTransaction
	Attempts    int
	AvailableAt *time.Time
	ExpiresAt   *time.Time
}

type WagerTransactionRepository interface {
	Create(
		ctx context.Context,
		transaction domain.WagerTransaction,
	) error

	GetByID(
		ctx context.Context,
		transactionID uuid.UUID,
	) (domain.WagerTransaction, error)

	GetByProviderAndExternalTransaction(
		ctx context.Context,
		providerID string,
		externalTransactionID string,
	) (domain.WagerTransaction, error)

	GetByProviderAndExternalTransactionForUpdate(
		ctx context.Context,
		providerID string,
		externalTransactionID string,
	) (domain.WagerTransaction, error)

	GetByProviderAndReference(
		ctx context.Context,
		providerID string,
		referenceExternalID string,
	) (domain.WagerTransaction, error)

	GetByProviderReferenceAndKind(
		ctx context.Context,
		providerID string,
		referenceExternalID string,
		kind domain.TransactionKind,
	) (domain.WagerTransaction, error)

	GetPendingReferenceBatch(
		ctx context.Context,
		limit int,
		now time.Time,
	) ([]PendingReferenceTransaction, error)

	UpdateReferenceRetry(
		ctx context.Context,
		transactionID uuid.UUID,
		attempts int,
		availableAt *time.Time,
	) error

	Update(
		ctx context.Context,
		transaction domain.WagerTransaction,
	) error
}
