package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

const (
	defaultOutboxClaimLimit = 50
	maxOutboxClaimLimit     = 100
)

// OutboxRepository persists and claims transactional outbox events.
//
// The repository is intentionally built on top of dbtx so that all operations
// can participate in the TransactionManager when the context carries an
// active PostgreSQL transaction.
type OutboxRepository struct {
	db dbtx
}

func NewOutboxRepository(db *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{
		db: db,
	}
}

func newOutboxRepositoryWithDBTX(db dbtx) *OutboxRepository {
	return &OutboxRepository{
		db: db,
	}
}

func (r *OutboxRepository) resolveDB(ctx context.Context) dbtx {
	if tx, ok := dbtxFromContext(ctx); ok {
		return tx
	}

	return r.db
}

func (r *OutboxRepository) Create(
	ctx context.Context,
	event ports.OutgoingEvent,
) error {
	if r == nil || r.db == nil {
		return errors.New("outbox repository: database is nil")
	}

	if err := validateOutgoingEvent(event); err != nil {
		return err
	}

	occurredAt, err := parseEventTime(event.OccurredAt)
	if err != nil {
		return err
	}

	db := r.resolveDB(ctx)

	_, err = db.Exec(
		ctx,
		`
		INSERT INTO outbox_events (
			event_id,
			event_type,
			aggregate_id,
			correlation_id,
			causation_id,
			occurred_at,
			version,
			payload,
			available_at
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8::jsonb,
			NOW()
		)
		`,
		event.EventID,
		event.EventType,
		event.AggregateID,
		event.CorrelationID,
		event.CausationID,
		occurredAt,
		event.Version,
		event.Payload,
	)
	if err != nil {
		return mapOutboxRepositoryError(err)
	}

	return nil
}

func (r *OutboxRepository) ClaimBatch(
	ctx context.Context,
	limit int,
	workerID string,
) ([]ports.OutboxMessage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("outbox repository: database is nil")
	}

	if workerID == "" {
		return nil, errors.New("outbox repository: worker id is required")
	}

	if limit <= 0 {
		limit = defaultOutboxClaimLimit
	}

	if limit > maxOutboxClaimLimit {
		limit = maxOutboxClaimLimit
	}

	db := r.resolveDB(ctx)

	rows, err := db.Query(
		ctx,
		`
		WITH candidates AS (
			SELECT event_id
			FROM outbox_events
			WHERE published_at IS NULL
			  AND available_at <= NOW()
			  AND (
				claimed_by IS NULL
				OR claimed_at IS NULL
				OR claimed_at < NOW() - INTERVAL '5 minutes'
			  )
			ORDER BY available_at, occurred_at, event_id
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE outbox_events AS o
		SET
			claimed_by = $2,
			claimed_at = NOW(),
			attempts = o.attempts + 1
		FROM candidates AS c
		WHERE o.event_id = c.event_id
		RETURNING
			o.event_id,
			o.event_type,
			o.aggregate_id,
			o.correlation_id,
			o.causation_id,
			o.occurred_at,
			o.version,
			o.payload,
			o.attempts
		`,
		limit,
		workerID,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	messages := make([]ports.OutboxMessage, 0, limit)

	for rows.Next() {
		message, err := scanOutboxMessage(rows)
		if err != nil {
			return nil, err
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

func (r *OutboxRepository) MarkPublished(
	ctx context.Context,
	eventID string,
	workerID string,
) error {
	if r == nil || r.db == nil {
		return errors.New("outbox repository: database is nil")
	}

	id, err := parseEventID(eventID)
	if err != nil {
		return err
	}

	if workerID == "" {
		return errors.New("outbox repository: worker id is required")
	}

	db := r.resolveDB(ctx)

	tag, err := db.Exec(
		ctx,
		`
		UPDATE outbox_events
		SET
			published_at = NOW(),
			claimed_by = NULL,
			claimed_at = NULL,
			last_error = NULL
		WHERE event_id = $1
		  AND claimed_by = $2
		  AND published_at IS NULL
		`,
		id,
		workerID,
	)
	if err != nil {
		return mapOutboxRepositoryError(err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *OutboxRepository) MarkFailed(
	ctx context.Context,
	eventID string,
	workerID string,
	nextAttemptAt string,
	lastError string,
) error {
	if r == nil || r.db == nil {
		return errors.New("outbox repository: database is nil")
	}

	id, err := parseEventID(eventID)
	if err != nil {
		return err
	}

	if workerID == "" {
		return errors.New("outbox repository: worker id is required")
	}

	if lastError == "" {
		return errors.New("outbox repository: last error is required")
	}

	nextAttempt, err := parseEventTime(nextAttemptAt)
	if err != nil {
		return err
	}

	db := r.resolveDB(ctx)

	tag, err := db.Exec(
		ctx,
		`
		UPDATE outbox_events
		SET
			available_at = $3,
			claimed_by = NULL,
			claimed_at = NULL,
			last_error = $4
		WHERE event_id = $1
		  AND claimed_by = $2
		  AND published_at IS NULL
		`,
		id,
		workerID,
		nextAttempt,
		lastError,
	)
	if err != nil {
		return mapOutboxRepositoryError(err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

type outboxScanner interface {
	Scan(dest ...any) error
}

func scanOutboxMessage(row outboxScanner) (ports.OutboxMessage, error) {
	var (
		eventID       uuid.UUID
		eventType     string
		aggregateID   string
		correlationID string
		causationID   string
		occurredAt    time.Time
		version       int
		payload       []byte
		attempts      int
	)

	if err := row.Scan(
		&eventID,
		&eventType,
		&aggregateID,
		&correlationID,
		&causationID,
		&occurredAt,
		&version,
		&payload,
		&attempts,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.OutboxMessage{}, pgx.ErrNoRows
		}

		return ports.OutboxMessage{}, err
	}

	event := ports.OutgoingEvent{
		EventID:       eventID.String(),
		EventType:     eventType,
		AggregateID:   aggregateID,
		CorrelationID: correlationID,
		CausationID:   causationID,
		OccurredAt:    occurredAt.UTC().Format(time.RFC3339Nano),
		Version:       version,
		Payload:       append([]byte(nil), payload...),
	}

	if err := validateOutgoingEvent(event); err != nil {
		return ports.OutboxMessage{}, err
	}

	return ports.OutboxMessage{
		Event:    event,
		Attempts: attempts,
	}, nil
}

func validateOutgoingEvent(event ports.OutgoingEvent) error {
	if event.EventID == "" {
		return errors.New("outbox event id is required")
	}

	if _, err := uuid.Parse(event.EventID); err != nil {
		return fmt.Errorf("invalid outbox event id: %w", err)
	}

	if event.EventType == "" {
		return errors.New("outbox event type is required")
	}

	if event.AggregateID == "" {
		return errors.New("outbox aggregate id is required")
	}

	if event.CorrelationID == "" {
		return errors.New("outbox correlation id is required")
	}

	if event.CausationID == "" {
		return errors.New("outbox causation id is required")
	}

	if event.OccurredAt == "" {
		return errors.New("outbox occurred at is required")
	}

	if event.Version < 1 {
		return errors.New("outbox event version must be positive")
	}

	if len(event.Payload) == 0 {
		return errors.New("outbox event payload is required")
	}

	return nil
}

func parseEventID(value string) (uuid.UUID, error) {
	if value == "" {
		return uuid.Nil, errors.New("outbox event id is required")
	}

	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		if err == nil {
			err = errors.New("uuid cannot be nil")
		}

		return uuid.Nil, fmt.Errorf("invalid outbox event id: %w", err)
	}

	return id, nil
}

func parseEventTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, errors.New("outbox timestamp is required")
	}

	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"invalid outbox timestamp: %w",
			err,
		)
	}

	return parsed.UTC(), nil
}

func mapOutboxRepositoryError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return errors.New("duplicate outbox event")

		case "23522":
			return errors.New("invalid outbox event")

		case "23514":
			return errors.New("invalid outbox event")
		}
	}

	return err
}
