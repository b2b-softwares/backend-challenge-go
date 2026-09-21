package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	otmetric "go.opentelemetry.io/otel/metric"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
	"github.com/junglegaming/backend-challenge-go/internal/observability"
	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

const (
	wagerConsumerName = "wager-transaction-consumer"
	wagerMessageType  = "WagerTransactionRequested"

	inboxStatusReceived   = "RECEIVED"
	inboxStatusProcessing = "PROCESSING"
	inboxStatusProcessed  = "PROCESSED"
)

type wagerConsumerMetrics struct {
	sqsMessagesReceivedTotal    otmetric.Int64Counter
	sqsMessagesProcessedTotal   otmetric.Int64Counter
	sqsMessagesFailedTotal      otmetric.Int64Counter
	sqsMessagesDeletedTotal     otmetric.Int64Counter
	sqsProcessingDuration       otmetric.Float64Histogram
	inboxMessagesReceivedTotal  otmetric.Int64Counter
	inboxMessagesProcessedTotal otmetric.Int64Counter
	inboxMessagesFailedTotal    otmetric.Int64Counter
	inboxDuplicateTotal         otmetric.Int64Counter
}

func newWagerConsumerMetrics(
	providers *observability.Providers,
) *wagerConsumerMetrics {
	meter := observability.Meter(providers)

	sqsMessagesReceivedTotal, _ := meter.Int64Counter(
		"sqs_messages_received_total",
		otmetric.WithDescription(
			"Total number of messages received from SQS.",
		),
	)

	sqsMessagesProcessedTotal, _ := meter.Int64Counter(
		"sqs_messages_processed_total",
		otmetric.WithDescription(
			"Total number of SQS messages successfully processed.",
		),
	)

	sqsMessagesFailedTotal, _ := meter.Int64Counter(
		"sqs_messages_failed_total",
		otmetric.WithDescription(
			"Total number of SQS messages that failed processing.",
		),
	)

	sqsMessagesDeletedTotal, _ := meter.Int64Counter(
		"sqs_messages_deleted_total",
		otmetric.WithDescription(
			"Total number of SQS messages successfully deleted.",
		),
	)

	sqsProcessingDuration, _ := meter.Float64Histogram(
		"sqs_message_processing_duration_seconds",
		otmetric.WithDescription(
			"Time spent processing SQS wager messages.",
		),
		otmetric.WithUnit("s"),
	)

	inboxMessagesReceivedTotal, _ := meter.Int64Counter(
		"inbox_messages_received_total",
		otmetric.WithDescription(
			"Total number of wager messages received by the Inbox.",
		),
	)

	inboxMessagesProcessedTotal, _ := meter.Int64Counter(
		"inbox_messages_processed_total",
		otmetric.WithDescription(
			"Total number of Inbox messages successfully processed.",
		),
	)

	inboxMessagesFailedTotal, _ := meter.Int64Counter(
		"inbox_messages_failed_total",
		otmetric.WithDescription(
			"Total number of Inbox message processing failures.",
		),
	)

	inboxDuplicateTotal, _ := meter.Int64Counter(
		"inbox_duplicate_total",
		otmetric.WithDescription(
			"Total number of duplicate Inbox messages detected.",
		),
	)

	return &wagerConsumerMetrics{
		sqsMessagesReceivedTotal:    sqsMessagesReceivedTotal,
		sqsMessagesProcessedTotal:   sqsMessagesProcessedTotal,
		sqsMessagesFailedTotal:      sqsMessagesFailedTotal,
		sqsMessagesDeletedTotal:     sqsMessagesDeletedTotal,
		sqsProcessingDuration:       sqsProcessingDuration,
		inboxMessagesReceivedTotal:  inboxMessagesReceivedTotal,
		inboxMessagesProcessedTotal: inboxMessagesProcessedTotal,
		inboxMessagesFailedTotal:    inboxMessagesFailedTotal,
		inboxDuplicateTotal:         inboxDuplicateTotal,
	}
}

type WagerConsumer struct {
	messages      ports.MessageConsumer
	inbox         ports.InboxRepository
	transactionDB ports.TransactionManager
	wagerService  *WagerService
	workerID      string
	metrics       *wagerConsumerMetrics
}

func NewWagerConsumer(
	messages ports.MessageConsumer,
	inbox ports.InboxRepository,
	transactionDB ports.TransactionManager,
	wagerService *WagerService,
	providers *observability.Providers,
) *WagerConsumer {
	return &WagerConsumer{
		messages:      messages,
		inbox:         inbox,
		transactionDB: transactionDB,
		wagerService:  wagerService,
		workerID:      uuid.New().String(),
		metrics:       newWagerConsumerMetrics(providers),
	}
}

type wagerTransactionMessage struct {
	MessageID  string               `json:"messageId"`
	Type       string               `json:"type"`
	OccurredAt time.Time            `json:"occurredAt"`
	Data       wagerTransactionData `json:"data"`
}

type wagerTransactionData struct {
	ProviderID            string     `json:"providerId"`
	ExternalTransactionID string     `json:"externalTransactionId"`
	IdempotencyKey        string     `json:"idempotencyKey"`
	PlayerID              uuid.UUID  `json:"playerId"`
	WalletID              uuid.UUID  `json:"walletId"`
	RoundID               string     `json:"roundId"`
	GameID                string     `json:"gameId"`
	Kind                  string     `json:"kind"`
	Money                 wagerMoney `json:"money"`
}

type wagerMoney struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

func (c *WagerConsumer) Run(ctx context.Context) error {
	if c == nil ||
		c.messages == nil ||
		c.inbox == nil ||
		c.transactionDB == nil ||
		c.wagerService == nil {
		return errors.New("wager consumer: dependencies are required")
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		messages, err := c.messages.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			return fmt.Errorf("receive wager messages: %w", err)
		}

		for _, message := range messages {
			c.recordSQSReceived(ctx)

			if err := c.processMessage(ctx, message); err != nil {
				c.recordSQSFailed(ctx)

				if ctx.Err() != nil {
					return ctx.Err()
				}

				log.Printf(
					"wager consumer failed to process message id=%s: %v",
					message.ID,
					err,
				)

				// A mensagem não é removida em caso de erro.
				// O SQS fará o redelivery e, após maxReceiveCount,
				// encaminhará para a DLQ configurada.
				continue
			}

			c.recordSQSProcessed(ctx)
		}
	}
}

func (c *WagerConsumer) processMessage(
	ctx context.Context,
	message ports.Message,
) (err error) {
	startedAt := time.Now()

	defer func() {
		if c == nil || c.metrics == nil {
			return
		}

		status := "processed"

		if err != nil {
			status = "failed"
		}

		c.metrics.sqsProcessingDuration.Record(
			ctx,
			time.Since(startedAt).Seconds(),
			otmetric.WithAttributes(
				attribute.String("status", status),
			),
		)
	}()

	if message.ID == "" {
		return errors.New("wager consumer: SQS message id is required")
	}

	if len(message.Body) == 0 {
		return fmt.Errorf(
			"wager consumer: message %s has empty body",
			message.ID,
		)
	}

	payloadHash := hashPayload(message.Body)

	var envelope wagerTransactionMessage

	if err := json.Unmarshal(message.Body, &envelope); err != nil {
		return fmt.Errorf(
			"message %s has invalid JSON: %w",
			message.ID,
			err,
		)
	}

	if err := validateWagerMessage(envelope); err != nil {
		return fmt.Errorf(
			"message %s is invalid: %w",
			message.ID,
			err,
		)
	}

	/*
		============================================================
		1. ENSURE INBOX MESSAGE
		============================================================

		A persistência inicial do Inbox precisa ser confirmada
		em uma transação independente.

		Não podemos executar Create + Claim na mesma transação,
		porque se Claim falhar o rollback apagaria o registro
		recém-criado.
	*/
	processed := false

	err = c.transactionDB.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			existing, err := c.inbox.Find(
				txCtx,
				wagerConsumerName,
				envelope.MessageID,
			)

			if err == nil {
				c.recordInboxReceived(ctx)

				if existing.MessageHash != payloadHash {
					return fmt.Errorf(
						"message %s was redelivered with different payload",
						envelope.MessageID,
					)
				}

				if existing.Status == inboxStatusProcessed {
					processed = true
					c.recordInboxDuplicate(ctx)
				}

				return nil
			}

			if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf(
					"find inbox message %s: %w",
					envelope.MessageID,
					err,
				)
			}

			now := time.Now().UTC()

			if err := c.inbox.Create(
				txCtx,
				ports.InboxMessage{
					ConsumerName: wagerConsumerName,
					MessageID:    envelope.MessageID,
					MessageType:  envelope.Type,
					MessageHash:  payloadHash,
					Payload:      append([]byte(nil), message.Body...),
					Status:       inboxStatusReceived,
					Attempts:     0,
					AvailableAt:  now,
					ReceivedAt:   now,
				},
			); err != nil {
				return fmt.Errorf(
					"create inbox message %s: %w",
					envelope.MessageID,
					err,
				)
			}

			c.recordInboxReceived(ctx)

			return nil
		},
	)
	if err != nil {
		c.recordInboxFailed(ctx)
		return err
	}

	/*
		Se a mensagem já foi processada anteriormente, o processamento
		financeiro não precisa acontecer novamente.

		A mensagem SQS pode ser removida com segurança porque o Inbox
		é a fonte persistente de idempotência do consumidor.
	*/
	if processed {
		if err := c.messages.Delete(ctx, message); err != nil {
			return fmt.Errorf(
				"delete already processed SQS message %s: %w",
				message.ID,
				err,
			)
		}

		c.recordSQSDeleted(ctx)
		c.recordInboxProcessed(ctx)

		return nil
	}

	/*
		============================================================
		2. CLAIM INBOX MESSAGE
		============================================================

		O Claim acontece em uma transação separada.

		Assim, o estado:

			RECEIVED -> PROCESSING

		fica efetivamente persistido antes de iniciar o
		processamento financeiro.
	*/
	var claimed ports.InboxMessage

	err = c.transactionDB.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			var err error

			claimed, err = c.inbox.Claim(
				txCtx,
				wagerConsumerName,
				envelope.MessageID,
				c.workerID,
			)
			if err != nil {
				return fmt.Errorf(
					"claim inbox message %s: %w",
					envelope.MessageID,
					err,
				)
			}

			if claimed.MessageHash != payloadHash {
				return fmt.Errorf(
					"message %s payload hash changed",
					envelope.MessageID,
				)
			}

			if claimed.Status == inboxStatusProcessed {
				processed = true
			}

			return nil
		},
	)
	if err != nil {
		c.recordInboxFailed(ctx)
		return err
	}

	if processed {
		c.recordInboxDuplicate(ctx)

		if err := c.messages.Delete(ctx, message); err != nil {
			return fmt.Errorf(
				"delete already processed SQS message %s: %w",
				message.ID,
				err,
			)
		}

		c.recordSQSDeleted(ctx)
		c.recordInboxProcessed(ctx)

		return nil
	}

	/*
		============================================================
		3. BUSINESS TRANSACTION
		============================================================

		PlaceBet e MarkProcessed permanecem na mesma transação.

		Isso garante que não exista o cenário:

			débito confirmado
			+
			inbox ainda PROCESSING

		se o commit do banco falhar.

		Se qualquer parte falhar, toda a transação financeira
		é revertida e a mensagem permanece no SQS para redelivery.
	*/
	err = c.transactionDB.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			amount, err := domain.NewMoney(
				envelope.Data.Money.Amount,
				domain.Currency(envelope.Data.Money.Currency),
			)
			if err != nil {
				return fmt.Errorf(
					"parse money for message %s: %w",
					envelope.MessageID,
					err,
				)
			}

			input := PlaceBetInput{
				ID:                    uuid.New(),
				ExternalTransactionID: envelope.Data.ExternalTransactionID,
				ProviderID:            envelope.Data.ProviderID,
				IdempotencyKey:        envelope.Data.IdempotencyKey,
				PayloadHash:           payloadHash,
				WalletID:              envelope.Data.WalletID,
				PlayerID:              envelope.Data.PlayerID,
				RoundID:               envelope.Data.RoundID,
				GameID:                envelope.Data.GameID,
				Amount:                amount,
			}

			_, err = c.wagerService.PlaceBet(
				txCtx,
				input,
			)
			if err != nil &&
				!errors.Is(err, domain.ErrInsufficientBalance) {
				return fmt.Errorf(
					"process wager message %s: %w",
					envelope.MessageID,
					err,
				)
			}

			if err := c.inbox.MarkProcessed(
				txCtx,
				wagerConsumerName,
				envelope.MessageID,
				c.workerID,
			); err != nil {
				return fmt.Errorf(
					"mark inbox message %s as processed: %w",
					envelope.MessageID,
					err,
				)
			}

			return nil
		},
	)
	if err != nil {
		c.recordInboxFailed(ctx)
		return err
	}

	/*
		============================================================
		4. DELETE FROM SQS
		============================================================

		Somente depois do commit da transação financeira + Inbox
		a mensagem é removida do SQS.

		Se o Delete falhar, o SQS fará redelivery. O Inbox já estará
		PROCESSED e a segunda entrega será tratada como idempotente.
	*/
	if err := c.messages.Delete(ctx, message); err != nil {
		return fmt.Errorf(
			"delete processed SQS message %s: %w",
			message.ID,
			err,
		)
	}

	c.recordSQSDeleted(ctx)
	c.recordInboxProcessed(ctx)

	return nil
}

func (c *WagerConsumer) recordSQSReceived(ctx context.Context) {
	if c == nil || c.metrics == nil {
		return
	}

	c.metrics.sqsMessagesReceivedTotal.Add(ctx, 1)
}

func (c *WagerConsumer) recordSQSProcessed(ctx context.Context) {
	if c == nil || c.metrics == nil {
		return
	}

	c.metrics.sqsMessagesProcessedTotal.Add(ctx, 1)
}

func (c *WagerConsumer) recordSQSFailed(ctx context.Context) {
	if c == nil || c.metrics == nil {
		return
	}

	c.metrics.sqsMessagesFailedTotal.Add(ctx, 1)
}

func (c *WagerConsumer) recordSQSDeleted(ctx context.Context) {
	if c == nil || c.metrics == nil {
		return
	}

	c.metrics.sqsMessagesDeletedTotal.Add(ctx, 1)
}

func (c *WagerConsumer) recordInboxReceived(ctx context.Context) {
	if c == nil || c.metrics == nil {
		return
	}

	c.metrics.inboxMessagesReceivedTotal.Add(ctx, 1)
}

func (c *WagerConsumer) recordInboxProcessed(ctx context.Context) {
	if c == nil || c.metrics == nil {
		return
	}

	c.metrics.inboxMessagesProcessedTotal.Add(ctx, 1)
}

func (c *WagerConsumer) recordInboxFailed(ctx context.Context) {
	if c == nil || c.metrics == nil {
		return
	}

	c.metrics.inboxMessagesFailedTotal.Add(ctx, 1)
}

func (c *WagerConsumer) recordInboxDuplicate(ctx context.Context) {
	if c == nil || c.metrics == nil {
		return
	}

	c.metrics.inboxDuplicateTotal.Add(ctx, 1)
}

func validateWagerMessage(
	message wagerTransactionMessage,
) error {
	if message.MessageID == "" {
		return errors.New("messageId is required")
	}

	if message.Type != wagerMessageType {
		return fmt.Errorf(
			"unsupported message type %q",
			message.Type,
		)
	}

	if message.OccurredAt.IsZero() {
		return errors.New("occurredAt is required")
	}

	data := message.Data

	if data.ProviderID == "" {
		return errors.New("providerId is required")
	}

	if data.ExternalTransactionID == "" {
		return errors.New("externalTransactionId is required")
	}

	if data.IdempotencyKey == "" {
		return errors.New("idempotencyKey is required")
	}

	if data.PlayerID == uuid.Nil {
		return errors.New("playerId is required")
	}

	if data.WalletID == uuid.Nil {
		return errors.New("walletId is required")
	}

	if data.RoundID == "" {
		return errors.New("roundId is required")
	}

	if data.GameID == "" {
		return errors.New("gameId is required")
	}

	if data.Kind != "BET" {
		return fmt.Errorf(
			"unsupported transaction kind %q",
			data.Kind,
		)
	}

	if data.Money.Amount == "" {
		return errors.New("money.amount is required")
	}

	if data.Money.Currency == "" {
		return errors.New("money.currency is required")
	}

	return nil
}

func hashPayload(payload []byte) string {
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}
