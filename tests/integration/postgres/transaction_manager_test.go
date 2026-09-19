package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/junglegaming/backend-challenge-go/internal/adapters/postgres"
	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

func TestTransactionManager_RollbackOnError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)
	manager := postgres.NewTransactionManager(pool)

	walletID := uuid.New()
	playerID := uuid.New()
	now := time.Now().UTC()

	_, err := pool.Exec(
		ctx,
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
		VALUES ($1, $2, 'BRL', 10000, 1, $3, $3)
		`,
		walletID,
		playerID,
		now,
	)
	if err != nil {
		t.Fatalf("setup wallet: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM wallets WHERE id = $1`,
			walletID,
		)
	})

	expectedErr := errors.New("forced transaction failure")

	err = manager.WithinTransaction(ctx, func(txCtx context.Context) error {
		_ = txCtx

		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"WithinTransaction() error = %v, want %v",
			err,
			expectedErr,
		)
	}

	var balance int64

	err = pool.QueryRow(
		ctx,
		`
		SELECT balance_minor
		FROM wallets
		WHERE id = $1
		`,
		walletID,
	).Scan(&balance)

	if err != nil {
		t.Fatalf("query balance: %v", err)
	}

	if balance != 10000 {
		t.Fatalf(
			"balance after rollback = %d, want 10000",
			balance,
		)
	}
}

func TestTransactionManager_Commit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)
	manager := postgres.NewTransactionManager(pool)

	walletID := uuid.New()
	playerID := uuid.New()
	now := time.Now().UTC()

	_, err := pool.Exec(
		ctx,
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
		VALUES ($1, $2, 'BRL', 10000, 1, $3, $3)
		`,
		walletID,
		playerID,
		now,
	)
	if err != nil {
		t.Fatalf("setup wallet: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM wallets WHERE id = $1`,
			walletID,
		)
	})

	err = manager.WithinTransaction(ctx, func(txCtx context.Context) error {
		_ = txCtx

		return nil
	})

	if err != nil {
		t.Fatalf("WithinTransaction() error = %v", err)
	}

	var balance int64

	err = pool.QueryRow(
		ctx,
		`
		SELECT balance_minor
		FROM wallets
		WHERE id = $1
		`,
		walletID,
	).Scan(&balance)

	if err != nil {
		t.Fatalf("query balance: %v", err)
	}

	if balance != 10000 {
		t.Fatalf(
			"balance after commit = %d, want 10000",
			balance,
		)
	}
}

func TestTransactionManager_CallbackReceivesContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)
	manager := postgres.NewTransactionManager(pool)

	var callbackCalled bool

	err := manager.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			callbackCalled = true

			if txCtx == nil {
				return errors.New("transaction context is nil")
			}

			return nil
		},
	)

	if err != nil {
		t.Fatalf("WithinTransaction() error = %v", err)
	}

	if !callbackCalled {
		t.Fatal("transaction callback was not called")
	}
}

func TestTransactionManager_WalletRepositoryRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)

	manager := postgres.NewTransactionManager(pool)
	repository := postgres.NewWalletRepository(pool)

	walletID := uuid.New()
	playerID := uuid.New()
	now := time.Now().UTC()

	money, err := domain.MoneyFromMinorUnits(
		10000,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("create money: %v", err)
	}

	wallet, err := domain.NewWallet(
		walletID,
		playerID,
		money,
		now,
	)
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	if err := repository.Create(ctx, wallet); err != nil {
		t.Fatalf("create wallet in repository: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM wallets WHERE id = $1`,
			walletID,
		)
	})

	debit, err := domain.MoneyFromMinorUnits(
		2500,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("create debit money: %v", err)
	}

	expectedErr := errors.New("forced repository transaction failure")

	err = manager.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			txWallet, err := repository.GetByID(
				txCtx,
				walletID,
			)
			if err != nil {
				return err
			}

			expectedVersion := txWallet.Version()

			if err := txWallet.Debit(debit, time.Now().UTC()); err != nil {
				return err
			}

			if err := repository.UpdateBalance(
				txCtx,
				txWallet,
				expectedVersion,
			); err != nil {
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

	persistedWallet, err := repository.GetByID(
		ctx,
		walletID,
	)
	if err != nil {
		t.Fatalf("get wallet after rollback: %v", err)
	}

	if persistedWallet.Balance().Amount() != 10000 {
		t.Fatalf(
			"balance after repository rollback = %d, want 10000",
			persistedWallet.Balance().Amount(),
		)
	}

	if persistedWallet.Version() != 1 {
		t.Fatalf(
			"version after repository rollback = %d, want 1",
			persistedWallet.Version(),
		)
	}
}
