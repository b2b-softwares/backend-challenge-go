package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/junglegaming/backend-challenge-go/internal/adapters/postgres"
	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

func TestWalletRepository_CreateAndGetByID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWalletRepository(pool)

	wallet := newTestWallet(t)

	err := repository.Create(ctx, wallet)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repository.GetByID(ctx, wallet.ID())
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.ID() != wallet.ID() {
		t.Fatalf("ID = %s, want %s", got.ID(), wallet.ID())
	}

	if got.PlayerID() != wallet.PlayerID() {
		t.Fatalf("PlayerID = %s, want %s", got.PlayerID(), wallet.PlayerID())
	}

	if got.Currency() != wallet.Currency() {
		t.Fatalf("Currency = %s, want %s", got.Currency(), wallet.Currency())
	}

	if !got.Balance().Equal(wallet.Balance()) {
		t.Fatalf(
			"Balance = %s, want %s",
			got.Balance(),
			wallet.Balance(),
		)
	}

	if got.Version() != wallet.Version() {
		t.Fatalf("Version = %d, want %d", got.Version(), wallet.Version())
	}
}

func TestWalletRepository_GetByPlayerAndCurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWalletRepository(pool)

	wallet := newTestWallet(t)

	if err := repository.Create(ctx, wallet); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repository.GetByPlayerAndCurrency(
		ctx,
		wallet.PlayerID(),
		wallet.Currency(),
	)
	if err != nil {
		t.Fatalf("GetByPlayerAndCurrency() error = %v", err)
	}

	if got.ID() != wallet.ID() {
		t.Fatalf("ID = %s, want %s", got.ID(), wallet.ID())
	}
}

func TestWalletRepository_GetByID_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWalletRepository(pool)

	_, err := repository.GetByID(ctx, uuid.New())
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetByID() error = %v, want pgx.ErrNoRows", err)
	}
}

func TestWalletRepository_UpdateBalance_OptimisticLocking(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWalletRepository(pool)

	wallet := newTestWallet(t)

	if err := repository.Create(ctx, wallet); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updatedWallet := wallet

	if err := updatedWallet.Debit(
		mustMoney(t, "10.00", wallet.Currency()),
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("Debit() error = %v", err)
	}

	if err := repository.UpdateBalance(
		ctx,
		updatedWallet,
		wallet.Version(),
	); err != nil {
		t.Fatalf("UpdateBalance() error = %v", err)
	}

	got, err := repository.GetByID(ctx, wallet.ID())
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.Balance().Amount() != 9000 {
		t.Fatalf(
			"Balance = %d, want 9000",
			got.Balance().Amount(),
		)
	}

	if got.Version() != 2 {
		t.Fatalf(
			"Version = %d, want 2",
			got.Version(),
		)
	}

	err = repository.UpdateBalance(
		ctx,
		updatedWallet,
		wallet.Version(),
	)
	if !errors.Is(err, domain.ErrConcurrentModification) {
		t.Fatalf(
			"stale UpdateBalance() error = %v, want %v",
			err,
			domain.ErrConcurrentModification,
		)
	}
}

func TestWalletRepository_Create_DuplicatePlayerCurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)
	repository := postgres.NewWalletRepository(pool)

	first := newTestWallet(t)

	if err := repository.Create(ctx, first); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	secondID := uuid.New()

	second, err := domain.NewWallet(
		secondID,
		first.PlayerID(),
		first.Balance(),
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("NewWallet() error = %v", err)
	}

	if err := repository.Create(ctx, second); !errors.Is(err, domain.ErrInvalidWallet) {
		t.Fatalf(
			"duplicate Create() error = %v, want %v",
			err,
			domain.ErrInvalidWallet,
		)
	}
}

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://backend:backend@localhost:5432/backend_challenge"
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("PostgreSQL ping error = %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func newTestWallet(t *testing.T) domain.Wallet {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Microsecond)

	money := mustMoney(t, "100.00", domain.CurrencyBRL)

	wallet, err := domain.NewWallet(
		uuid.New(),
		uuid.New(),
		money,
		now,
	)
	if err != nil {
		t.Fatalf("NewWallet() error = %v", err)
	}

	return wallet
}

func mustMoney(
	t *testing.T,
	amount string,
	currency domain.Currency,
) domain.Money {
	t.Helper()

	money, err := domain.NewMoney(amount, currency)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	return money
}
