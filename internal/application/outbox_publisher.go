package application

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

const (
	defaultOutboxPollInterval = 2 * time.Second
	defaultOutboxBatchSize    = 50

	maxOutboxRetryDelay = 60 * time.Second
)

type OutboxPublisher struct {
	outbox         ports.OutboxRepository
	eventPublisher ports.EventPublisher
	pollInterval   time.Duration
	batchSize      int
	workerID       string
}

func NewOutboxPublisher(
	outbox ports.OutboxRepository,
	eventPublisher ports.EventPublisher,
	pollInterval time.Duration,
	batchSize int,
	workerID string,
) *OutboxPublisher {
	if pollInterval <= 0 {
		pollInterval = defaultOutboxPollInterval
	}

	if batchSize <= 0 {
		batchSize = defaultOutboxBatchSize
	}

	if batchSize > 100 {
		batchSize = 100
	}

	if workerID == "" {
		workerID = "outbox-publisher"
	}

	workerID = fmt.Sprintf(
		"%s-%s",
		workerID,
		uuid.NewString(),
	)

	return &OutboxPublisher{
		outbox:         outbox,
		eventPublisher: eventPublisher,
		pollInterval:   pollInterval,
		batchSize:      batchSize,
		workerID:       workerID,
	}
}

func (p *OutboxPublisher) Run(ctx context.Context) error {
	if p.outbox == nil {
		return fmt.Errorf("outbox repository is required")
	}

	if p.eventPublisher == nil {
		return fmt.Errorf("event publisher is required")
	}

	if p.workerID == "" {
		return fmt.Errorf("outbox publisher worker id is required")
	}

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		if err := p.publishBatch(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			log.Printf(
				"outbox publisher batch failed: %v",
				err,
			)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
		}
	}
}

func (p *OutboxPublisher) publishBatch(ctx context.Context) error {
	messages, err := p.outbox.ClaimBatch(
		ctx,
		p.batchSize,
		p.workerID,
	)
	if err != nil {
		return fmt.Errorf("claim outbox batch: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	var firstErr error

	for _, message := range messages {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := p.publishMessage(ctx, message); err != nil {
			if firstErr == nil {
				firstErr = err
			}

			log.Printf(
				"outbox event %s failed: %v",
				message.Event.EventID,
				err,
			)

			continue
		}

		log.Printf(
			"outbox event %s published successfully",
			message.Event.EventID,
		)
	}

	return firstErr
}

func (p *OutboxPublisher) publishMessage(
	ctx context.Context,
	message ports.OutboxMessage,
) error {
	eventID := message.Event.EventID

	err := p.eventPublisher.Publish(
		ctx,
		message.Event,
	)
	if err == nil {
		if markErr := p.outbox.MarkPublished(
			ctx,
			eventID,
			p.workerID,
		); markErr != nil {
			return fmt.Errorf(
				"mark event %s as published: %w",
				eventID,
				markErr,
			)
		}

		return nil
	}

	nextAttemptAt := time.Now().UTC().Add(
		outboxRetryDelay(message.Attempts),
	)

	markErr := p.outbox.MarkFailed(
		ctx,
		eventID,
		p.workerID,
		nextAttemptAt.Format(time.RFC3339),
		err.Error(),
	)

	if markErr != nil {
		return fmt.Errorf(
			"publish event %s: %v; mark failed: %w",
			eventID,
			err,
			markErr,
		)
	}

	return fmt.Errorf(
		"publish event %s: %w",
		eventID,
		err,
	)
}

func outboxRetryDelay(attempts int) time.Duration {
	if attempts <= 1 {
		return 2 * time.Second
	}

	delay := 2 * time.Second

	for i := 1; i < attempts; i++ {
		delay *= 2

		if delay >= maxOutboxRetryDelay {
			return maxOutboxRetryDelay
		}
	}

	return delay
}
