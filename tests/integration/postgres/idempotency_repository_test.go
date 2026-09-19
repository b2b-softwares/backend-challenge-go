package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/junglegaming/backend-challenge-go/internal/adapters/postgres"
	"github.com/junglegaming/backend-challenge-go/internal/domain"
	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

func cleanupIdempotencyRecord(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	providerID string,
	idempotencyKey string,
) {
	t.Helper()

	_, err := pool.Exec(
		ctx,
		`
		DELETE FROM idempotency_records
		WHERE provider_id = $1
		  AND idempotency_key = $2
		`,
		providerID,
		idempotencyKey,
	)
	if err != nil {
		t.Fatalf("cleanup idempotency record: %v", err)
	}
}

func TestIdempotencyRepository_CreateAndFind(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewIdempotencyRepository(pool)
	wagerTransactionRepository := postgres.NewWagerTransactionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	fixture := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		"provider-"+uuid.New().String(),
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindBet,
		mustMoney(t, "25.00", domain.CurrencyBRL),
		"",
		now,
	)

	if err := wagerTransactionRepository.Create(
		ctx,
		fixture.transaction,
	); err != nil {
		t.Fatalf("create wager transaction: %v", err)
	}

	t.Cleanup(func() {
		cleanupIdempotencyRecord(
			t,
			ctx,
			pool,
			fixture.transaction.ProviderID(),
			fixture.transaction.IdempotencyKey(),
		)

		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			fixture.transaction.ID(),
		)
	})

	record := ports.IdempotencyRecord{
		ProviderID:              fixture.transaction.ProviderID(),
		IdempotencyKey:          fixture.transaction.IdempotencyKey(),
		PayloadHash:             fixture.transaction.PayloadHash(),
		TransactionID:           fixture.transaction.ID(),
		Status:                  string(domain.TransactionStatusProcessed),
		ResponseBody:            []byte(`{"status":"processed","balance":"75.00"}`),
		ObservedBalanceAmount:   7500,
		ObservedBalanceCurrency: string(domain.CurrencyBRL),
	}

	if err := repository.Create(ctx, record); err != nil {
		t.Fatalf("create idempotency record: %v", err)
	}

	got, err := repository.Find(
		ctx,
		record.ProviderID,
		record.IdempotencyKey,
	)
	if err != nil {
		t.Fatalf("find idempotency record: %v", err)
	}

	if got.ProviderID != record.ProviderID {
		t.Fatalf(
			"provider id mismatch: expected %s, got %s",
			record.ProviderID,
			got.ProviderID,
		)
	}

	if got.IdempotencyKey != record.IdempotencyKey {
		t.Fatalf(
			"idempotency key mismatch: expected %s, got %s",
			record.IdempotencyKey,
			got.IdempotencyKey,
		)
	}

	if got.PayloadHash != record.PayloadHash {
		t.Fatalf(
			"payload hash mismatch: expected %s, got %s",
			record.PayloadHash,
			got.PayloadHash,
		)
	}

	if got.TransactionID != record.TransactionID {
		t.Fatalf(
			"transaction id mismatch: expected %s, got %s",
			record.TransactionID,
			got.TransactionID,
		)
	}

	if got.Status != record.Status {
		t.Fatalf(
			"status mismatch: expected %s, got %s",
			record.Status,
			got.Status,
		)
	}

	var expectedResponse any
	var actualResponse any

	if err := json.Unmarshal(record.ResponseBody, &expectedResponse); err != nil {
		t.Fatalf(
			"invalid expected response body: %v",
			err,
		)
	}

	if err := json.Unmarshal(got.ResponseBody, &actualResponse); err != nil {
		t.Fatalf(
			"invalid stored response body: %v",
			err,
		)
	}

	expectedJSON, err := json.Marshal(expectedResponse)
	if err != nil {
		t.Fatalf(
			"marshal expected response body: %v",
			err,
		)
	}

	actualJSON, err := json.Marshal(actualResponse)
	if err != nil {
		t.Fatalf(
			"marshal actual response body: %v",
			err,
		)
	}

	if string(actualJSON) != string(expectedJSON) {
		t.Fatalf(
			"response body mismatch: expected %s, got %s",
			expectedJSON,
			actualJSON,
		)
	}

	if got.ObservedBalanceAmount != record.ObservedBalanceAmount {
		t.Fatalf(
			"observed balance amount mismatch: expected %d, got %d",
			record.ObservedBalanceAmount,
			got.ObservedBalanceAmount,
		)
	}

	if got.ObservedBalanceCurrency != record.ObservedBalanceCurrency {
		t.Fatalf(
			"observed balance currency mismatch: expected %s, got %s",
			record.ObservedBalanceCurrency,
			got.ObservedBalanceCurrency,
		)
	}
}

func TestIdempotencyRepository_Find_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewIdempotencyRepository(pool)

	_, err := repository.Find(
		ctx,
		"provider-does-not-exist",
		"idempotency-does-not-exist",
	)

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected pgx.ErrNoRows, got %v",
			err,
		)
	}
}

func TestIdempotencyRepository_DuplicateProviderAndKey(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewIdempotencyRepository(pool)
	wagerTransactionRepository := postgres.NewWagerTransactionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	fixture := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		"provider-"+uuid.New().String(),
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindBet,
		mustMoney(t, "25.00", domain.CurrencyBRL),
		"",
		now,
	)

	if err := wagerTransactionRepository.Create(
		ctx,
		fixture.transaction,
	); err != nil {
		t.Fatalf("create wager transaction: %v", err)
	}

	t.Cleanup(func() {
		cleanupIdempotencyRecord(
			t,
			ctx,
			pool,
			fixture.transaction.ProviderID(),
			fixture.transaction.IdempotencyKey(),
		)

		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			fixture.transaction.ID(),
		)
	})

	record := ports.IdempotencyRecord{
		ProviderID:              fixture.transaction.ProviderID(),
		IdempotencyKey:          fixture.transaction.IdempotencyKey(),
		PayloadHash:             fixture.transaction.PayloadHash(),
		TransactionID:           fixture.transaction.ID(),
		Status:                  string(domain.TransactionStatusProcessed),
		ResponseBody:            []byte(`{"status":"processed"}`),
		ObservedBalanceAmount:   7500,
		ObservedBalanceCurrency: string(domain.CurrencyBRL),
	}

	if err := repository.Create(ctx, record); err != nil {
		t.Fatalf(
			"create first idempotency record: %v",
			err,
		)
	}

	err := repository.Create(ctx, record)
	if err == nil {
		t.Fatal("expected duplicate idempotency error")
	}
}

func TestIdempotencyRepository_RollbackWithinTransaction(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewIdempotencyRepository(pool)
	transactionManager := postgres.NewTransactionManager(pool)
	wagerTransactionRepository := postgres.NewWagerTransactionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	fixture := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		"provider-"+uuid.New().String(),
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindBet,
		mustMoney(t, "25.00", domain.CurrencyBRL),
		"",
		now,
	)

	if err := wagerTransactionRepository.Create(
		ctx,
		fixture.transaction,
	); err != nil {
		t.Fatalf("create wager transaction: %v", err)
	}

	t.Cleanup(func() {
		cleanupIdempotencyRecord(
			t,
			ctx,
			pool,
			fixture.transaction.ProviderID(),
			fixture.transaction.IdempotencyKey(),
		)

		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			fixture.transaction.ID(),
		)
	})

	record := ports.IdempotencyRecord{
		ProviderID:              fixture.transaction.ProviderID(),
		IdempotencyKey:          fixture.transaction.IdempotencyKey(),
		PayloadHash:             fixture.transaction.PayloadHash(),
		TransactionID:           fixture.transaction.ID(),
		Status:                  string(domain.TransactionStatusPending),
		ResponseBody:            []byte(`{"status":"pending"}`),
		ObservedBalanceAmount:   7500,
		ObservedBalanceCurrency: string(domain.CurrencyBRL),
	}

	err := transactionManager.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			if err := repository.Create(txCtx, record); err != nil {
				return err
			}

			return errors.New("force rollback")
		},
	)

	if err == nil {
		t.Fatal("expected rollback error")
	}

	_, err = repository.Find(
		ctx,
		record.ProviderID,
		record.IdempotencyKey,
	)

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected idempotency record to be absent after rollback, got %v",
			err,
		)
	}
}

func TestIdempotencyRepository_TransactionIsolation(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewIdempotencyRepository(pool)
	transactionManager := postgres.NewTransactionManager(pool)
	wagerTransactionRepository := postgres.NewWagerTransactionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	fixture := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		"provider-"+uuid.New().String(),
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindBet,
		mustMoney(t, "25.00", domain.CurrencyBRL),
		"",
		now,
	)

	if err := wagerTransactionRepository.Create(
		ctx,
		fixture.transaction,
	); err != nil {
		t.Fatalf("create wager transaction: %v", err)
	}

	t.Cleanup(func() {
		cleanupIdempotencyRecord(
			t,
			ctx,
			pool,
			fixture.transaction.ProviderID(),
			fixture.transaction.IdempotencyKey(),
		)

		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			fixture.transaction.ID(),
		)
	})

	record := ports.IdempotencyRecord{
		ProviderID:              fixture.transaction.ProviderID(),
		IdempotencyKey:          fixture.transaction.IdempotencyKey(),
		PayloadHash:             fixture.transaction.PayloadHash(),
		TransactionID:           fixture.transaction.ID(),
		Status:                  string(domain.TransactionStatusProcessed),
		ResponseBody:            []byte(`{"status":"processed"}`),
		ObservedBalanceAmount:   7500,
		ObservedBalanceCurrency: string(domain.CurrencyBRL),
	}

	err := transactionManager.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			if err := repository.Create(txCtx, record); err != nil {
				return err
			}

			_, err := repository.Find(
				txCtx,
				record.ProviderID,
				record.IdempotencyKey,
			)
			if err != nil {
				return err
			}

			return nil
		},
	)

	if err != nil {
		t.Fatalf(
			"transaction with create and find failed: %v",
			err,
		)
	}

	got, err := repository.Find(
		ctx,
		record.ProviderID,
		record.IdempotencyKey,
	)
	if err != nil {
		t.Fatalf(
			"find after committed transaction: %v",
			err,
		)
	}

	if got.TransactionID != record.TransactionID {
		t.Fatalf(
			"transaction id mismatch: expected %s, got %s",
			record.TransactionID,
			got.TransactionID,
		)
	}
}
