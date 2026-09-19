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

func TestLedgerRepository_CreateAndGetByTransactionID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)

	walletRepository := postgres.NewWalletRepository(pool)
	repository := postgres.NewLedgerRepository(pool)

	wallet := newTestWallet(t)

	if err := walletRepository.Create(ctx, wallet); err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	transactionID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	insertWagerTransaction(
		t,
		ctx,
		pool,
		transactionID,
		wallet,
		2500,
		7500,
		now,
	)

	t.Cleanup(func() {
		cleanupLedgerData(
			t,
			ctx,
			pool,
			transactionID,
			wallet.ID(),
		)
	})

	amount, err := domain.MoneyFromMinorUnits(
		2500,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("amount: %v", err)
	}

	before, err := domain.MoneyFromMinorUnits(
		10000,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("balance before: %v", err)
	}

	after, err := domain.MoneyFromMinorUnits(
		7500,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("balance after: %v", err)
	}

	entry, err := domain.NewLedgerEntry(
		uuid.New(),
		wallet.ID(),
		transactionID,
		domain.LedgerDirectionDebit,
		amount,
		before,
		after,
		now,
	)
	if err != nil {
		t.Fatalf("NewLedgerEntry(): %v", err)
	}

	if err := repository.Create(ctx, entry); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	found, err := repository.GetByTransactionID(
		ctx,
		transactionID,
	)
	if err != nil {
		t.Fatalf("GetByTransactionID(): %v", err)
	}

	if found.ID() != entry.ID() {
		t.Fatalf(
			"ID = %s, want %s",
			found.ID(),
			entry.ID(),
		)
	}

	if found.TransactionID() != transactionID {
		t.Fatalf(
			"TransactionID = %s, want %s",
			found.TransactionID(),
			transactionID,
		)
	}

	if found.WalletID() != wallet.ID() {
		t.Fatalf(
			"WalletID = %s, want %s",
			found.WalletID(),
			wallet.ID(),
		)
	}

	if found.Direction() != domain.LedgerDirectionDebit {
		t.Fatalf(
			"Direction = %s, want %s",
			found.Direction(),
			domain.LedgerDirectionDebit,
		)
	}

	if found.Money().Amount() != 2500 {
		t.Fatalf(
			"Amount = %d, want 2500",
			found.Money().Amount(),
		)
	}

	if found.BalanceBefore().Amount() != 10000 {
		t.Fatalf(
			"BalanceBefore = %d, want 10000",
			found.BalanceBefore().Amount(),
		)
	}

	if found.BalanceAfter().Amount() != 7500 {
		t.Fatalf(
			"BalanceAfter = %d, want 7500",
			found.BalanceAfter().Amount(),
		)
	}
}

func TestLedgerRepository_GetByTransactionID_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)

	repository := postgres.NewLedgerRepository(pool)

	_, err := repository.GetByTransactionID(ctx, uuid.New())

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"GetByTransactionID() error = %v, want pgx.ErrNoRows",
			err,
		)
	}
}

func TestLedgerRepository_Create_DuplicateWalletTransaction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)

	walletRepository := postgres.NewWalletRepository(pool)
	repository := postgres.NewLedgerRepository(pool)

	wallet := newTestWallet(t)

	if err := walletRepository.Create(ctx, wallet); err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	transactionID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	insertWagerTransaction(
		t,
		ctx,
		pool,
		transactionID,
		wallet,
		1000,
		9000,
		now,
	)

	t.Cleanup(func() {
		cleanupLedgerData(
			t,
			ctx,
			pool,
			transactionID,
			wallet.ID(),
		)
	})

	amount, err := domain.MoneyFromMinorUnits(
		1000,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("amount: %v", err)
	}

	before, err := domain.MoneyFromMinorUnits(
		10000,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("balance before: %v", err)
	}

	after, err := domain.MoneyFromMinorUnits(
		9000,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("balance after: %v", err)
	}

	first, err := domain.NewLedgerEntry(
		uuid.New(),
		wallet.ID(),
		transactionID,
		domain.LedgerDirectionDebit,
		amount,
		before,
		after,
		now,
	)
	if err != nil {
		t.Fatalf("first entry: %v", err)
	}

	second, err := domain.NewLedgerEntry(
		uuid.New(),
		wallet.ID(),
		transactionID,
		domain.LedgerDirectionDebit,
		amount,
		before,
		after,
		now.Add(time.Second),
	)
	if err != nil {
		t.Fatalf("second entry: %v", err)
	}

	if err := repository.Create(ctx, first); err != nil {
		t.Fatalf("first Create(): %v", err)
	}

	err = repository.Create(ctx, second)

	if !errors.Is(err, domain.ErrDuplicateTransaction) {
		t.Fatalf(
			"second Create() error = %v, want %v",
			err,
			domain.ErrDuplicateTransaction,
		)
	}
}

func TestLedgerRepository_GetByWalletID_Pagination(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)

	walletRepository := postgres.NewWalletRepository(pool)
	repository := postgres.NewLedgerRepository(pool)

	wallet := newTestWallet(t)

	if err := walletRepository.Create(ctx, wallet); err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	baseTime := time.Now().UTC().Truncate(time.Microsecond)

	transactionIDs := []uuid.UUID{
		uuid.New(),
		uuid.New(),
		uuid.New(),
	}

	t.Cleanup(func() {
		for _, transactionID := range transactionIDs {
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM wallet_ledger WHERE transaction_id = $1`,
				transactionID,
			)

			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM wager_transactions WHERE id = $1`,
				transactionID,
			)
		}

		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM wallets WHERE id = $1`,
			wallet.ID(),
		)
	})

	for i, transactionID := range transactionIDs {
		transactionTime := baseTime.Add(
			time.Duration(i) * time.Second,
		)

		amountMinor := int64((i + 1) * 1000)
		beforeMinor := int64(20000 - i*1000)
		afterMinor := beforeMinor - amountMinor

		insertWagerTransaction(
			t,
			ctx,
			pool,
			transactionID,
			wallet,
			amountMinor,
			afterMinor,
			transactionTime,
		)

		amount, err := domain.MoneyFromMinorUnits(
			amountMinor,
			domain.CurrencyBRL,
		)
		if err != nil {
			t.Fatalf("amount: %v", err)
		}

		before, err := domain.MoneyFromMinorUnits(
			beforeMinor,
			domain.CurrencyBRL,
		)
		if err != nil {
			t.Fatalf("balance before: %v", err)
		}

		after, err := domain.MoneyFromMinorUnits(
			afterMinor,
			domain.CurrencyBRL,
		)
		if err != nil {
			t.Fatalf("balance after: %v", err)
		}

		entry, err := domain.NewLedgerEntry(
			uuid.New(),
			wallet.ID(),
			transactionID,
			domain.LedgerDirectionDebit,
			amount,
			before,
			after,
			transactionTime,
		)
		if err != nil {
			t.Fatalf("NewLedgerEntry(): %v", err)
		}

		if err := repository.Create(ctx, entry); err != nil {
			t.Fatalf(
				"Create() %d: %v",
				i,
				err,
			)
		}
	}

	firstPage, nextCursor, err := repository.GetByWalletID(
		ctx,
		wallet.ID(),
		"",
		2,
	)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}

	if len(firstPage) != 2 {
		t.Fatalf(
			"first page length = %d, want 2",
			len(firstPage),
		)
	}

	if nextCursor == "" {
		t.Fatal("expected next cursor")
	}

	secondPage, finalCursor, err := repository.GetByWalletID(
		ctx,
		wallet.ID(),
		nextCursor,
		2,
	)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}

	if len(secondPage) != 1 {
		t.Fatalf(
			"second page length = %d, want 1",
			len(secondPage),
		)
	}

	if finalCursor != "" {
		t.Fatalf(
			"final cursor = %q, want empty",
			finalCursor,
		)
	}

	if firstPage[0].CreatedAt().Before(firstPage[1].CreatedAt()) {
		t.Fatal("first page is not ordered descending")
	}

	if firstPage[1].CreatedAt().Before(secondPage[0].CreatedAt()) {
		t.Fatal("pagination order is not descending")
	}
}

func TestLedgerRepository_RollbackWithinTransaction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)

	walletRepository := postgres.NewWalletRepository(pool)
	manager := postgres.NewTransactionManager(pool)
	repository := postgres.NewLedgerRepository(pool)

	wallet := newTestWallet(t)

	if err := walletRepository.Create(ctx, wallet); err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	transactionID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	insertWagerTransaction(
		t,
		ctx,
		pool,
		transactionID,
		wallet,
		1000,
		9000,
		now,
	)

	t.Cleanup(func() {
		cleanupLedgerData(
			t,
			ctx,
			pool,
			transactionID,
			wallet.ID(),
		)
	})

	amount, err := domain.MoneyFromMinorUnits(
		1000,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("amount: %v", err)
	}

	before, err := domain.MoneyFromMinorUnits(
		10000,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("balance before: %v", err)
	}

	after, err := domain.MoneyFromMinorUnits(
		9000,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("balance after: %v", err)
	}

	entry, err := domain.NewLedgerEntry(
		uuid.New(),
		wallet.ID(),
		transactionID,
		domain.LedgerDirectionDebit,
		amount,
		before,
		after,
		now,
	)
	if err != nil {
		t.Fatalf("NewLedgerEntry(): %v", err)
	}

	expectedErr := errors.New(
		"forced ledger transaction failure",
	)

	err = manager.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			if err := repository.Create(txCtx, entry); err != nil {
				return err
			}

			return expectedErr
		},
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"WithinTransaction() error = %v, want %v",
			err,
			expectedErr,
		)
	}

	_, err = repository.GetByTransactionID(
		ctx,
		transactionID,
	)

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"ledger entry after rollback: error = %v, want pgx.ErrNoRows",
			err,
		)
	}
}

func insertWagerTransaction(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	transactionID uuid.UUID,
	wallet domain.Wallet,
	amountMinor int64,
	resultBalanceMinor int64,
	now time.Time,
) {
	t.Helper()

	_, err := pool.Exec(
		ctx,
		`
		INSERT INTO wager_transactions (
			id,
			external_transaction_id,
			provider_id,
			idempotency_key,
			payload_hash,
			wallet_id,
			player_id,
			round_id,
			game_id,
			kind,
			amount_minor,
			currency,
			reference_external_id,
			reference_transaction_id,
			status,
			failure_code,
			result_balance_minor,
			result_balance_currency,
			created_at,
			updated_at
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
			'BET',
			$10,
			'BRL',
			'',
			NULL,
			'PROCESSED',
			'',
			$11,
			'BRL',
			$12,
			$12
		)
		`,
		transactionID,
		"external-"+transactionID.String(),
		"provider-"+transactionID.String(),
		"idempotency-"+transactionID.String(),
		"hash-"+transactionID.String(),
		wallet.ID(),
		wallet.PlayerID(),
		"round-"+transactionID.String(),
		"game-"+transactionID.String(),
		amountMinor,
		resultBalanceMinor,
		now,
	)

	if err != nil {
		t.Fatalf(
			"insert wager transaction: %v",
			err,
		)
	}
}

func cleanupLedgerData(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	transactionID uuid.UUID,
	walletID uuid.UUID,
) {
	t.Helper()

	_, _ = pool.Exec(
		ctx,
		`DELETE FROM wallet_ledger WHERE transaction_id = $1`,
		transactionID,
	)

	_, _ = pool.Exec(
		ctx,
		`DELETE FROM wager_transactions WHERE id = $1`,
		transactionID,
	)

	_, _ = pool.Exec(
		ctx,
		`DELETE FROM wallets WHERE id = $1`,
		walletID,
	)
}
