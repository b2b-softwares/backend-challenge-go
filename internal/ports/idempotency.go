package ports

import (
	"context"

	"github.com/google/uuid"
)

type IdempotencyRecord struct {
	ProviderID              string
	IdempotencyKey          string
	PayloadHash             string
	TransactionID           uuid.UUID
	Status                  string
	ResponseBody            []byte
	ObservedBalanceAmount   int64
	ObservedBalanceCurrency string
}

type IdempotencyRepository interface {
	Find(
		ctx context.Context,
		providerID string,
		idempotencyKey string,
	) (IdempotencyRecord, error)

	Create(
		ctx context.Context,
		record IdempotencyRecord,
	) error

	Update(
		ctx context.Context,
		record IdempotencyRecord,
	) error
}
