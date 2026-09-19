package ports

import (
	"context"
)

type OutboxRepository interface {
	Create(
		ctx context.Context,
		event OutgoingEvent,
	) error

	ClaimBatch(
		ctx context.Context,
		limit int,
		workerID string,
	) ([]OutboxMessage, error)

	MarkPublished(
		ctx context.Context,
		eventID string,
		workerID string,
	) error

	MarkFailed(
		ctx context.Context,
		eventID string,
		workerID string,
		nextAttemptAt string,
		lastError string,
	) error
}

type OutboxMessage struct {
	Event    OutgoingEvent
	Attempts int
}
