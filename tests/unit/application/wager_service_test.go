package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/junglegaming/backend-challenge-go/internal/application"
	"github.com/junglegaming/backend-challenge-go/internal/domain"
	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

type fakeWalletRepository struct {
	wallet          domain.Wallet
	getErr          error
	updateErr       error
	updateCalls     int
	lastExpectedVer int64
}

func (f *fakeWalletRepository) Create(
	_ context.Context,
	_ domain.Wallet,
) error {
	return nil
}

func (f *fakeWalletRepository) GetByID(
	_ context.Context,
	_ uuid.UUID,
) (domain.Wallet, error) {
	if f.getErr != nil {
		return domain.Wallet{}, f.getErr
	}

	return f.wallet, nil
}

func (f *fakeWalletRepository) GetByPlayerAndCurrency(
	_ context.Context,
	_ uuid.UUID,
	_ domain.Currency,
) (domain.Wallet, error) {
	return f.wallet, nil
}

func (f *fakeWalletRepository) UpdateBalance(
	_ context.Context,
	wallet domain.Wallet,
	expectedVersion int64,
) error {
	f.updateCalls++
	f.lastExpectedVer = expectedVersion

	if f.updateErr != nil {
		return f.updateErr
	}

	f.wallet = wallet

	return nil
}

type fakeTransactionRepository struct {
	transaction domain.WagerTransaction
	createErr   error
	updateErr   error
	createCalls int
	updateCalls int
}

func (f *fakeTransactionRepository) Create(
	_ context.Context,
	transaction domain.WagerTransaction,
) error {
	f.createCalls++

	if f.createErr != nil {
		return f.createErr
	}

	f.transaction = transaction

	return nil
}

func (f *fakeTransactionRepository) GetByID(
	_ context.Context,
	_ uuid.UUID,
) (domain.WagerTransaction, error) {
	return f.transaction, nil
}

func (f *fakeTransactionRepository) GetByProviderAndExternalTransaction(
	_ context.Context,
	_ string,
	_ string,
) (domain.WagerTransaction, error) {
	return f.transaction, nil
}

func (f *fakeTransactionRepository) GetByProviderAndExternalTransactionForUpdate(
	ctx context.Context,
	providerID string,
	externalTransactionID string,
) (domain.WagerTransaction, error) {
	return f.GetByProviderAndExternalTransaction(
		ctx,
		providerID,
		externalTransactionID,
	)
}

func (f *fakeTransactionRepository) GetByProviderAndReference(
	_ context.Context,
	_ string,
	_ string,
) (domain.WagerTransaction, error) {
	return domain.WagerTransaction{}, pgx.ErrNoRows
}

func (f *fakeTransactionRepository) GetByProviderReferenceAndKind(
	_ context.Context,
	_ string,
	_ string,
	_ domain.TransactionKind,
) (domain.WagerTransaction, error) {
	return domain.WagerTransaction{}, pgx.ErrNoRows
}

func (f *fakeTransactionRepository) GetPendingReferenceBatch(
	_ context.Context,
	_ int,
	_ time.Time,
) ([]ports.PendingReferenceTransaction, error) {
	return nil, nil
}

func (f *fakeTransactionRepository) UpdateReferenceRetry(
	_ context.Context,
	_ uuid.UUID,
	_ int,
	_ *time.Time,
) error {
	return nil
}

func (f *fakeTransactionRepository) Update(
	_ context.Context,
	transaction domain.WagerTransaction,
) error {
	f.updateCalls++

	if f.updateErr != nil {
		return f.updateErr
	}

	f.transaction = transaction

	return nil
}

type fakeLedgerRepository struct {
	createErr   error
	createCalls int
	entry       domain.WalletLedgerEntry
}

func (f *fakeLedgerRepository) Create(
	_ context.Context,
	entry domain.WalletLedgerEntry,
) error {
	f.createCalls++

	if f.createErr != nil {
		return f.createErr
	}

	f.entry = entry

	return nil
}

func (f *fakeLedgerRepository) GetByWalletID(
	_ context.Context,
	_ uuid.UUID,
	_ string,
	_ int,
) ([]domain.WalletLedgerEntry, string, error) {
	return nil, "", nil
}

func (f *fakeLedgerRepository) GetByTransactionID(
	_ context.Context,
	_ uuid.UUID,
) (domain.WalletLedgerEntry, error) {
	return f.entry, nil
}

type fakeIdempotencyRepository struct {
	record      ports.IdempotencyRecord
	findErr     error
	createErr   error
	updateErr   error
	findCalls   int
	createCalls int
	updateCalls int
}

func (f *fakeIdempotencyRepository) Find(
	_ context.Context,
	_ string,
	_ string,
) (ports.IdempotencyRecord, error) {
	f.findCalls++

	if f.findErr != nil {
		return ports.IdempotencyRecord{}, f.findErr
	}

	return f.record, nil
}

func (f *fakeIdempotencyRepository) Create(
	_ context.Context,
	record ports.IdempotencyRecord,
) error {
	f.createCalls++

	if f.createErr != nil {
		return f.createErr
	}

	f.record = record

	return nil
}

func (f *fakeIdempotencyRepository) Update(
	_ context.Context,
	record ports.IdempotencyRecord,
) error {
	f.updateCalls++

	if f.updateErr != nil {
		return f.updateErr
	}

	f.record = record

	return nil
}

type fakeOutboxRepository struct {
	createErr   error
	createCalls int
	event       ports.OutgoingEvent
}

func (f *fakeOutboxRepository) Create(
	_ context.Context,
	event ports.OutgoingEvent,
) error {
	f.createCalls++

	if f.createErr != nil {
		return f.createErr
	}

	f.event = event

	return nil
}

func (f *fakeOutboxRepository) ClaimBatch(
	_ context.Context,
	_ int,
	_ string,
) ([]ports.OutboxMessage, error) {
	return nil, nil
}

func (f *fakeOutboxRepository) MarkPublished(
	_ context.Context,
	_ string,
	_ string,
) error {
	return nil
}

func (f *fakeOutboxRepository) MarkFailed(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	_ string,
) error {
	return nil
}

type fakeTransactionManager struct {
	err           error
	callbackCalls int
}

func (f *fakeTransactionManager) WithinTransaction(
	ctx context.Context,
	fn func(ctx context.Context) error,
) error {
	f.callbackCalls++

	if f.err != nil {
		return f.err
	}

	return fn(ctx)
}

type testFixture struct {
	service       *application.WagerService
	wallets       *fakeWalletRepository
	transactions  *fakeTransactionRepository
	ledger        *fakeLedgerRepository
	idempotency   *fakeIdempotencyRepository
	outbox        *fakeOutboxRepository
	transactionDB *fakeTransactionManager
	input         application.PlaceBetInput
	amount        domain.Money
}

func newFixture(t *testing.T) testFixture {
	t.Helper()

	playerID := uuid.New()
	walletID := uuid.New()
	transactionID := uuid.New()

	amount, err := domain.NewMoney(
		"10.00",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("create amount: %v", err)
	}

	balance, err := domain.NewMoney(
		"100.00",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("create balance: %v", err)
	}

	wallet, err := domain.NewWallet(
		walletID,
		playerID,
		balance,
		testTime(),
	)
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	wallets := &fakeWalletRepository{
		wallet: wallet,
	}

	transactions := &fakeTransactionRepository{}
	ledger := &fakeLedgerRepository{}

	idempotency := &fakeIdempotencyRepository{
		findErr: pgx.ErrNoRows,
	}

	outbox := &fakeOutboxRepository{}
	transactionDB := &fakeTransactionManager{}

	service := application.NewWagerService(
		wallets,
		transactions,
		ledger,
		idempotency,
		outbox,
		transactionDB,
		nil,
	)

	return testFixture{
		service:       service,
		wallets:       wallets,
		transactions:  transactions,
		ledger:        ledger,
		idempotency:   idempotency,
		outbox:        outbox,
		transactionDB: transactionDB,
		amount:        amount,
		input: application.PlaceBetInput{
			ID:                    transactionID,
			ExternalTransactionID: "bet-001",
			ProviderID:            "provider-001",
			IdempotencyKey:        "idem-001",
			PayloadHash:           "payload-hash-001",
			WalletID:              walletID,
			PlayerID:              playerID,
			RoundID:               "round-001",
			GameID:                "game-001",
			Amount:                amount,
		},
	}
}

func testTime() time.Time {
	return time.Date(
		2026,
		1,
		1,
		12,
		0,
		0,
		0,
		time.UTC,
	)
}

func TestPlaceBetSuccess(t *testing.T) {
	fixture := newFixture(t)

	result, err := fixture.service.PlaceBet(
		context.Background(),
		fixture.input,
	)
	if err != nil {
		t.Fatalf("PlaceBet() error = %v", err)
	}

	if result.TransactionID != fixture.input.ID {
		t.Fatalf(
			"transaction id = %s, want %s",
			result.TransactionID,
			fixture.input.ID,
		)
	}

	if result.Balance.Amount() != 9000 {
		t.Fatalf(
			"balance = %d, want 9000",
			result.Balance.Amount(),
		)
	}

	if fixture.transactions.createCalls != 1 {
		t.Fatalf(
			"transaction Create() calls = %d, want 1",
			fixture.transactions.createCalls,
		)
	}

	if fixture.ledger.createCalls != 1 {
		t.Fatalf(
			"ledger Create() calls = %d, want 1",
			fixture.ledger.createCalls,
		)
	}

	if fixture.outbox.createCalls != 1 {
		t.Fatalf(
			"outbox Create() calls = %d, want 1",
			fixture.outbox.createCalls,
		)
	}
}

func TestPlaceBetInsufficientBalance(t *testing.T) {
	fixture := newFixture(t)

	amount, err := domain.NewMoney(
		"150.00",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("create amount: %v", err)
	}

	fixture.input.Amount = amount

	_, err = fixture.service.PlaceBet(
		context.Background(),
		fixture.input,
	)
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf(
			"PlaceBet() error = %v, want %v",
			err,
			domain.ErrInsufficientBalance,
		)
	}

	if fixture.wallets.wallet.Balance().Amount() != 10000 {
		t.Fatalf(
			"balance = %d, want 10000",
			fixture.wallets.wallet.Balance().Amount(),
		)
	}

	if fixture.ledger.createCalls != 0 {
		t.Fatalf(
			"ledger Create() calls = %d, want 0",
			fixture.ledger.createCalls,
		)
	}

	if fixture.outbox.createCalls != 1 {
		t.Fatalf(
			"outbox Create() calls = %d, want 1",
			fixture.outbox.createCalls,
		)
	}
}

func TestPlaceBetIdempotencyReplay(t *testing.T) {
	fixture := newFixture(t)

	responseBody, err := json.Marshal(struct {
		TransactionID string `json:"transactionId"`
		Status        string `json:"status"`
		Balance       string `json:"balance"`
		Currency      string `json:"currency"`
	}{
		TransactionID: fixture.input.ID.String(),
		Status:        string(domain.TransactionStatusProcessed),
		Balance:       fixture.wallets.wallet.Balance().String(),
		Currency:      string(domain.CurrencyBRL),
	})
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	fixture.idempotency.findErr = nil
	fixture.idempotency.record = ports.IdempotencyRecord{
		ProviderID:              fixture.input.ProviderID,
		IdempotencyKey:          fixture.input.IdempotencyKey,
		PayloadHash:             fixture.input.PayloadHash,
		TransactionID:           fixture.input.ID,
		Status:                  string(domain.TransactionStatusProcessed),
		ResponseBody:            responseBody,
		ObservedBalanceAmount:   10000,
		ObservedBalanceCurrency: string(domain.CurrencyBRL),
	}

	result, err := fixture.service.PlaceBet(
		context.Background(),
		fixture.input,
	)
	if err != nil {
		t.Fatalf("PlaceBet() error = %v", err)
	}

	if result.TransactionID != fixture.input.ID {
		t.Fatalf(
			"transaction id = %s, want %s",
			result.TransactionID,
			fixture.input.ID,
		)
	}

	if result.Status != domain.TransactionStatusProcessed {
		t.Fatalf(
			"status = %s, want %s",
			result.Status,
			domain.TransactionStatusProcessed,
		)
	}

	if result.Balance.Amount() != 10000 {
		t.Fatalf(
			"balance = %d, want 10000",
			result.Balance.Amount(),
		)
	}

	if !result.IdempotentReplay {
		t.Fatal("IdempotentReplay = false, want true")
	}

	if fixture.transactions.createCalls != 0 {
		t.Fatalf(
			"transaction Create() calls = %d, want 0",
			fixture.transactions.createCalls,
		)
	}

	if fixture.ledger.createCalls != 0 {
		t.Fatalf(
			"ledger Create() calls = %d, want 0",
			fixture.ledger.createCalls,
		)
	}

	if fixture.outbox.createCalls != 0 {
		t.Fatalf(
			"outbox Create() calls = %d, want 0",
			fixture.outbox.createCalls,
		)
	}
}

func TestPlaceBetTransactionCreateError(t *testing.T) {
	fixture := newFixture(t)

	createErr := errors.New("transaction create failed")

	fixture.transactions.createErr = createErr

	_, err := fixture.service.PlaceBet(
		context.Background(),
		fixture.input,
	)
	if !errors.Is(err, createErr) {
		t.Fatalf(
			"PlaceBet() error = %v, want %v",
			err,
			createErr,
		)
	}
}

func TestPlaceBetWalletError(t *testing.T) {
	fixture := newFixture(t)

	fixture.wallets.getErr = pgx.ErrNoRows

	_, err := fixture.service.PlaceBet(
		context.Background(),
		fixture.input,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"PlaceBet() error = %v, want %v",
			err,
			pgx.ErrNoRows,
		)
	}
}

func TestPlaceBetWalletUpdateError(t *testing.T) {
	fixture := newFixture(t)

	fixture.wallets.updateErr =
		domain.ErrConcurrentModification

	_, err := fixture.service.PlaceBet(
		context.Background(),
		fixture.input,
	)
	if !errors.Is(err, domain.ErrConcurrentModification) {
		t.Fatalf(
			"PlaceBet() error = %v, want %v",
			err,
			domain.ErrConcurrentModification,
		)
	}
}
