package sqs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

type EventPublisher struct {
	client   *sqs.Client
	queueURL string
}

type eventEnvelope struct {
	EventID       string          `json:"eventId"`
	EventType     string          `json:"eventType"`
	AggregateID   string          `json:"aggregateId"`
	CorrelationID string          `json:"correlationId,omitempty"`
	CausationID   string          `json:"causationId,omitempty"`
	OccurredAt    string          `json:"occurredAt"`
	Version       int             `json:"version"`
	Payload       json.RawMessage `json:"payload"`
}

func NewEventPublisher(
	ctx context.Context,
	region string,
	accessKeyID string,
	secretAccessKey string,
	endpointURL string,
	queueURL string,
) (*EventPublisher, error) {
	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				accessKeyID,
				secretAccessKey,
				"",
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS configuration: %w", err)
	}

	if endpointURL != "" {
		cfg.BaseEndpoint = aws.String(endpointURL)
	}

	client := sqs.NewFromConfig(cfg)

	return &EventPublisher{
		client:   client,
		queueURL: queueURL,
	}, nil
}

func (p *EventPublisher) Publish(
	ctx context.Context,
	event ports.OutgoingEvent,
) error {
	if event.EventID == "" {
		return fmt.Errorf("event id is required")
	}

	if event.AggregateID == "" {
		return fmt.Errorf("aggregate id is required for FIFO message group")
	}

	if p.queueURL == "" {
		return fmt.Errorf("SQS queue URL is required")
	}

	payload := json.RawMessage(event.Payload)

	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	if !json.Valid(payload) {
		return fmt.Errorf(
			"event %s has invalid JSON payload",
			event.EventID,
		)
	}

	envelope := eventEnvelope{
		EventID:       event.EventID,
		EventType:     event.EventType,
		AggregateID:   event.AggregateID,
		CorrelationID: event.CorrelationID,
		CausationID:   event.CausationID,
		OccurredAt:    event.OccurredAt,
		Version:       event.Version,
		Payload:       payload,
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf(
			"marshal event %s: %w",
			event.EventID,
			err,
		)
	}

	_, err = p.client.SendMessage(
		ctx,
		&sqs.SendMessageInput{
			QueueUrl:               aws.String(p.queueURL),
			MessageBody:            aws.String(string(body)),
			MessageGroupId:         aws.String(event.AggregateID),
			MessageDeduplicationId: aws.String(event.EventID),
		},
	)
	if err != nil {
		return fmt.Errorf(
			"publish event %s to SQS: %w",
			event.EventID,
			err,
		)
	}

	return nil
}
