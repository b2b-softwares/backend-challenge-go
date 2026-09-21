package ports

import (
	"context"
	"time"
)

type InboxMessage struct {
	ConsumerName string
	MessageID    string
	MessageType  string
	MessageHash  string
	Payload      []byte
	Status       string
	Attempts     int
	AvailableAt  time.Time
	ReceivedAt   time.Time
	ProcessedAt  *time.Time
	ClaimedBy    string
	ClaimedAt    *time.Time
	LastError    string
}

type InboxRepository interface {
	Find(
		ctx context.Context,
		consumerName string,
		messageID string,
	) (InboxMessage, error)

	Create(
		ctx context.Context,
		message InboxMessage,
	) error

	Claim(
		ctx context.Context,
		consumerName string,
		messageID string,
		workerID string,
	) (InboxMessage, error)

	MarkProcessed(
		ctx context.Context,
		consumerName string,
		messageID string,
		workerID string,
	) error

	MarkFailed(
		ctx context.Context,
		consumerName string,
		messageID string,
		workerID string,
		nextAttemptAt time.Time,
		lastError string,
	) error

	RecoverAbandoned(
		ctx context.Context,
		consumerName string,
		before time.Time,
	) error
}
