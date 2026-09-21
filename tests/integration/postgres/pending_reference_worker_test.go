package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/junglegaming/backend-challenge-go/internal/adapters/postgres"
	"github.com/junglegaming/backend-challenge-go/internal/application"
	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

func TestPendingReferenceWorker_ResolvesPendingReversal(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	// The worker processes pending references globally. Remove leftovers from
	// previous integration-test runs so this test owns the pending-reference
	// queue it is about to process.
	if _, err := pool.Exec(
		ctx,
		`
		DELETE FROM idempotency_records
		WHERE transaction_id IN (
			SELECT id
			FROM wager_transactions
			WHERE status = 'PENDING_REFERENCE'
		)
		`,
	); err != nil {
		t.Fatalf("cleanup pending reference idempotency records: %v", err)
	}

	if _, err := pool.Exec(
		ctx,
		`DELETE FROM wager_transactions WHERE status = 'PENDING_REFERENCE'`,
	); err != nil {
		t.Fatalf("cleanup pending reference transactions: %v", err)
	}

	walletRepository := postgres.NewWalletRepository(pool)
	transactionRepository := postgres.NewWagerTransactionRepository(pool)
	ledgerRepository := postgres.NewLedgerRepository(pool)
	idempotencyRepository := postgres.NewIdempotencyRepository(pool)
	outboxRepository := postgres.NewOutboxRepository(pool)
	transactionManager := postgres.NewTransactionManager(pool)

	reversalService := application.NewReversalService(
		walletRepository,
		transactionRepository,
		ledgerRepository,
		idempotencyRepository,
		outboxRepository,
		transactionManager,
	)

	worker := application.NewPendingReferenceWorker(
		reversalService,
		transactionManager,
		transactionRepository,
	)

	now := time.Now().UTC().Truncate(time.Microsecond)

	playerID := uuid.New()
	walletID := uuid.New()
	providerID := "provider-" + uuid.New().String()

	walletMoney := mustMoney(t, "100.00", domain.CurrencyBRL)

	wallet, err := domain.NewWallet(
		walletID,
		playerID,
		walletMoney,
		now,
	)
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	if err := walletRepository.Create(ctx, wallet); err != nil {
		t.Fatalf("create wallet in database: %v", err)
	}

	referenceExternalID := "reference-" + uuid.New().String()
	reversalExternalID := "reversal-" + uuid.New().String()
	reversalID := uuid.New()

	reversalIdempotencyKey := "idempotency-" + uuid.New().String()
	payloadHash := "payload-" + uuid.New().String()

	t.Cleanup(func() {
		cleanupWagerTransaction(t, ctx, pool, reversalID, walletID)
	})

	reversalInput := application.ReverseTransactionInput{
		ID:                    reversalID,
		ExternalTransactionID: reversalExternalID,
		ProviderID:            providerID,
		IdempotencyKey:        reversalIdempotencyKey,
		PayloadHash:           payloadHash,
		WalletID:              walletID,
		PlayerID:              playerID,
		RoundID:               "round-" + uuid.New().String(),
		GameID:                "game-" + uuid.New().String(),
		Kind:                  domain.TransactionKindRefund,
		ReferenceExternalID:   referenceExternalID,
	}

	t.Logf(
		"reverse input: id=%s external=%q provider=%q idem=%q payload=%q wallet=%s player=%s kind=%q reference=%q",
		reversalInput.ID,
		reversalInput.ExternalTransactionID,
		reversalInput.ProviderID,
		reversalInput.IdempotencyKey,
		reversalInput.PayloadHash,
		reversalInput.WalletID,
		reversalInput.PlayerID,
		reversalInput.Kind,
		reversalInput.ReferenceExternalID,
	)

	reversalResult, err := reversalService.Reverse(
		ctx,
		reversalInput,
	)
	if err != nil {
		t.Fatalf("create pending reversal: %v", err)
	}

	if reversalResult.Status != domain.TransactionStatusPendingReference {
		t.Fatalf(
			"expected reversal status %s, got %s",
			domain.TransactionStatusPendingReference,
			reversalResult.Status,
		)
	}

	pending, err := transactionRepository.GetByID(ctx, reversalID)
	if err != nil {
		t.Fatalf("get pending reversal: %v", err)
	}

	if pending.Status() != domain.TransactionStatusPendingReference {
		t.Fatalf(
			"expected persisted reversal status %s, got %s",
			domain.TransactionStatusPendingReference,
			pending.Status(),
		)
	}

	referenceMoney := mustMoney(t, "10.00", domain.CurrencyBRL)

	reference, err := domain.NewExternalTransaction(
		uuid.New(),
		referenceExternalID,
		providerID,
		"reference-idempotency-"+uuid.New().String(),
		"reference-payload-"+uuid.New().String(),
		walletID,
		playerID,
		"round-reference-"+uuid.New().String(),
		"game-reference-"+uuid.New().String(),
		domain.TransactionKindBet,
		referenceMoney,
		"",
		now,
	)
	if err != nil {
		t.Fatalf("create reference transaction: %v", err)
	}

	referenceBalance := mustMoney(t, "90.00", domain.CurrencyBRL)

	if err := reference.MarkProcessed(referenceBalance, now); err != nil {
		t.Fatalf("mark reference as processed: %v", err)
	}

	if err := transactionRepository.Create(ctx, reference); err != nil {
		t.Fatalf("persist reference transaction: %v", err)
	}

	t.Cleanup(func() {
		cleanupWagerTransaction(t, ctx, pool, reference.ID(), walletID)
	})

	availableAt := time.Now().UTC().Add(-time.Second)

	if err := transactionRepository.UpdateReferenceRetry(
		ctx,
		reversalID,
		0,
		&availableAt,
	); err != nil {
		t.Fatalf("make pending reversal available: %v", err)
	}

	var (
		status        string
		attempts      int
		availableAtDB *time.Time
		expiresAt     *time.Time
		amountMinor   int64
	)

	err = pool.QueryRow(
		ctx,
		`
		SELECT
			status,
			reference_attempts,
			reference_available_at,
			reference_expires_at,
			amount_minor
		FROM wager_transactions
		WHERE id = $1
		`,
		reversalID,
	).Scan(
		&status,
		&attempts,
		&availableAtDB,
		&expiresAt,
		&amountMinor,
	)
	if err != nil {
		t.Fatalf("inspect pending reversal: %v", err)
	}

	t.Logf(
		"BEFORE WORKER: status=%q attempts=%d available_at=%v expires_at=%v amount_minor=%d",
		status,
		attempts,
		availableAtDB,
		expiresAt,
		amountMinor,
	)

	processed, err := worker.ProcessBatch(ctx, 1)
	if err != nil {
		t.Fatalf("process pending reference batch: %v", err)
	}

	if processed != 1 {
		t.Fatalf("expected 1 processed transaction, got %d", processed)
	}

	reversal, err := transactionRepository.GetByID(ctx, reversalID)
	if err != nil {
		t.Fatalf("get processed reversal: %v", err)
	}

	if reversal.Status() != domain.TransactionStatusProcessed {
		t.Fatalf(
			"expected reversal status %s, got %s",
			domain.TransactionStatusProcessed,
			reversal.Status(),
		)
	}

	if reversal.ReferenceExternalID() != referenceExternalID {
		t.Fatalf(
			"expected reference external id %s, got %s",
			referenceExternalID,
			reversal.ReferenceExternalID(),
		)
	}

	if reversal.ReferenceTransactionID() == nil {
		t.Fatal("expected reversal reference transaction id to be populated")
	}

	if *reversal.ReferenceTransactionID() != reference.ID() {
		t.Fatalf(
			"expected reference transaction id %s, got %s",
			reference.ID(),
			*reversal.ReferenceTransactionID(),
		)
	}

	if !reversal.Money().Equal(referenceMoney) {
		t.Fatalf(
			"expected reversal money %s, got %s",
			referenceMoney,
			reversal.Money(),
		)
	}

	updatedWallet, err := walletRepository.GetByID(ctx, walletID)
	if err != nil {
		t.Fatalf("get updated wallet: %v", err)
	}

	expectedBalance := mustMoney(t, "110.00", domain.CurrencyBRL)

	if !updatedWallet.Balance().Equal(expectedBalance) {
		t.Fatalf(
			"expected wallet balance %s, got %s",
			expectedBalance,
			updatedWallet.Balance(),
		)
	}

	ledgerEntry, err := ledgerRepository.GetByTransactionID(ctx, reversalID)
	if err != nil {
		t.Fatalf("get reversal ledger entry: %v", err)
	}

	if ledgerEntry.TransactionID() != reversalID {
		t.Fatalf(
			"expected ledger transaction id %s, got %s",
			reversalID,
			ledgerEntry.TransactionID(),
		)
	}

	if !ledgerEntry.Money().Equal(referenceMoney) {
		t.Fatalf(
			"expected ledger money %s, got %s",
			referenceMoney,
			ledgerEntry.Money(),
		)
	}

	if ledgerEntry.Direction() != domain.LedgerDirectionCredit {
		t.Fatalf(
			"expected ledger direction %s, got %s",
			domain.LedgerDirectionCredit,
			ledgerEntry.Direction(),
		)
	}
}
