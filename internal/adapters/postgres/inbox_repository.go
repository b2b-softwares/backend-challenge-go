package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

type InboxRepository struct {
	db dbtx
}

func NewInboxRepository(db *pgxpool.Pool) *InboxRepository {
	return &InboxRepository{
		db: db,
	}
}

func (r *InboxRepository) resolveDB(ctx context.Context) dbtx {
	txDB, ok := dbtxFromContext(ctx)
	if ok {
		return txDB
	}

	return r.db
}

func (r *InboxRepository) Find(
	ctx context.Context,
	consumerName string,
	messageID string,
) (ports.InboxMessage, error) {
	if consumerName == "" {
		return ports.InboxMessage{}, errors.New("consumer name is required")
	}

	if messageID == "" {
		return ports.InboxMessage{}, errors.New("message id is required")
	}

	db := r.resolveDB(ctx)
	if db == nil {
		return ports.InboxMessage{}, errors.New(
			"inbox repository: database is nil",
		)
	}

	var message ports.InboxMessage
	var claimedBy pgtype.Text
	var lastError pgtype.Text

	err := db.QueryRow(
		ctx,
		`
		SELECT
			consumer_name,
			message_id,
			message_type,
			message_hash,
			payload,
			status,
			attempts,
			available_at,
			received_at,
			processed_at,
			claimed_by,
			claimed_at,
			last_error
		FROM inbox_messages
		WHERE consumer_name = $1
		  AND message_id = $2
		`,
		consumerName,
		messageID,
	).Scan(
		&message.ConsumerName,
		&message.MessageID,
		&message.MessageType,
		&message.MessageHash,
		&message.Payload,
		&message.Status,
		&message.Attempts,
		&message.AvailableAt,
		&message.ReceivedAt,
		&message.ProcessedAt,
		&claimedBy,
		&message.ClaimedAt,
		&lastError,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.InboxMessage{}, pgx.ErrNoRows
		}

		return ports.InboxMessage{}, fmt.Errorf(
			"find inbox message: %w",
			err,
		)
	}

	message.ClaimedBy = nullableTextValue(claimedBy)
	message.LastError = nullableTextValue(lastError)

	return message, nil
}

func (r *InboxRepository) Create(
	ctx context.Context,
	message ports.InboxMessage,
) error {
	if message.ConsumerName == "" {
		return errors.New("consumer name is required")
	}

	if message.MessageID == "" {
		return errors.New("message id is required")
	}

	if message.MessageType == "" {
		return errors.New("message type is required")
	}

	if message.MessageHash == "" {
		return errors.New("message hash is required")
	}

	if len(message.Payload) == 0 {
		return errors.New("payload is required")
	}

	db := r.resolveDB(ctx)
	if db == nil {
		return errors.New("inbox repository: database is nil")
	}

	receivedAt := message.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}

	availableAt := message.AvailableAt
	if availableAt.IsZero() {
		availableAt = receivedAt
	}

	attempts := message.Attempts
	if attempts < 0 {
		return errors.New("attempts cannot be negative")
	}

	status := message.Status
	if status == "" {
		status = "RECEIVED"
	}

	// A newly persisted inbox message must enter RECEIVED state.
	// Claim is responsible for the transition RECEIVED -> PROCESSING.
	if status == "PROCESSING" {
		status = "RECEIVED"
	}

	_, err := db.Exec(
		ctx,
		`
		INSERT INTO inbox_messages (
			consumer_name,
			message_id,
			message_type,
			event_id,
			payload,
			status,
			attempts,
			available_at,
			received_at,
			processed_at,
			claimed_by,
			claimed_at,
			last_error,
			message_hash
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			$13,
			$14
		)
		`,
		message.ConsumerName,
		message.MessageID,
		message.MessageType,
		nil,
		message.Payload,
		status,
		attempts,
		availableAt,
		receivedAt,
		message.ProcessedAt,
		nullIfEmpty(message.ClaimedBy),
		message.ClaimedAt,
		nullIfEmpty(message.LastError),
		message.MessageHash,
	)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "inbox_messages_consumer_message_pkey" {
			return fmt.Errorf(
				"message already exists: consumer=%s message=%s",
				message.ConsumerName,
				message.MessageID,
			)
		}

		return fmt.Errorf(
			"create inbox message: %w",
			err,
		)
	}

	return nil
}

func (r *InboxRepository) Claim(
	ctx context.Context,
	consumerName string,
	messageID string,
	workerID string,
) (ports.InboxMessage, error) {
	if consumerName == "" {
		return ports.InboxMessage{}, errors.New("consumer name is required")
	}

	if messageID == "" {
		return ports.InboxMessage{}, errors.New("message id is required")
	}

	if workerID == "" {
		return ports.InboxMessage{}, errors.New("worker id is required")
	}

	db := r.resolveDB(ctx)
	if db == nil {
		return ports.InboxMessage{}, errors.New(
			"inbox repository: database is nil",
		)
	}

	var message ports.InboxMessage
	var claimedBy pgtype.Text
	var lastError pgtype.Text

	err := db.QueryRow(
		ctx,
		`
		UPDATE inbox_messages
		SET
			status = 'PROCESSING',
			attempts = attempts + 1,
			claimed_by = $3,
			claimed_at = NOW(),
			last_error = NULL
		WHERE consumer_name = $1
		  AND message_id = $2
		  AND status IN ('RECEIVED', 'FAILED')
		  AND available_at <= NOW()
		RETURNING
			consumer_name,
			message_id,
			message_type,
			message_hash,
			payload,
			status,
			attempts,
			available_at,
			received_at,
			processed_at,
			claimed_by,
			claimed_at,
			last_error
		`,
		consumerName,
		messageID,
		workerID,
	).Scan(
		&message.ConsumerName,
		&message.MessageID,
		&message.MessageType,
		&message.MessageHash,
		&message.Payload,
		&message.Status,
		&message.Attempts,
		&message.AvailableAt,
		&message.ReceivedAt,
		&message.ProcessedAt,
		&claimedBy,
		&message.ClaimedAt,
		&lastError,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.InboxMessage{}, pgx.ErrNoRows
		}

		return ports.InboxMessage{}, fmt.Errorf(
			"claim inbox message: %w",
			err,
		)
	}

	message.ClaimedBy = nullableTextValue(claimedBy)
	message.LastError = nullableTextValue(lastError)

	return message, nil
}

func (r *InboxRepository) MarkProcessed(
	ctx context.Context,
	consumerName string,
	messageID string,
	workerID string,
) error {
	if consumerName == "" {
		return errors.New("consumer name is required")
	}

	if messageID == "" {
		return errors.New("message id is required")
	}

	if workerID == "" {
		return errors.New("worker id is required")
	}

	db := r.resolveDB(ctx)
	if db == nil {
		return errors.New("inbox repository: database is nil")
	}

	commandTag, err := db.Exec(
		ctx,
		`
		UPDATE inbox_messages
		SET
			status = 'PROCESSED',
			processed_at = NOW(),
			claimed_by = NULL,
			claimed_at = NULL,
			last_error = NULL
		WHERE consumer_name = $1
		  AND message_id = $2
		  AND status = 'PROCESSING'
		  AND claimed_by = $3
		`,
		consumerName,
		messageID,
		workerID,
	)
	if err != nil {
		return fmt.Errorf(
			"mark inbox message as processed: %w",
			err,
		)
	}

	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf(
			"mark inbox message as processed: message not found or not owned by worker",
		)
	}

	return nil
}

func (r *InboxRepository) MarkFailed(
	ctx context.Context,
	consumerName string,
	messageID string,
	workerID string,
	nextAttemptAt time.Time,
	lastError string,
) error {
	if consumerName == "" {
		return errors.New("consumer name is required")
	}

	if messageID == "" {
		return errors.New("message id is required")
	}

	if workerID == "" {
		return errors.New("worker id is required")
	}

	if nextAttemptAt.IsZero() {
		return errors.New("next attempt time is required")
	}

	if lastError == "" {
		return errors.New("last error is required")
	}

	db := r.resolveDB(ctx)
	if db == nil {
		return errors.New("inbox repository: database is nil")
	}

	commandTag, err := db.Exec(
		ctx,
		`
		UPDATE inbox_messages
		SET
			status = 'FAILED',
			available_at = $4,
			claimed_by = NULL,
			claimed_at = NULL,
			last_error = $5
		WHERE consumer_name = $1
		  AND message_id = $2
		  AND status = 'PROCESSING'
		  AND claimed_by = $3
		`,
		consumerName,
		messageID,
		workerID,
		nextAttemptAt,
		lastError,
	)
	if err != nil {
		return fmt.Errorf(
			"mark inbox message as failed: %w",
			err,
		)
	}

	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf(
			"mark inbox message as failed: message not found or not owned by worker",
		)
	}

	return nil
}

func (r *InboxRepository) RecoverAbandoned(
	ctx context.Context,
	consumerName string,
	before time.Time,
) error {
	if consumerName == "" {
		return errors.New("consumer name is required")
	}

	if before.IsZero() {
		return errors.New("recovery cutoff time is required")
	}

	db := r.resolveDB(ctx)
	if db == nil {
		return errors.New("inbox repository: database is nil")
	}

	_, err := db.Exec(
		ctx,
		`
		UPDATE inbox_messages
		SET
			status = 'FAILED',
			available_at = NOW(),
			claimed_by = NULL,
			claimed_at = NULL,
			last_error = 'processing claim expired and was recovered'
		WHERE consumer_name = $1
		  AND status = 'PROCESSING'
		  AND claimed_at IS NOT NULL
		  AND claimed_at < $2
		`,
		consumerName,
		before,
	)
	if err != nil {
		return fmt.Errorf(
			"recover abandoned inbox messages: %w",
			err,
		)
	}

	return nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func nullableTextValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}

	return value.String
}
