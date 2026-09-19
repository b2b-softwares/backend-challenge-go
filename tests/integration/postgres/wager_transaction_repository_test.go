package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/junglegaming/backend-challenge-go/internal/adapters/postgres"
	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

type persistedWagerTransaction struct {
	transaction domain.WagerTransaction
	walletID    uuid.UUID
}

func newPersistedWagerTransaction(
	t *testing.T,
	pool *pgxpool.Pool,
	externalTransactionID string,
	providerID string,
	idempotencyKey string,
	kind domain.TransactionKind,
	money domain.Money,
	referenceExternalID string,
	now time.Time,
) persistedWagerTransaction {
	t.Helper()

	walletID := uuid.New()
	playerID := uuid.New()

	_, err := pool.Exec(
		context.Background(),
		`
		INSERT INTO wallets (
			id,
			player_id,
			currency,
			balance_minor,
			version,
			created_at,
			updated_at
		)
		VALUES (
			$1,
			$2,
			$3,
			0,
			1,
			$4,
			$4
		)
		`,
		walletID,
		playerID,
		string(money.Currency()),
		now,
	)
	if err != nil {
		t.Fatalf("create wallet fixture: %v", err)
	}

	transaction, err := domain.NewExternalTransaction(
		uuid.New(),
		externalTransactionID,
		providerID,
		idempotencyKey,
		"payload-hash-"+uuid.New().String(),
		walletID,
		playerID,
		"round-"+uuid.New().String(),
		"game-"+uuid.New().String(),
		kind,
		money,
		referenceExternalID,
		now,
	)
	if err != nil {
		_, _ = pool.Exec(
			context.Background(),
			`
			DELETE FROM wallets
			WHERE id = $1
			`,
			walletID,
		)

		t.Fatalf("create wager transaction fixture: %v", err)
	}

	return persistedWagerTransaction{
		transaction: transaction,
		walletID:    walletID,
	}
}

func TestWagerTransactionRepository_CreateAndFindByID(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWagerTransactionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	transaction := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		"provider-"+uuid.New().String(),
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindBet,
		mustMoney(t, "10.00", domain.CurrencyBRL),
		"",
		now,
	)

	t.Cleanup(func() {
		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			transaction.transaction.ID(),
			transaction.walletID,
		)
	})

	if err := repository.Create(ctx, transaction.transaction); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	got, err := repository.GetByID(
		ctx,
		transaction.transaction.ID(),
	)
	if err != nil {
		t.Fatalf("GetByID(): %v", err)
	}

	if got.ID() != transaction.transaction.ID() {
		t.Fatalf(
			"ID = %s, want %s",
			got.ID(),
			transaction.transaction.ID(),
		)
	}

	if got.ProviderID() != transaction.transaction.ProviderID() {
		t.Fatalf(
			"ProviderID = %s, want %s",
			got.ProviderID(),
			transaction.transaction.ProviderID(),
		)
	}

	if got.ExternalTransactionID() != transaction.transaction.ExternalTransactionID() {
		t.Fatalf(
			"ExternalTransactionID = %s, want %s",
			got.ExternalTransactionID(),
			transaction.transaction.ExternalTransactionID(),
		)
	}

	if got.IdempotencyKey() != transaction.transaction.IdempotencyKey() {
		t.Fatalf(
			"IdempotencyKey = %s, want %s",
			got.IdempotencyKey(),
			transaction.transaction.IdempotencyKey(),
		)
	}

	if got.Kind() != transaction.transaction.Kind() {
		t.Fatalf(
			"Kind = %s, want %s",
			got.Kind(),
			transaction.transaction.Kind(),
		)
	}

	if !got.Money().Equal(transaction.transaction.Money()) {
		t.Fatalf(
			"Money = %s, want %s",
			got.Money(),
			transaction.transaction.Money(),
		)
	}

	if got.ReferenceExternalID() != transaction.transaction.ReferenceExternalID() {
		t.Fatalf(
			"ReferenceExternalID = %s, want %s",
			got.ReferenceExternalID(),
			transaction.transaction.ReferenceExternalID(),
		)
	}

	if !got.CreatedAt().Equal(transaction.transaction.CreatedAt()) {
		t.Fatalf(
			"CreatedAt = %s, want %s",
			got.CreatedAt(),
			transaction.transaction.CreatedAt(),
		)
	}
}

func TestWagerTransactionRepository_FindByIDNotFound(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWagerTransactionRepository(pool)

	_, err := repository.GetByID(ctx, uuid.New())

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"GetByID() error = %v, want pgx.ErrNoRows",
			err,
		)
	}
}

func TestWagerTransactionRepository_CreateDuplicateExternalTransaction(
	t *testing.T,
) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWagerTransactionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	providerID := "provider-" + uuid.New().String()
	externalTransactionID := "external-" + uuid.New().String()

	first := newPersistedWagerTransaction(
		t,
		pool,
		externalTransactionID,
		providerID,
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindBet,
		mustMoney(t, "10.00", domain.CurrencyBRL),
		"",
		now,
	)

	second := newPersistedWagerTransaction(
		t,
		pool,
		externalTransactionID,
		providerID,
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindBet,
		mustMoney(t, "20.00", domain.CurrencyBRL),
		"",
		now,
	)

	t.Cleanup(func() {
		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			first.transaction.ID(),
			first.walletID,
		)
		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			second.transaction.ID(),
			second.walletID,
		)
	})

	if err := repository.Create(ctx, first.transaction); err != nil {
		t.Fatalf("first Create(): %v", err)
	}

	err := repository.Create(ctx, second.transaction)

	if !errors.Is(err, domain.ErrDuplicateTransaction) {
		t.Fatalf(
			"expected ErrDuplicateTransaction, got %v",
			err,
		)
	}
}

func TestWagerTransactionRepository_DuplicateProviderIdempotency(
	t *testing.T,
) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWagerTransactionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	providerID := "provider-" + uuid.New().String()
	idempotencyKey := "idempotency-" + uuid.New().String()

	first := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		providerID,
		idempotencyKey,
		domain.TransactionKindBet,
		mustMoney(t, "10.00", domain.CurrencyBRL),
		"",
		now,
	)

	second := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		providerID,
		idempotencyKey,
		domain.TransactionKindBet,
		mustMoney(t, "20.00", domain.CurrencyBRL),
		"",
		now,
	)

	t.Cleanup(func() {
		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			first.transaction.ID(),
			first.walletID,
		)
		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			second.transaction.ID(),
			second.walletID,
		)
	})

	if err := repository.Create(ctx, first.transaction); err != nil {
		t.Fatalf("first Create(): %v", err)
	}

	err := repository.Create(ctx, second.transaction)

	if !errors.Is(err, domain.ErrDuplicateWagerIdempotency) {
		t.Fatalf(
			"expected ErrDuplicateWagerIdempotency, got %v",
			err,
		)
	}
}

func TestWagerTransactionRepository_CreateWithinTransactionRollback(
	t *testing.T,
) {
	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewWagerTransactionRepository(pool)
	manager := postgres.NewTransactionManager(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	transaction := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		"provider-"+uuid.New().String(),
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindBet,
		mustMoney(t, "10.00", domain.CurrencyBRL),
		"",
		now,
	)

	t.Cleanup(func() {
		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			transaction.transaction.ID(),
			transaction.walletID,
		)
	})

	err := manager.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			if err := repository.Create(txCtx, transaction.transaction); err != nil {
				return err
			}

			return errors.New("forced wager transaction rollback")
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
		FROM wager_transactions
		WHERE id = $1
		`,
		transaction.transaction.ID(),
	).Scan(&count)
	if err != nil {
		t.Fatalf("query wager transaction: %v", err)
	}

	if count != 0 {
		t.Fatalf(
			"wager transaction count after rollback = %d, want 0",
			count,
		)
	}
}

func TestWagerTransactionRepository_CreateWithinTransactionCommit(
	t *testing.T,
) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWagerTransactionRepository(pool)
	manager := postgres.NewTransactionManager(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	transaction := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		"provider-"+uuid.New().String(),
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindBet,
		mustMoney(t, "10.00", domain.CurrencyBRL),
		"",
		now,
	)

	t.Cleanup(func() {
		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			transaction.transaction.ID(),
			transaction.walletID,
		)
	})

	err := manager.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			return repository.Create(
				txCtx,
				transaction.transaction,
			)
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
		FROM wager_transactions
		WHERE id = $1
		`,
		transaction.transaction.ID(),
	).Scan(&count)
	if err != nil {
		t.Fatalf("query wager transaction: %v", err)
	}

	if count != 1 {
		t.Fatalf(
			"wager transaction count after commit = %d, want 1",
			count,
		)
	}
}

func TestWagerTransactionRepository_CreateRejectsInvalidTransaction(
	t *testing.T,
) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWagerTransactionRepository(pool)

	err := repository.Create(
		ctx,
		domain.WagerTransaction{},
	)

	if err == nil {
		t.Fatal("Create(invalid transaction) error = nil, want error")
	}
}

func TestWagerTransactionRepository_GetByProviderAndReference(
	t *testing.T,
) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWagerTransactionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	providerID := "provider-" + uuid.New().String()
	referenceExternalID := "reference-" + uuid.New().String()

	transaction := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		providerID,
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindBet,
		mustMoney(t, "25.00", domain.CurrencyBRL),
		referenceExternalID,
		now,
	)

	t.Cleanup(func() {
		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			transaction.transaction.ID(),
			transaction.walletID,
		)
	})

	if err := repository.Create(ctx, transaction.transaction); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	_, err := pool.Exec(
		ctx,
		`
		UPDATE wager_transactions
		SET status = 'PROCESSED'
		WHERE id = $1
		`,
		transaction.transaction.ID(),
	)
	if err != nil {
		t.Fatalf("mark transaction as processed: %v", err)
	}

	got, err := repository.GetByProviderAndReference(
		ctx,
		providerID,
		referenceExternalID,
	)
	if err != nil {
		t.Fatalf(
			"GetByProviderAndReference(): %v",
			err,
		)
	}

	if got.ID() != transaction.transaction.ID() {
		t.Fatalf(
			"ID = %s, want %s",
			got.ID(),
			transaction.transaction.ID(),
		)
	}

	if got.ProviderID() != providerID {
		t.Fatalf(
			"ProviderID = %s, want %s",
			got.ProviderID(),
			providerID,
		)
	}

	if got.ReferenceExternalID() != referenceExternalID {
		t.Fatalf(
			"ReferenceExternalID = %s, want %s",
			got.ReferenceExternalID(),
			referenceExternalID,
		)
	}

	if got.Status() != domain.TransactionStatusProcessed {
		t.Fatalf(
			"Status = %s, want %s",
			got.Status(),
			domain.TransactionStatusProcessed,
		)
	}
}

func TestWagerTransactionRepository_GetByProviderAndReferenceNotFound(
	t *testing.T,
) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWagerTransactionRepository(pool)

	_, err := repository.GetByProviderAndReference(
		ctx,
		"provider-"+uuid.New().String(),
		"reference-"+uuid.New().String(),
	)

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"GetByProviderAndReference() error = %v, want pgx.ErrNoRows",
			err,
		)
	}
}

func TestWagerTransactionRepository_GetPendingReferenceBatch(
	t *testing.T,
) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWagerTransactionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	transaction := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		"provider-"+uuid.New().String(),
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindRefund,
		mustMoney(t, "15.00", domain.CurrencyBRL),
		"reference-"+uuid.New().String(),
		now,
	)

	t.Cleanup(func() {
		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			transaction.transaction.ID(),
			transaction.walletID,
		)
	})

	if err := repository.Create(ctx, transaction.transaction); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	availableAt := now.Add(-1 * time.Minute)
	expiresAt := now.Add(30 * time.Minute)

	_, err := pool.Exec(
		ctx,
		`
		UPDATE wager_transactions
		SET
			status = 'PENDING_REFERENCE',
			reference_attempts = 2,
			reference_available_at = $2,
			reference_expires_at = $3
		WHERE id = $1
		`,
		transaction.transaction.ID(),
		availableAt,
		expiresAt,
	)
	if err != nil {
		t.Fatalf(
			"mark transaction as pending reference: %v",
			err,
		)
	}

	got, err := repository.GetPendingReferenceBatch(
		ctx,
		10,
		now,
	)
	if err != nil {
		t.Fatalf(
			"GetPendingReferenceBatch(): %v",
			err,
		)
	}

	var found bool

	for _, item := range got {
		if item.Transaction.ID() != transaction.transaction.ID() {
			continue
		}

		found = true

		if item.Attempts != 2 {
			t.Fatalf(
				"Attempts = %d, want 2",
				item.Attempts,
			)
		}

		if item.AvailableAt == nil {
			t.Fatal("AvailableAt = nil, want timestamp")
		}

		if !item.AvailableAt.Equal(availableAt) {
			t.Fatalf(
				"AvailableAt = %s, want %s",
				item.AvailableAt,
				availableAt,
			)
		}

		if item.ExpiresAt == nil {
			t.Fatal("ExpiresAt = nil, want timestamp")
		}

		if !item.ExpiresAt.Equal(expiresAt) {
			t.Fatalf(
				"ExpiresAt = %s, want %s",
				item.ExpiresAt,
				expiresAt,
			)
		}

		if item.Transaction.Status() != domain.TransactionStatusPendingReference {
			t.Fatalf(
				"Status = %s, want %s",
				item.Transaction.Status(),
				domain.TransactionStatusPendingReference,
			)
		}
	}

	if !found {
		t.Fatalf(
			"transaction %s not found in pending reference batch",
			transaction.transaction.ID(),
		)
	}
}

func TestWagerTransactionRepository_GetPendingReferenceBatchSkipsUnavailable(
	t *testing.T,
) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWagerTransactionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	transaction := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		"provider-"+uuid.New().String(),
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindRefund,
		mustMoney(t, "15.00", domain.CurrencyBRL),
		"reference-"+uuid.New().String(),
		now,
	)

	t.Cleanup(func() {
		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			transaction.transaction.ID(),
			transaction.walletID,
		)
	})

	if err := repository.Create(ctx, transaction.transaction); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	unavailableAt := now.Add(10 * time.Minute)

	_, err := pool.Exec(
		ctx,
		`
		UPDATE wager_transactions
		SET
			status = 'PENDING_REFERENCE',
			reference_attempts = 1,
			reference_available_at = $2,
			reference_expires_at = $3
		WHERE id = $1
		`,
		transaction.transaction.ID(),
		unavailableAt,
		now.Add(30*time.Minute),
	)
	if err != nil {
		t.Fatalf(
			"mark transaction as pending reference: %v",
			err,
		)
	}

	got, err := repository.GetPendingReferenceBatch(
		ctx,
		10,
		now,
	)
	if err != nil {
		t.Fatalf(
			"GetPendingReferenceBatch(): %v",
			err,
		)
	}

	for _, item := range got {
		if item.Transaction.ID() == transaction.transaction.ID() {
			t.Fatalf(
				"transaction %s should not be returned before reference_available_at",
				transaction.transaction.ID(),
			)
		}
	}
}

func TestWagerTransactionRepository_UpdateReferenceRetry(
	t *testing.T,
) {
	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWagerTransactionRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)

	transaction := newPersistedWagerTransaction(
		t,
		pool,
		"external-"+uuid.New().String(),
		"provider-"+uuid.New().String(),
		"idempotency-"+uuid.New().String(),
		domain.TransactionKindRollback,
		mustMoney(t, "12.00", domain.CurrencyBRL),
		"reference-"+uuid.New().String(),
		now,
	)

	t.Cleanup(func() {
		cleanupWagerTransaction(
			t,
			ctx,
			pool,
			transaction.transaction.ID(),
			transaction.walletID,
		)
	})

	if err := repository.Create(ctx, transaction.transaction); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	availableAt := now.Add(30 * time.Second)
	expiresAt := now.Add(30 * time.Minute)

	_, err := pool.Exec(
		ctx,
		`
		UPDATE wager_transactions
		SET
			status = 'PENDING_REFERENCE',
			reference_attempts = 1,
			reference_available_at = $2,
			reference_expires_at = $3
		WHERE id = $1
		`,
		transaction.transaction.ID(),
		now,
		expiresAt,
	)
	if err != nil {
		t.Fatalf(
			"mark transaction as pending reference: %v",
			err,
		)
	}

	if err := repository.UpdateReferenceRetry(
		ctx,
		transaction.transaction.ID(),
		2,
		&availableAt,
	); err != nil {
		t.Fatalf(
			"UpdateReferenceRetry(): %v",
			err,
		)
	}

	var (
		attempts        int
		persistedAt     *time.Time
		persistedExpiry *time.Time
		status          string
	)

	err = pool.QueryRow(
		ctx,
		`
		SELECT
			reference_attempts,
			reference_available_at,
			reference_expires_at,
			status
		FROM wager_transactions
		WHERE id = $1
		`,
		transaction.transaction.ID(),
	).Scan(
		&attempts,
		&persistedAt,
		&persistedExpiry,
		&status,
	)
	if err != nil {
		t.Fatalf(
			"query reference retry state: %v",
			err,
		)
	}

	if attempts != 2 {
		t.Fatalf(
			"reference_attempts = %d, want 2",
			attempts,
		)
	}

	if persistedAt == nil {
		t.Fatal("reference_available_at = nil, want timestamp")
	}

	if !persistedAt.Equal(availableAt) {
		t.Fatalf(
			"reference_available_at = %s, want %s",
			persistedAt,
			availableAt,
		)
	}

	if persistedExpiry == nil {
		t.Fatal("reference_expires_at = nil, want timestamp")
	}

	if !persistedExpiry.Equal(expiresAt) {
		t.Fatalf(
			"reference_expires_at = %s, want %s",
			persistedExpiry,
			expiresAt,
		)
	}

	if status != string(domain.TransactionStatusPendingReference) {
		t.Fatalf(
			"status = %s, want %s",
			status,
			domain.TransactionStatusPendingReference,
		)
	}
}

func cleanupWagerTransaction(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	transactionID uuid.UUID,
	walletIDs ...uuid.UUID,
) {
	t.Helper()

	_, _ = pool.Exec(
		ctx,
		`
		DELETE FROM wager_transactions
		WHERE id = $1
		`,
		transactionID,
	)

	for _, walletID := range walletIDs {
		if walletID == uuid.Nil {
			continue
		}

		_, _ = pool.Exec(
			ctx,
			`
			DELETE FROM wallets
			WHERE id = $1
			`,
			walletID,
		)
	}
}
