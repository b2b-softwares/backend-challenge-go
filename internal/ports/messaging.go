package ports

import "context"

type Message struct {
	ID            string
	Body          []byte
	ReceiptHandle string
}

type MessageConsumer interface {
	Receive(ctx context.Context) ([]Message, error)

	Delete(ctx context.Context, message Message) error

	ChangeVisibility(
		ctx context.Context,
		message Message,
		visibilitySeconds int32,
	) error
}

type EventPublisher interface {
	Publish(ctx context.Context, event OutgoingEvent) error
}

type OutgoingEvent struct {
	EventID       string
	EventType     string
	AggregateID   string
	CorrelationID string
	CausationID   string
	OccurredAt    string
	Version       int
	Payload       []byte
}
