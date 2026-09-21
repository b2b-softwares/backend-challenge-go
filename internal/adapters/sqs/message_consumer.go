package sqs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

const (
	defaultMaxNumberOfMessages = int32(10)
	defaultWaitTimeSeconds     = int32(10)
	defaultVisibilitySeconds   = int32(30)
)

type MessageConsumer struct {
	client              *sqs.Client
	queueURL            string
	maxNumberOfMessages int32
	waitTimeSeconds     int32
	visibilitySeconds   int32
}

func NewMessageConsumer(
	ctx context.Context,
	region string,
	accessKeyID string,
	secretAccessKey string,
	endpointURL string,
	queueURL string,
) (*MessageConsumer, error) {
	if queueURL == "" {
		return nil, fmt.Errorf("SQS queue URL is required")
	}

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

	return &MessageConsumer{
		client:              client,
		queueURL:            queueURL,
		maxNumberOfMessages: defaultMaxNumberOfMessages,
		waitTimeSeconds:     defaultWaitTimeSeconds,
		visibilitySeconds:   defaultVisibilitySeconds,
	}, nil
}

func (c *MessageConsumer) Receive(
	ctx context.Context,
) ([]ports.Message, error) {
	if c.queueURL == "" {
		return nil, fmt.Errorf("SQS queue URL is required")
	}

	output, err := c.client.ReceiveMessage(
		ctx,
		&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.queueURL),
			MaxNumberOfMessages: c.maxNumberOfMessages,
			WaitTimeSeconds:     c.waitTimeSeconds,
			VisibilityTimeout:   c.visibilitySeconds,
			MessageAttributeNames: []string{
				"All",
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("receive SQS messages: %w", err)
	}

	messages := make([]ports.Message, 0, len(output.Messages))

	for _, message := range output.Messages {
		if message.MessageId == nil || *message.MessageId == "" {
			return nil, fmt.Errorf("received SQS message without message id")
		}

		if message.ReceiptHandle == nil || *message.ReceiptHandle == "" {
			return nil, fmt.Errorf(
				"received SQS message %s without receipt handle",
				*message.MessageId,
			)
		}

		body := []byte{}
		if message.Body != nil {
			body = []byte(*message.Body)
		}

		messages = append(
			messages,
			ports.Message{
				ID:            *message.MessageId,
				Body:          body,
				ReceiptHandle: *message.ReceiptHandle,
			},
		)
	}

	return messages, nil
}

func (c *MessageConsumer) Delete(
	ctx context.Context,
	message ports.Message,
) error {
	if message.ReceiptHandle == "" {
		return fmt.Errorf(
			"receipt handle is required for message %s",
			message.ID,
		)
	}

	_, err := c.client.DeleteMessage(
		ctx,
		&sqs.DeleteMessageInput{
			QueueUrl:      aws.String(c.queueURL),
			ReceiptHandle: aws.String(message.ReceiptHandle),
		},
	)
	if err != nil {
		return fmt.Errorf(
			"delete SQS message %s: %w",
			message.ID,
			err,
		)
	}

	return nil
}

func (c *MessageConsumer) ChangeVisibility(
	ctx context.Context,
	message ports.Message,
	visibilitySeconds int32,
) error {
	if message.ReceiptHandle == "" {
		return fmt.Errorf(
			"receipt handle is required for message %s",
			message.ID,
		)
	}

	if visibilitySeconds < 0 {
		return fmt.Errorf(
			"visibility timeout cannot be negative",
		)
	}

	_, err := c.client.ChangeMessageVisibility(
		ctx,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(c.queueURL),
			ReceiptHandle:     aws.String(message.ReceiptHandle),
			VisibilityTimeout: visibilitySeconds,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"change visibility for SQS message %s: %w",
			message.ID,
			err,
		)
	}

	return nil
}
