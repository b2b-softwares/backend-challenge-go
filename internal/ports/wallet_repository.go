package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

type WalletRepository interface {
	Create(ctx context.Context, wallet domain.Wallet) error

	GetByID(
		ctx context.Context,
		walletID uuid.UUID,
	) (domain.Wallet, error)

	GetByPlayerAndCurrency(
		ctx context.Context,
		playerID uuid.UUID,
		currency domain.Currency,
	) (domain.Wallet, error)

	UpdateBalance(
		ctx context.Context,
		wallet domain.Wallet,
		expectedVersion int64,
	) error
}
