package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/junglegaming/backend-challenge-go/internal/adapters/postgres"
	"github.com/junglegaming/backend-challenge-go/internal/application"
	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

func TestWagerService_ConcurrentBetsNeverProduceNegativeBalance(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	walletRepository := postgres.NewWalletRepository(pool)
	transactionRepository := postgres.NewWagerTransactionRepository(pool)
	ledgerRepository := postgres.NewLedgerRepository(pool)
	idempotencyRepository := postgres.NewIdempotencyRepository(pool)
	outboxRepository := postgres.NewOutboxRepository(pool)
	transactionManager := postgres.NewTransactionManager(pool)

	service := application.NewWagerService(
		walletRepository,
		transactionRepository,
		ledgerRepository,
		idempotencyRepository,
		outboxRepository,
		transactionManager,
	)

	playerID := uuid.New()
	walletID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	initialBalance := mustMoney(t, "100.00", domain.CurrencyBRL)
	betAmount := mustMoney(t, "20.00", domain.CurrencyBRL)

	wallet, err := domain.NewWallet(
		walletID,
		playerID,
		initialBalance,
		now,
	)
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	if err := walletRepository.Create(ctx, wallet); err != nil {
		t.Fatalf("persist wallet: %v", err)
	}

	t.Cleanup(func() {
		cleanupConcurrentBetFixture(t, ctx, pool, walletID)
	})

	const concurrentBets = 10

	var (
		start = make(chan struct{})
		wg    sync.WaitGroup
		mu    sync.Mutex

		successes           int
		concurrencyFailures int
		insufficientBalance int
		otherErrors         int
	)

	wg.Add(concurrentBets)

	for i := 0; i < concurrentBets; i++ {
		go func() {
			defer wg.Done()

			<-start

			input := application.PlaceBetInput{
				ID:                    uuid.New(),
				ExternalTransactionID: "concurrent-external-" + uuid.New().String(),
				ProviderID:            "concurrent-provider-" + uuid.New().String(),
				IdempotencyKey:        "concurrent-idempotency-" + uuid.New().String(),
				PayloadHash:           "concurrent-payload-" + uuid.New().String(),
				WalletID:              walletID,
				PlayerID:              playerID,
				RoundID:               "round-concurrent",
				GameID:                "game-concurrent",
				Amount:                betAmount,
			}

			_, err := service.PlaceBet(ctx, input)

			mu.Lock()
			defer mu.Unlock()

			switch {
			case err == nil:
				successes++

			case errors.Is(err, domain.ErrConcurrentModification):
				concurrencyFailures++

			case errors.Is(err, domain.ErrInsufficientBalance):
				insufficientBalance++

			default:
				otherErrors++
			}
		}()
	}

	close(start)
	wg.Wait()

	if otherErrors > 0 {
		t.Fatalf(
			"unexpected errors during concurrent betting: %d",
			otherErrors,
		)
	}

	if successes > 5 {
		t.Fatalf(
			"more successful bets than available balance allows: successes=%d",
			successes,
		)
	}

	if successes+concurrencyFailures+insufficientBalance != concurrentBets {
		t.Fatalf(
			"unexpected result accounting: total=%d successes=%d concurrencyFailures=%d insufficientBalance=%d",
			concurrentBets,
			successes,
			concurrencyFailures,
			insufficientBalance,
		)
	}

	finalWallet, err := walletRepository.GetByID(ctx, walletID)
	if err != nil {
		t.Fatalf("reload final wallet: %v", err)
	}

	if finalWallet.Balance().IsNegative() {
		t.Fatalf(
			"wallet has negative balance after concurrent bets: %s",
			finalWallet.Balance().String(),
		)
	}

	expectedBalanceMinor := int64(10000 - successes*2000)

	if finalWallet.Balance().Amount() != expectedBalanceMinor {
		t.Fatalf(
			"unexpected final balance: expected=%d got=%d successes=%d",
			expectedBalanceMinor,
			finalWallet.Balance().Amount(),
			successes,
		)
	}

	if finalWallet.Version() != int64(successes+1) {
		t.Fatalf(
			"unexpected wallet version: expected=%d got=%d successes=%d",
			successes+1,
			finalWallet.Version(),
			successes,
		)
	}

	ledgerEntries, _, err := ledgerRepository.GetByWalletID(
		ctx,
		walletID,
		"",
		100,
	)
	if err != nil {
		t.Fatalf("get ledger entries: %v", err)
	}

	if len(ledgerEntries) != successes {
		t.Fatalf(
			"unexpected ledger entries: expected=%d got=%d",
			successes,
			len(ledgerEntries),
		)
	}

	for _, entry := range ledgerEntries {
		if entry.Direction() != domain.LedgerDirectionDebit {
			t.Fatalf(
				"unexpected ledger direction: %s",
				entry.Direction(),
			)
		}

		if entry.Money().Amount() != 2000 {
			t.Fatalf(
				"unexpected ledger amount: expected=2000 got=%d",
				entry.Money().Amount(),
			)
		}

		if entry.BalanceAfter().IsNegative() {
			t.Fatalf(
				"ledger contains negative resulting balance: %s",
				entry.BalanceAfter().String(),
			)
		}
	}

	var persistedProcessed int

	err = pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM wager_transactions
		WHERE wallet_id = $1
		  AND status = $2
		`,
		walletID,
		string(domain.TransactionStatusProcessed),
	).Scan(&persistedProcessed)
	if err != nil {
		t.Fatalf("count processed transactions: %v", err)
	}

	if persistedProcessed != successes {
		t.Fatalf(
			"unexpected processed transactions: expected=%d got=%d",
			successes,
			persistedProcessed,
		)
	}

	var persistedTotal int

	err = pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM wager_transactions
		WHERE wallet_id = $1
		`,
		walletID,
	).Scan(&persistedTotal)
	if err != nil {
		t.Fatalf("count persisted transactions: %v", err)
	}

	expectedPersistedTransactions := successes + insufficientBalance

	if persistedTotal != expectedPersistedTransactions {
		t.Fatalf(
			"unexpected persisted transactions: expected=%d got=%d successes=%d insufficientBalance=%d concurrencyFailures=%d",
			expectedPersistedTransactions,
			persistedTotal,
			successes,
			insufficientBalance,
			concurrencyFailures,
		)
	}

	var outboxTotal int

	err = pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM outbox_events
		WHERE aggregate_id = $1
		`,
		walletID.String(),
	).Scan(&outboxTotal)
	if err != nil {
		t.Fatalf("count outbox events: %v", err)
	}

	expectedOutboxEvents := successes + insufficientBalance

	if outboxTotal != expectedOutboxEvents {
		t.Fatalf(
			"unexpected outbox events: expected=%d got=%d successes=%d insufficientBalance=%d concurrencyFailures=%d",
			expectedOutboxEvents,
			outboxTotal,
			successes,
			insufficientBalance,
			concurrencyFailures,
		)
	}

	var remainingPending int

	err = pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM wager_transactions
		WHERE wallet_id = $1
		  AND status NOT IN ($2, $3, $4)
		`,
		walletID,
		string(domain.TransactionStatusProcessed),
		string(domain.TransactionStatusRejected),
		string(domain.TransactionStatusFailed),
	).Scan(&remainingPending)
	if err != nil {
		t.Fatalf("count non-terminal transactions: %v", err)
	}

	if remainingPending != 0 {
		t.Fatalf(
			"unexpected non-terminal transactions after concurrent processing: %d",
			remainingPending,
		)
	}

	_, err = transactionRepository.GetByID(ctx, uuid.New())
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected unknown transaction lookup to return pgx.ErrNoRows, got %v",
			err,
		)
	}
}

func cleanupConcurrentBetFixture(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	walletID uuid.UUID,
) {
	t.Helper()

	_, _ = pool.Exec(
		ctx,
		`
		DELETE FROM outbox_events
		WHERE aggregate_id = $1
		`,
		walletID.String(),
	)

	_, _ = pool.Exec(
		ctx,
		`
		DELETE FROM idempotency_records
		WHERE transaction_id IN (
			SELECT id
			FROM wager_transactions
			WHERE wallet_id = $1
		)
		`,
		walletID,
	)

	_, _ = pool.Exec(
		ctx,
		`
		DELETE FROM wallet_ledger
		WHERE wallet_id = $1
		`,
		walletID,
	)

	_, _ = pool.Exec(
		ctx,
		`
		DELETE FROM wager_transactions
		WHERE wallet_id = $1
		`,
		walletID,
	)

	_, _ = pool.Exec(
		ctx,
		`
		DELETE FROM wallets
		WHERE id = $1
		`,
		walletID,
	)
}
