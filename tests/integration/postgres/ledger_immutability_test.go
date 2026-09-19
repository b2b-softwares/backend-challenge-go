package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/junglegaming/backend-challenge-go/internal/adapters/postgres"
	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

func TestLedgerImmutability_UpdateIsRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)

	walletRepository := postgres.NewWalletRepository(pool)
	ledgerRepository := postgres.NewLedgerRepository(pool)

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

	if err := ledgerRepository.Create(ctx, entry); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`
		UPDATE wallet_ledger
		SET amount_minor = 500
		WHERE id = $1
		`,
		entry.ID(),
	)

	if err == nil {
		t.Fatal("UPDATE wallet_ledger succeeded; expected rejection")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf(
			"expected PostgreSQL error, got %T: %v",
			err,
			err,
		)
	}

	if pgErr.Code != "55000" {
		t.Fatalf(
			"PostgreSQL error code = %s, want 55000",
			pgErr.Code,
		)
	}

	found, err := ledgerRepository.GetByTransactionID(
		ctx,
		transactionID,
	)
	if err != nil {
		t.Fatalf(
			"GetByTransactionID() after rejected UPDATE: %v",
			err,
		)
	}

	if found.Money().Amount() != 1000 {
		t.Fatalf(
			"ledger amount changed to %d, want 1000",
			found.Money().Amount(),
		)
	}
}

func TestLedgerImmutability_DeleteIsRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := newTestPool(t)

	walletRepository := postgres.NewWalletRepository(pool)
	ledgerRepository := postgres.NewLedgerRepository(pool)

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

	if err := ledgerRepository.Create(ctx, entry); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`
		DELETE FROM wallet_ledger
		WHERE id = $1
		`,
		entry.ID(),
	)

	if err == nil {
		t.Fatal("DELETE wallet_ledger succeeded; expected rejection")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf(
			"expected PostgreSQL error, got %T: %v",
			err,
			err,
		)
	}

	if pgErr.Code != "55000" {
		t.Fatalf(
			"PostgreSQL error code = %s, want 55000",
			pgErr.Code,
		)
	}

	found, err := ledgerRepository.GetByTransactionID(
		ctx,
		transactionID,
	)
	if err != nil {
		t.Fatalf(
			"GetByTransactionID() after rejected DELETE: %v",
			err,
		)
	}

	if found.ID() != entry.ID() {
		t.Fatalf(
			"ledger entry was deleted; got ID %s, want %s",
			found.ID(),
			entry.ID(),
		)
	}
}
