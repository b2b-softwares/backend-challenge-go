package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/junglegaming/backend-challenge-go/internal/adapters/postgres"
	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

func TestOutboxRepository_CreateAndClaimBatch(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewOutboxRepository(pool)

	eventID := uuid.New()
	event := newTestOutboxEvent(eventID)

	t.Cleanup(func() {
		cleanupOutboxEvent(t, ctx, pool, eventID)
	})

	if err := repository.Create(ctx, event); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	messages, err := repository.ClaimBatch(ctx, 1, "worker-1")
	if err != nil {
		t.Fatalf("ClaimBatch(): %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("ClaimBatch() returned %d messages, want 1", len(messages))
	}

	message := messages[0]

	if message.Event.EventID != event.EventID {
		t.Fatalf("EventID = %s, want %s", message.Event.EventID, event.EventID)
	}

	if message.Event.EventType != event.EventType {
		t.Fatalf("EventType = %s, want %s", message.Event.EventType, event.EventType)
	}

	if message.Event.AggregateID != event.AggregateID {
		t.Fatalf("AggregateID = %s, want %s", message.Event.AggregateID, event.AggregateID)
	}

	var actualPayload any
	var expectedPayload any

	if err := json.Unmarshal(message.Event.Payload, &actualPayload); err != nil {
		t.Fatalf("unmarshal actual payload: %v", err)
	}

	if err := json.Unmarshal(event.Payload, &expectedPayload); err != nil {
		t.Fatalf("unmarshal expected payload: %v", err)
	}

	if !reflect.DeepEqual(actualPayload, expectedPayload) {
		t.Fatalf(
			"Payload = %s, want %s",
			string(message.Event.Payload),
			string(event.Payload),
		)
	}

	if message.Attempts != 1 {
		t.Fatalf("Attempts = %d, want 1", message.Attempts)
	}

	var attempts int
	var claimedBy string

	err = pool.QueryRow(
		ctx,
		`
		SELECT attempts, claimed_by
		FROM outbox_events
		WHERE event_id = $1
		`,
		eventID,
	).Scan(&attempts, &claimedBy)
	if err != nil {
		t.Fatalf("query claimed event: %v", err)
	}

	if attempts != 1 {
		t.Fatalf("persisted attempts = %d, want 1", attempts)
	}

	if claimedBy != "worker-1" {
		t.Fatalf("claimed_by = %s, want worker-1", claimedBy)
	}
}

func TestOutboxRepository_Create_DuplicateEvent(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewOutboxRepository(pool)

	eventID := uuid.New()
	event := newTestOutboxEvent(eventID)

	t.Cleanup(func() {
		cleanupOutboxEvent(t, ctx, pool, eventID)
	})

	if err := repository.Create(ctx, event); err != nil {
		t.Fatalf("first Create(): %v", err)
	}

	if err := repository.Create(ctx, event); err == nil {
		t.Fatal("second Create() error = nil, want duplicate error")
	}
}

func TestOutboxRepository_CreateWithinTransactionRollback(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewOutboxRepository(pool)
	manager := postgres.NewTransactionManager(pool)

	eventID := uuid.New()
	event := newTestOutboxEvent(eventID)

	err := manager.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			if err := repository.Create(txCtx, event); err != nil {
				return err
			}

			return errors.New("forced outbox rollback")
		},
	)
	if err == nil {
		t.Fatal("WithinTransaction() error = nil, want rollback error")
	}

	var count int

	err = pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM outbox_events
		WHERE event_id = $1
		`,
		eventID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query outbox event: %v", err)
	}

	if count != 0 {
		t.Fatalf("outbox event count after rollback = %d, want 0", count)
	}
}

func TestOutboxRepository_CreateWithinTransactionCommit(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewOutboxRepository(pool)
	manager := postgres.NewTransactionManager(pool)

	eventID := uuid.New()
	event := newTestOutboxEvent(eventID)

	t.Cleanup(func() {
		cleanupOutboxEvent(t, ctx, pool, eventID)
	})

	err := manager.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			return repository.Create(txCtx, event)
		},
	)
	if err != nil {
		t.Fatalf("WithinTransaction(): %v", err)
	}

	var count int

	err = pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM outbox_events
		WHERE event_id = $1
		`,
		eventID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query outbox event: %v", err)
	}

	if count != 1 {
		t.Fatalf("outbox event count after commit = %d, want 1", count)
	}
}

func TestOutboxRepository_ConcurrentClaimOnlyOneWorker(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewOutboxRepository(pool)

	eventID := uuid.New()
	event := newTestOutboxEvent(eventID)

	t.Cleanup(func() {
		cleanupOutboxEvent(t, ctx, pool, eventID)
	})

	if err := repository.Create(ctx, event); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	type claimResult struct {
		worker   string
		messages []ports.OutboxMessage
		err      error
	}

	results := make(chan claimResult, 2)

	var wg sync.WaitGroup
	wg.Add(2)

	for _, workerID := range []string{"worker-a", "worker-b"} {
		workerID := workerID

		go func() {
			defer wg.Done()

			messages, err := repository.ClaimBatch(
				ctx,
				1,
				workerID,
			)

			results <- claimResult{
				worker:   workerID,
				messages: messages,
				err:      err,
			}
		}()
	}

	wg.Wait()
	close(results)

	var claimedCount int
	var claimingWorker string

	for result := range results {
		if result.err != nil {
			t.Fatalf(
				"worker %s ClaimBatch(): %v",
				result.worker,
				result.err,
			)
		}

		if len(result.messages) > 0 {
			claimedCount += len(result.messages)
			claimingWorker = result.worker
		}
	}

	if claimedCount != 1 {
		t.Fatalf(
			"concurrent ClaimBatch() claimed %d events, want 1",
			claimedCount,
		)
	}

	if claimingWorker != "worker-a" && claimingWorker != "worker-b" {
		t.Fatalf("unexpected claiming worker: %s", claimingWorker)
	}
}

func TestOutboxRepository_MarkPublished(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewOutboxRepository(pool)

	eventID := uuid.New()
	event := newTestOutboxEvent(eventID)

	t.Cleanup(func() {
		cleanupOutboxEvent(t, ctx, pool, eventID)
	})

	if err := repository.Create(ctx, event); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	messages, err := repository.ClaimBatch(
		ctx,
		1,
		"worker-publisher",
	)
	if err != nil {
		t.Fatalf("ClaimBatch(): %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf(
			"ClaimBatch() returned %d messages, want 1",
			len(messages),
		)
	}

	if err := repository.MarkPublished(
		ctx,
		event.EventID,
		"worker-publisher",
	); err != nil {
		t.Fatalf("MarkPublished(): %v", err)
	}

	var publishedAt *time.Time
	var claimedBy *string

	err = pool.QueryRow(
		ctx,
		`
		SELECT published_at, claimed_by
		FROM outbox_events
		WHERE event_id = $1
		`,
		eventID,
	).Scan(&publishedAt, &claimedBy)
	if err != nil {
		t.Fatalf("query published event: %v", err)
	}

	if publishedAt == nil {
		t.Fatal("published_at = nil, want timestamp")
	}

	if claimedBy != nil {
		t.Fatalf("claimed_by = %v, want nil", *claimedBy)
	}

	messages, err = repository.ClaimBatch(
		ctx,
		1,
		"worker-second",
	)
	if err != nil {
		t.Fatalf("second ClaimBatch(): %v", err)
	}

	if len(messages) != 0 {
		t.Fatalf(
			"second ClaimBatch() returned %d messages, want 0",
			len(messages),
		)
	}
}

func TestOutboxRepository_MarkPublishedRequiresClaimOwner(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewOutboxRepository(pool)

	eventID := uuid.New()
	event := newTestOutboxEvent(eventID)

	t.Cleanup(func() {
		cleanupOutboxEvent(t, ctx, pool, eventID)
	})

	if err := repository.Create(ctx, event); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	messages, err := repository.ClaimBatch(
		ctx,
		1,
		"worker-owner",
	)
	if err != nil {
		t.Fatalf("ClaimBatch(): %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf(
			"ClaimBatch() returned %d messages, want 1",
			len(messages),
		)
	}

	err = repository.MarkPublished(
		ctx,
		event.EventID,
		"worker-other",
	)

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"MarkPublished() error = %v, want pgx.ErrNoRows",
			err,
		)
	}

	var publishedAt *time.Time

	err = pool.QueryRow(
		ctx,
		`
		SELECT published_at
		FROM outbox_events
		WHERE event_id = $1
		`,
		eventID,
	).Scan(&publishedAt)
	if err != nil {
		t.Fatalf("query published_at: %v", err)
	}

	if publishedAt != nil {
		t.Fatal("event was published by a non-owner worker")
	}
}

func TestOutboxRepository_MarkFailedMakesEventAvailableAgain(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewOutboxRepository(pool)

	eventID := uuid.New()
	event := newTestOutboxEvent(eventID)

	t.Cleanup(func() {
		cleanupOutboxEvent(t, ctx, pool, eventID)
	})

	if err := repository.Create(ctx, event); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	messages, err := repository.ClaimBatch(
		ctx,
		1,
		"worker-1",
	)
	if err != nil {
		t.Fatalf("first ClaimBatch(): %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf(
			"first ClaimBatch() returned %d messages, want 1",
			len(messages),
		)
	}

	nextAttemptAt := time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)

	if err := repository.MarkFailed(
		ctx,
		event.EventID,
		"worker-1",
		nextAttemptAt,
		"temporary publisher failure",
	); err != nil {
		t.Fatalf("MarkFailed(): %v", err)
	}

	messages, err = repository.ClaimBatch(
		ctx,
		1,
		"worker-2",
	)
	if err != nil {
		t.Fatalf("second ClaimBatch(): %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf(
			"second ClaimBatch() returned %d messages, want 1",
			len(messages),
		)
	}

	if messages[0].Attempts != 2 {
		t.Fatalf(
			"Attempts after retry = %d, want 2",
			messages[0].Attempts,
		)
	}

	if messages[0].Event.EventID != event.EventID {
		t.Fatalf(
			"retried EventID = %s, want %s",
			messages[0].Event.EventID,
			event.EventID,
		)
	}
}

func TestOutboxRepository_MarkFailedRequiresClaimOwner(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewOutboxRepository(pool)

	eventID := uuid.New()
	event := newTestOutboxEvent(eventID)

	t.Cleanup(func() {
		cleanupOutboxEvent(t, ctx, pool, eventID)
	})

	if err := repository.Create(ctx, event); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	messages, err := repository.ClaimBatch(
		ctx,
		1,
		"worker-owner",
	)
	if err != nil {
		t.Fatalf("ClaimBatch(): %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf(
			"ClaimBatch() returned %d messages, want 1",
			len(messages),
		)
	}

	nextAttemptAt := time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)

	err = repository.MarkFailed(
		ctx,
		event.EventID,
		"worker-other",
		nextAttemptAt,
		"wrong worker",
	)

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"MarkFailed() error = %v, want pgx.ErrNoRows",
			err,
		)
	}

	var claimedBy string
	var lastError *string

	err = pool.QueryRow(
		ctx,
		`
		SELECT claimed_by, last_error
		FROM outbox_events
		WHERE event_id = $1
		`,
		eventID,
	).Scan(&claimedBy, &lastError)
	if err != nil {
		t.Fatalf("query failed event: %v", err)
	}

	if claimedBy != "worker-owner" {
		t.Fatalf(
			"claimed_by = %s, want worker-owner",
			claimedBy,
		)
	}

	if lastError != nil {
		t.Fatalf(
			"last_error = %v, want nil",
			*lastError,
		)
	}
}

func TestOutboxRepository_ClaimBatchDoesNotReturnUnavailableEvent(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewOutboxRepository(pool)

	eventID := uuid.New()
	event := newTestOutboxEvent(eventID)

	t.Cleanup(func() {
		cleanupOutboxEvent(t, ctx, pool, eventID)
	})

	if err := repository.Create(ctx, event); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	future := time.Now().UTC().Add(10 * time.Minute)

	_, err := pool.Exec(
		ctx,
		`
		UPDATE outbox_events
		SET available_at = $2
		WHERE event_id = $1
		`,
		eventID,
		future,
	)
	if err != nil {
		t.Fatalf("update available_at: %v", err)
	}

	messages, err := repository.ClaimBatch(
		ctx,
		1,
		"worker-1",
	)
	if err != nil {
		t.Fatalf("ClaimBatch(): %v", err)
	}

	if len(messages) != 0 {
		t.Fatalf(
			"ClaimBatch() returned %d messages, want 0",
			len(messages),
		)
	}
}

func newTestOutboxEvent(eventID uuid.UUID) ports.OutgoingEvent {
	return ports.OutgoingEvent{
		EventID:       eventID.String(),
		EventType:     "wallet.balance.changed",
		AggregateID:   uuid.New().String(),
		CorrelationID: uuid.New().String(),
		CausationID:   uuid.New().String(),
		OccurredAt:    time.Now().UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano),
		Version:       1,
		Payload: []byte(`{
			"walletId": "test-wallet",
			"balanceMinor": 7500,
			"currency": "BRL"
		}`),
	}
}

func cleanupOutboxEvent(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	eventID uuid.UUID,
) {
	t.Helper()

	_, _ = pool.Exec(
		ctx,
		`
		DELETE FROM outbox_events
		WHERE event_id = $1
		`,
		eventID,
	)
}
