package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

type LedgerRepository interface {
	Create(
		ctx context.Context,
		entry domain.WalletLedgerEntry,
	) error

	GetByWalletID(
		ctx context.Context,
		walletID uuid.UUID,
		cursor string,
		limit int,
	) ([]domain.WalletLedgerEntry, string, error)

	GetByTransactionID(
		ctx context.Context,
		transactionID uuid.UUID,
	) (domain.WalletLedgerEntry, error)
}
