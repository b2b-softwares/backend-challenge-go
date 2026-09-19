package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/junglegaming/backend-challenge-go/internal/adapters/postgres"
	"github.com/junglegaming/backend-challenge-go/internal/application"
	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

func TestWagerService_SameIdempotencyKeyConcurrentRequests_ExecutesExactlyOnce(t *testing.T) {
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

	const concurrentRequests = 10

	// These values must be identical for every concurrent request in this
	// execution because the test verifies persistent idempotency.
	//
	// They must nevertheless be unique across test executions so a previous
	// successful execution cannot collide with the current wallet fixture.
	providerID := "same-provider-" + uuid.New().String()
	idempotencyKey := "same-idempotency-key-" + uuid.New().String()
	payloadHash := "same-payload-hash"

	type callResult struct {
		result application.WagerResult
		err    error
	}

	results := make([]callResult, concurrentRequests)

	var (
		start = make(chan struct{})
		wg    sync.WaitGroup
	)

	wg.Add(concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func(index int) {
			defer wg.Done()

			<-start

			input := application.PlaceBetInput{
				ID:                    uuid.New(),
				ExternalTransactionID: "same-external-" + uuid.New().String(),
				ProviderID:            providerID,
				IdempotencyKey:        idempotencyKey,
				PayloadHash:           payloadHash,
				WalletID:              walletID,
				PlayerID:              playerID,
				RoundID:               "round-idempotency",
				GameID:                "game-idempotency",
				Amount:                betAmount,
			}

			result, err := service.PlaceBet(ctx, input)

			results[index] = callResult{
				result: result,
				err:    err,
			}
		}(i)
	}

	close(start)
	wg.Wait()

	var transactionID uuid.UUID

	for i, result := range results {
		if result.err != nil {
			t.Fatalf(
				"request %d returned unexpected error: %v",
				i,
				result.err,
			)
		}

		if result.result.Status != domain.TransactionStatusProcessed {
			t.Fatalf(
				"request %d returned status %s, want %s",
				i,
				result.result.Status,
				domain.TransactionStatusProcessed,
			)
		}

		if result.result.Balance.Amount() != 8000 {
			t.Fatalf(
				"request %d returned balance %d, want 8000",
				i,
				result.result.Balance.Amount(),
			)
		}

		if transactionID == uuid.Nil {
			transactionID = result.result.TransactionID
			continue
		}

		if result.result.TransactionID != transactionID {
			t.Fatalf(
				"request %d returned transaction %s, want %s",
				i,
				result.result.TransactionID,
				transactionID,
			)
		}
	}

	if transactionID == uuid.Nil {
		t.Fatal("no transaction ID returned")
	}

	// ---------------------------------------------------------------------
	// Wallet: exactly one debit.
	// ---------------------------------------------------------------------

	var (
		finalBalance int64
		finalVersion int64
	)

	err = pool.QueryRow(
		ctx,
		`
		SELECT balance_minor, version
		FROM wallets
		WHERE id = $1
		`,
		walletID,
	).Scan(&finalBalance, &finalVersion)
	if err != nil {
		t.Fatalf("query final wallet: %v", err)
	}

	if finalBalance != 8000 {
		t.Fatalf(
			"final balance = %d, want 8000",
			finalBalance,
		)
	}

	if finalVersion != 2 {
		t.Fatalf(
			"final wallet version = %d, want 2",
			finalVersion,
		)
	}

	// ---------------------------------------------------------------------
	// Wager transactions: exactly one.
	// ---------------------------------------------------------------------

	var transactionCount int

	err = pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM wager_transactions
		WHERE wallet_id = $1
		`,
		walletID,
	).Scan(&transactionCount)
	if err != nil {
		t.Fatalf("count wager transactions: %v", err)
	}

	if transactionCount != 1 {
		t.Fatalf(
			"wager transaction count = %d, want 1",
			transactionCount,
		)
	}

	var processedCount int

	err = pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM wager_transactions
		WHERE wallet_id = $1
		  AND status = 'PROCESSED'
		`,
		walletID,
	).Scan(&processedCount)
	if err != nil {
		t.Fatalf("count processed transactions: %v", err)
	}

	if processedCount != 1 {
		t.Fatalf(
			"processed transaction count = %d, want 1",
			processedCount,
		)
	}

	// ---------------------------------------------------------------------
	// Ledger: exactly one debit.
	// ---------------------------------------------------------------------

	var ledgerCount int

	err = pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM wallet_ledger
		WHERE wallet_id = $1
		`,
		walletID,
	).Scan(&ledgerCount)
	if err != nil {
		t.Fatalf("count ledger entries: %v", err)
	}

	if ledgerCount != 1 {
		t.Fatalf(
			"ledger entry count = %d, want 1",
			ledgerCount,
		)
	}

	var (
		ledgerAmount        int64
		ledgerDirection     string
		ledgerTransactionID uuid.UUID
	)

	err = pool.QueryRow(
		ctx,
		`
		SELECT amount_minor, direction, transaction_id
		FROM wallet_ledger
		WHERE wallet_id = $1
		`,
		walletID,
	).Scan(
		&ledgerAmount,
		&ledgerDirection,
		&ledgerTransactionID,
	)
	if err != nil {
		t.Fatalf("query ledger entry: %v", err)
	}

	if ledgerAmount != 2000 {
		t.Fatalf(
			"ledger amount = %d, want 2000",
			ledgerAmount,
		)
	}

	if ledgerDirection != "DEBIT" {
		t.Fatalf(
			"ledger direction = %s, want DEBIT",
			ledgerDirection,
		)
	}

	if ledgerTransactionID != transactionID {
		t.Fatalf(
			"ledger transaction ID = %s, want %s",
			ledgerTransactionID,
			transactionID,
		)
	}

	// ---------------------------------------------------------------------
	// Idempotency: exactly one persistent record.
	// ---------------------------------------------------------------------

	var idempotencyCount int

	err = pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM idempotency_records
		WHERE provider_id = $1
		  AND idempotency_key = $2
		`,
		providerID,
		idempotencyKey,
	).Scan(&idempotencyCount)
	if err != nil {
		t.Fatalf("count idempotency records: %v", err)
	}

	if idempotencyCount != 1 {
		t.Fatalf(
			"idempotency record count = %d, want 1",
			idempotencyCount,
		)
	}

	var (
		idempotencyTransactionID uuid.UUID
		idempotencyStatus        string
		idempotencyPayloadHash   string
	)

	err = pool.QueryRow(
		ctx,
		`
		SELECT transaction_id, status, payload_hash
		FROM idempotency_records
		WHERE provider_id = $1
		  AND idempotency_key = $2
		`,
		providerID,
		idempotencyKey,
	).Scan(
		&idempotencyTransactionID,
		&idempotencyStatus,
		&idempotencyPayloadHash,
	)
	if err != nil {
		t.Fatalf("query idempotency record: %v", err)
	}

	if idempotencyTransactionID != transactionID {
		t.Fatalf(
			"idempotency transaction ID = %s, want %s",
			idempotencyTransactionID,
			transactionID,
		)
	}

	if idempotencyStatus != string(domain.TransactionStatusProcessed) {
		t.Fatalf(
			"idempotency status = %s, want %s",
			idempotencyStatus,
			domain.TransactionStatusProcessed,
		)
	}

	if idempotencyPayloadHash != payloadHash {
		t.Fatalf(
			"idempotency payload hash = %s, want %s",
			idempotencyPayloadHash,
			payloadHash,
		)
	}

	// ---------------------------------------------------------------------
	// Outbox: exactly one event.
	//
	// The wager service models the wallet as the aggregate. Therefore:
	//
	//   aggregate_id   = walletID
	//   correlation_id = transactionID
	//   causation_id   = transactionID
	//
	// The previous test incorrectly searched aggregate_id using transactionID.
	// ---------------------------------------------------------------------

	var outboxCount int

	err = pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM outbox_events
		WHERE aggregate_id = $1
		`,
		walletID.String(),
	).Scan(&outboxCount)
	if err != nil {
		t.Fatalf("count outbox events: %v", err)
	}

	if outboxCount != 1 {
		t.Fatalf(
			"outbox event count = %d, want 1",
			outboxCount,
		)
	}

	var (
		outboxEventType     string
		outboxAggregateID   string
		outboxCorrelationID string
		outboxCausationID   string
		outboxVersion       int
	)

	err = pool.QueryRow(
		ctx,
		`
		SELECT
			event_type,
			aggregate_id,
			correlation_id,
			causation_id,
			version
		FROM outbox_events
		WHERE aggregate_id = $1
		`,
		walletID.String(),
	).Scan(
		&outboxEventType,
		&outboxAggregateID,
		&outboxCorrelationID,
		&outboxCausationID,
		&outboxVersion,
	)
	if err != nil {
		t.Fatalf("query outbox event: %v", err)
	}

	if outboxEventType != "WAGER_BET_PROCESSED" {
		t.Fatalf(
			"outbox event type = %s, want WAGER_BET_PROCESSED",
			outboxEventType,
		)
	}

	if outboxAggregateID != walletID.String() {
		t.Fatalf(
			"outbox aggregate ID = %s, want %s",
			outboxAggregateID,
			walletID,
		)
	}

	if outboxCorrelationID != transactionID.String() {
		t.Fatalf(
			"outbox correlation ID = %s, want %s",
			outboxCorrelationID,
			transactionID,
		)
	}

	if outboxCausationID != transactionID.String() {
		t.Fatalf(
			"outbox causation ID = %s, want %s",
			outboxCausationID,
			transactionID,
		)
	}

	if outboxVersion != 2 {
		t.Fatalf(
			"outbox version = %d, want 2",
			outboxVersion,
		)
	}

	// ---------------------------------------------------------------------
	// No transaction left pending.
	// ---------------------------------------------------------------------

	var nonTerminalCount int

	err = pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM wager_transactions
		WHERE wallet_id = $1
		  AND status NOT IN ('PROCESSED', 'REJECTED', 'FAILED')
		`,
		walletID,
	).Scan(&nonTerminalCount)
	if err != nil {
		t.Fatalf("count non-terminal transactions: %v", err)
	}

	if nonTerminalCount != 0 {
		t.Fatalf(
			"non-terminal transaction count = %d, want 0",
			nonTerminalCount,
		)
	}
}
