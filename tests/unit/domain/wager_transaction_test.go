package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

func transactionTestNow() time.Time {
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

func newTestTransaction(
	t *testing.T,
	kind domain.TransactionKind,
	money domain.Money,
	referenceExternalID string,
) domain.WagerTransaction {
	t.Helper()

	transaction, err := domain.NewExternalTransaction(
		uuid.New(),
		"external-transaction-001",
		"provider-001",
		"idempotency-001",
		"payload-hash-001",
		uuid.New(),
		uuid.New(),
		"round-001",
		"game-001",
		kind,
		money,
		referenceExternalID,
		transactionTestNow(),
	)
	if err != nil {
		t.Fatalf(
			"NewExternalTransaction() error = %v",
			err,
		)
	}

	return transaction
}

func newTransactionMoney(
	t *testing.T,
	amount string,
	currency domain.Currency,
) domain.Money {
	t.Helper()

	money, err := domain.NewMoney(amount, currency)
	if err != nil {
		t.Fatalf(
			"NewMoney(%q, %q) error = %v",
			amount,
			currency,
			err,
		)
	}

	return money
}

func TestNewExternalTransaction(t *testing.T) {
	money := newTransactionMoney(
		t,
		"100.00",
		domain.CurrencyBRL,
	)

	transaction, err := domain.NewExternalTransaction(
		uuid.New(),
		"external-001",
		"provider-001",
		"idempotency-001",
		"hash-001",
		uuid.New(),
		uuid.New(),
		"round-001",
		"game-001",
		domain.TransactionKindBet,
		money,
		"",
		transactionTestNow(),
	)
	if err != nil {
		t.Fatalf(
			"NewExternalTransaction() error = %v",
			err,
		)
	}

	if transaction.ID() == uuid.Nil {
		t.Fatal("ID() is nil")
	}

	if transaction.ExternalTransactionID() != "external-001" {
		t.Fatalf(
			"ExternalTransactionID() = %q, want %q",
			transaction.ExternalTransactionID(),
			"external-001",
		)
	}

	if transaction.ProviderID() != "provider-001" {
		t.Fatalf(
			"ProviderID() = %q, want %q",
			transaction.ProviderID(),
			"provider-001",
		)
	}

	if transaction.IdempotencyKey() != "idempotency-001" {
		t.Fatalf(
			"IdempotencyKey() = %q, want %q",
			transaction.IdempotencyKey(),
			"idempotency-001",
		)
	}

	if transaction.PayloadHash() != "hash-001" {
		t.Fatalf(
			"PayloadHash() = %q, want %q",
			transaction.PayloadHash(),
			"hash-001",
		)
	}

	if transaction.Kind() != domain.TransactionKindBet {
		t.Fatalf(
			"Kind() = %q, want %q",
			transaction.Kind(),
			domain.TransactionKindBet,
		)
	}

	if !transaction.Money().Equal(money) {
		t.Fatalf(
			"Money() = %s, want %s",
			transaction.Money(),
			money,
		)
	}

	if transaction.Status() != domain.TransactionStatusPending {
		t.Fatalf(
			"Status() = %q, want %q",
			transaction.Status(),
			domain.TransactionStatusPending,
		)
	}

	if transaction.ReferenceExternalID() != "" {
		t.Fatalf(
			"ReferenceExternalID() = %q, want empty",
			transaction.ReferenceExternalID(),
		)
	}

	if transaction.ReferenceTransactionID() != nil {
		t.Fatal("ReferenceTransactionID() is not nil")
	}

	if transaction.FailureCode() != "" {
		t.Fatalf(
			"FailureCode() = %q, want empty",
			transaction.FailureCode(),
		)
	}

	if transaction.ResultBalance() != nil {
		t.Fatal("ResultBalance() is not nil")
	}

	if !transaction.CreatedAt().Equal(transactionTestNow()) {
		t.Fatalf(
			"CreatedAt() = %v, want %v",
			transaction.CreatedAt(),
			transactionTestNow(),
		)
	}

	if !transaction.UpdatedAt().Equal(transactionTestNow()) {
		t.Fatalf(
			"UpdatedAt() = %v, want %v",
			transaction.UpdatedAt(),
			transactionTestNow(),
		)
	}
}

func TestNewExternalTransactionRejectsOpening(t *testing.T) {
	money := newTransactionMoney(
		t,
		"100.00",
		domain.CurrencyBRL,
	)

	_, err := domain.NewExternalTransaction(
		uuid.New(),
		"external-001",
		"provider-001",
		"idempotency-001",
		"hash-001",
		uuid.New(),
		uuid.New(),
		"round-001",
		"game-001",
		domain.TransactionKindOpening,
		money,
		"",
		transactionTestNow(),
	)

	if !errors.Is(err, domain.ErrInvalidOpening) {
		t.Fatalf(
			"NewExternalTransaction() error = %v, want ErrInvalidOpening",
			err,
		)
	}
}

func TestNewExternalTransactionRejectsInvalidKind(t *testing.T) {
	money := newTransactionMoney(
		t,
		"100.00",
		domain.CurrencyBRL,
	)

	_, err := domain.NewExternalTransaction(
		uuid.New(),
		"external-001",
		"provider-001",
		"idempotency-001",
		"hash-001",
		uuid.New(),
		uuid.New(),
		"round-001",
		"game-001",
		domain.TransactionKind("INVALID"),
		money,
		"",
		transactionTestNow(),
	)

	if !errors.Is(err, domain.ErrInvalidTransactionKind) {
		t.Fatalf(
			"NewExternalTransaction() error = %v, want ErrInvalidTransactionKind",
			err,
		)
	}
}

func TestNewExternalTransactionRejectsMissingRequiredFields(t *testing.T) {
	money := newTransactionMoney(
		t,
		"100.00",
		domain.CurrencyBRL,
	)

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "nil transaction id",
			fn: func() error {
				_, err := domain.NewExternalTransaction(
					uuid.Nil,
					"external-001",
					"provider-001",
					"idempotency-001",
					"hash-001",
					uuid.New(),
					uuid.New(),
					"round-001",
					"game-001",
					domain.TransactionKindBet,
					money,
					"",
					transactionTestNow(),
				)
				return err
			},
		},
		{
			name: "empty external transaction id",
			fn: func() error {
				_, err := domain.NewExternalTransaction(
					uuid.New(),
					"",
					"provider-001",
					"idempotency-001",
					"hash-001",
					uuid.New(),
					uuid.New(),
					"round-001",
					"game-001",
					domain.TransactionKindBet,
					money,
					"",
					transactionTestNow(),
				)
				return err
			},
		},
		{
			name: "empty provider id",
			fn: func() error {
				_, err := domain.NewExternalTransaction(
					uuid.New(),
					"external-001",
					"",
					"idempotency-001",
					"hash-001",
					uuid.New(),
					uuid.New(),
					"round-001",
					"game-001",
					domain.TransactionKindBet,
					money,
					"",
					transactionTestNow(),
				)
				return err
			},
		},
		{
			name: "empty idempotency key",
			fn: func() error {
				_, err := domain.NewExternalTransaction(
					uuid.New(),
					"external-001",
					"provider-001",
					"",
					"hash-001",
					uuid.New(),
					uuid.New(),
					"round-001",
					"game-001",
					domain.TransactionKindBet,
					money,
					"",
					transactionTestNow(),
				)
				return err
			},
		},
		{
			name: "empty payload hash",
			fn: func() error {
				_, err := domain.NewExternalTransaction(
					uuid.New(),
					"external-001",
					"provider-001",
					"idempotency-001",
					"",
					uuid.New(),
					uuid.New(),
					"round-001",
					"game-001",
					domain.TransactionKindBet,
					money,
					"",
					transactionTestNow(),
				)
				return err
			},
		},
		{
			name: "nil wallet id",
			fn: func() error {
				_, err := domain.NewExternalTransaction(
					uuid.New(),
					"external-001",
					"provider-001",
					"idempotency-001",
					"hash-001",
					uuid.Nil,
					uuid.New(),
					"round-001",
					"game-001",
					domain.TransactionKindBet,
					money,
					"",
					transactionTestNow(),
				)
				return err
			},
		},
		{
			name: "nil player id",
			fn: func() error {
				_, err := domain.NewExternalTransaction(
					uuid.New(),
					"external-001",
					"provider-001",
					"idempotency-001",
					"hash-001",
					uuid.New(),
					uuid.Nil,
					"round-001",
					"game-001",
					domain.TransactionKindBet,
					money,
					"",
					transactionTestNow(),
				)
				return err
			},
		},
		{
			name: "zero timestamp",
			fn: func() error {
				_, err := domain.NewExternalTransaction(
					uuid.New(),
					"external-001",
					"provider-001",
					"idempotency-001",
					"hash-001",
					uuid.New(),
					uuid.New(),
					"round-001",
					"game-001",
					domain.TransactionKindBet,
					money,
					"",
					time.Time{},
				)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()

			if !errors.Is(err, domain.ErrInvalidTransaction) {
				t.Fatalf(
					"error = %v, want ErrInvalidTransaction",
					err,
				)
			}
		})
	}
}

func TestNewExternalTransactionLossRequiresZeroMoney(t *testing.T) {
	zero := newTransactionMoney(
		t,
		"0.00",
		domain.CurrencyBRL,
	)

	transaction, err := domain.NewExternalTransaction(
		uuid.New(),
		"external-loss",
		"provider-001",
		"idempotency-loss",
		"hash-loss",
		uuid.New(),
		uuid.New(),
		"round-001",
		"game-001",
		domain.TransactionKindLoss,
		zero,
		"",
		transactionTestNow(),
	)
	if err != nil {
		t.Fatalf(
			"NewExternalTransaction() error = %v",
			err,
		)
	}

	if transaction.Kind() != domain.TransactionKindLoss {
		t.Fatalf(
			"Kind() = %q, want %q",
			transaction.Kind(),
			domain.TransactionKindLoss,
		)
	}

	if !transaction.Money().IsZero() {
		t.Fatal("LOSS transaction money is not zero")
	}
}

func TestNewExternalTransactionLossRejectsPositiveMoney(t *testing.T) {
	money := newTransactionMoney(
		t,
		"0.01",
		domain.CurrencyBRL,
	)

	_, err := domain.NewExternalTransaction(
		uuid.New(),
		"external-loss",
		"provider-001",
		"idempotency-loss",
		"hash-loss",
		uuid.New(),
		uuid.New(),
		"round-001",
		"game-001",
		domain.TransactionKindLoss,
		money,
		"",
		transactionTestNow(),
	)

	if !errors.Is(err, domain.ErrInvalidMoney) {
		t.Fatalf(
			"error = %v, want ErrInvalidMoney",
			err,
		)
	}
}

func TestNewExternalTransactionPositiveKindsRequirePositiveMoney(t *testing.T) {
	kinds := []domain.TransactionKind{
		domain.TransactionKindBet,
		domain.TransactionKindWin,
		domain.TransactionKindRefund,
		domain.TransactionKindRollback,
	}

	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			zero := newTransactionMoney(
				t,
				"0.00",
				domain.CurrencyBRL,
			)

			reference := ""

			if kind == domain.TransactionKindRefund ||
				kind == domain.TransactionKindRollback {
				reference = "reference-001"
			}

			_, err := domain.NewExternalTransaction(
				uuid.New(),
				"external-001",
				"provider-001",
				"idempotency-001",
				"hash-001",
				uuid.New(),
				uuid.New(),
				"round-001",
				"game-001",
				kind,
				zero,
				reference,
				transactionTestNow(),
			)

			if !errors.Is(err, domain.ErrInvalidMoney) {
				t.Fatalf(
					"error = %v, want ErrInvalidMoney",
					err,
				)
			}
		})
	}
}

func TestNewExternalTransactionRefundRequiresReference(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	_, err := domain.NewExternalTransaction(
		uuid.New(),
		"external-refund",
		"provider-001",
		"idempotency-refund",
		"hash-refund",
		uuid.New(),
		uuid.New(),
		"round-001",
		"game-001",
		domain.TransactionKindRefund,
		money,
		"",
		transactionTestNow(),
	)

	if !errors.Is(err, domain.ErrInvalidTransaction) {
		t.Fatalf(
			"error = %v, want ErrInvalidTransaction",
			err,
		)
	}
}

func TestNewExternalTransactionRollbackRequiresReference(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	_, err := domain.NewExternalTransaction(
		uuid.New(),
		"external-rollback",
		"provider-001",
		"idempotency-rollback",
		"hash-rollback",
		uuid.New(),
		uuid.New(),
		"round-001",
		"game-001",
		domain.TransactionKindRollback,
		money,
		"",
		transactionTestNow(),
	)

	if !errors.Is(err, domain.ErrInvalidTransaction) {
		t.Fatalf(
			"error = %v, want ErrInvalidTransaction",
			err,
		)
	}
}

func TestNewExternalTransactionAcceptsReferenceForReversal(t *testing.T) {
	money := newTransactionMoney(
		t,
		"25.00",
		domain.CurrencyBRL,
	)

	for _, kind := range []domain.TransactionKind{
		domain.TransactionKindRefund,
		domain.TransactionKindRollback,
	} {
		t.Run(string(kind), func(t *testing.T) {
			transaction := newTestTransaction(
				t,
				kind,
				money,
				"reference-001",
			)

			if transaction.ReferenceExternalID() != "reference-001" {
				t.Fatalf(
					"ReferenceExternalID() = %q, want %q",
					transaction.ReferenceExternalID(),
					"reference-001",
				)
			}
		})
	}
}

func TestMarkPendingReference(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	transaction := newTestTransaction(
		t,
		domain.TransactionKindRefund,
		money,
		"reference-001",
	)

	now := transactionTestNow().Add(time.Minute)

	if err := transaction.MarkPendingReference(now); err != nil {
		t.Fatalf(
			"MarkPendingReference() error = %v",
			err,
		)
	}

	if transaction.Status() != domain.TransactionStatusPendingReference {
		t.Fatalf(
			"Status() = %q, want %q",
			transaction.Status(),
			domain.TransactionStatusPendingReference,
		)
	}

	if !transaction.UpdatedAt().Equal(now) {
		t.Fatalf(
			"UpdatedAt() = %v, want %v",
			transaction.UpdatedAt(),
			now,
		)
	}
}

func TestMarkPendingReferenceRejectsNonPendingState(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	transaction := newTestTransaction(
		t,
		domain.TransactionKindRefund,
		money,
		"reference-001",
	)

	if err := transaction.MarkPendingReference(
		transactionTestNow().Add(time.Minute),
	); err != nil {
		t.Fatalf(
			"MarkPendingReference() error = %v",
			err,
		)
	}

	err := transaction.MarkPendingReference(
		transactionTestNow().Add(2 * time.Minute),
	)

	if !errors.Is(err, domain.ErrInvalidTransactionState) {
		t.Fatalf(
			"MarkPendingReference() error = %v, want ErrInvalidTransactionState",
			err,
		)
	}
}

func TestResolveReference(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	transaction := newTestTransaction(
		t,
		domain.TransactionKindRefund,
		money,
		"reference-001",
	)

	pendingAt := transactionTestNow().Add(time.Minute)

	if err := transaction.MarkPendingReference(pendingAt); err != nil {
		t.Fatalf(
			"MarkPendingReference() error = %v",
			err,
		)
	}

	referenceID := uuid.New()
	resolvedAt := pendingAt.Add(time.Minute)

	if err := transaction.ResolveReference(
		referenceID,
		resolvedAt,
	); err != nil {
		t.Fatalf(
			"ResolveReference() error = %v",
			err,
		)
	}

	if transaction.Status() != domain.TransactionStatusPending {
		t.Fatalf(
			"Status() = %q, want %q",
			transaction.Status(),
			domain.TransactionStatusPending,
		)
	}

	if transaction.ReferenceTransactionID() == nil {
		t.Fatal("ReferenceTransactionID() is nil")
	}

	if *transaction.ReferenceTransactionID() != referenceID {
		t.Fatalf(
			"ReferenceTransactionID() = %s, want %s",
			*transaction.ReferenceTransactionID(),
			referenceID,
		)
	}

	if !transaction.UpdatedAt().Equal(resolvedAt) {
		t.Fatalf(
			"UpdatedAt() = %v, want %v",
			transaction.UpdatedAt(),
			resolvedAt,
		)
	}
}

func TestResolveReferenceRejectsInvalidReferenceID(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	transaction := newTestTransaction(
		t,
		domain.TransactionKindRefund,
		money,
		"reference-001",
	)

	if err := transaction.MarkPendingReference(
		transactionTestNow().Add(time.Minute),
	); err != nil {
		t.Fatalf(
			"MarkPendingReference() error = %v",
			err,
		)
	}

	err := transaction.ResolveReference(
		uuid.Nil,
		transactionTestNow().Add(2*time.Minute),
	)

	if !errors.Is(err, domain.ErrInvalidTransaction) {
		t.Fatalf(
			"ResolveReference() error = %v, want ErrInvalidTransaction",
			err,
		)
	}
}

func TestResolveReferenceRejectsNonPendingReferenceState(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	transaction := newTestTransaction(
		t,
		domain.TransactionKindBet,
		money,
		"",
	)

	err := transaction.ResolveReference(
		uuid.New(),
		transactionTestNow().Add(time.Minute),
	)

	if !errors.Is(err, domain.ErrInvalidTransactionState) {
		t.Fatalf(
			"ResolveReference() error = %v, want ErrInvalidTransactionState",
			err,
		)
	}
}

func TestMarkProcessed(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	transaction := newTestTransaction(
		t,
		domain.TransactionKindBet,
		money,
		"",
	)

	resultBalance := newTransactionMoney(
		t,
		"90.00",
		domain.CurrencyBRL,
	)

	now := transactionTestNow().Add(time.Minute)

	if err := transaction.MarkProcessed(
		resultBalance,
		now,
	); err != nil {
		t.Fatalf(
			"MarkProcessed() error = %v",
			err,
		)
	}

	if transaction.Status() != domain.TransactionStatusProcessed {
		t.Fatalf(
			"Status() = %q, want %q",
			transaction.Status(),
			domain.TransactionStatusProcessed,
		)
	}

	if transaction.ResultBalance() == nil {
		t.Fatal("ResultBalance() is nil")
	}

	if !transaction.ResultBalance().Equal(resultBalance) {
		t.Fatalf(
			"ResultBalance() = %s, want %s",
			transaction.ResultBalance(),
			resultBalance,
		)
	}

	if transaction.FailureCode() != "" {
		t.Fatalf(
			"FailureCode() = %q, want empty",
			transaction.FailureCode(),
		)
	}

	if !transaction.UpdatedAt().Equal(now) {
		t.Fatalf(
			"UpdatedAt() = %v, want %v",
			transaction.UpdatedAt(),
			now,
		)
	}
}

func TestMarkProcessedRejectsCurrencyMismatch(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	transaction := newTestTransaction(
		t,
		domain.TransactionKindBet,
		money,
		"",
	)

	resultBalance := newTransactionMoney(
		t,
		"90.00",
		domain.Currency("USD"),
	)

	err := transaction.MarkProcessed(
		resultBalance,
		transactionTestNow().Add(time.Minute),
	)

	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf(
			"MarkProcessed() error = %v, want ErrCurrencyMismatch",
			err,
		)
	}

	if transaction.Status() != domain.TransactionStatusPending {
		t.Fatalf(
			"Status() = %q, want %q",
			transaction.Status(),
			domain.TransactionStatusPending,
		)
	}

	if transaction.ResultBalance() != nil {
		t.Fatal("ResultBalance() changed after failed operation")
	}
}

func TestMarkRejected(t *testing.T) {
	transaction := newTestTransaction(
		t,
		domain.TransactionKindBet,
		newTransactionMoney(t, "10.00", domain.CurrencyBRL),
		"",
	)

	now := transactionTestNow().Add(time.Minute)

	if err := transaction.MarkRejected(
		"INSUFFICIENT_BALANCE",
		now,
	); err != nil {
		t.Fatalf(
			"MarkRejected() error = %v",
			err,
		)
	}

	if transaction.Status() != domain.TransactionStatusRejected {
		t.Fatalf(
			"Status() = %q, want %q",
			transaction.Status(),
			domain.TransactionStatusRejected,
		)
	}

	if transaction.FailureCode() != "INSUFFICIENT_BALANCE" {
		t.Fatalf(
			"FailureCode() = %q, want %q",
			transaction.FailureCode(),
			"INSUFFICIENT_BALANCE",
		)
	}

	if !transaction.UpdatedAt().Equal(now) {
		t.Fatalf(
			"UpdatedAt() = %v, want %v",
			transaction.UpdatedAt(),
			now,
		)
	}
}

func TestMarkRejectedRequiresFailureCode(t *testing.T) {
	transaction := newTestTransaction(
		t,
		domain.TransactionKindBet,
		newTransactionMoney(t, "10.00", domain.CurrencyBRL),
		"",
	)

	err := transaction.MarkRejected(
		"",
		transactionTestNow().Add(time.Minute),
	)

	if !errors.Is(err, domain.ErrInvalidTransaction) {
		t.Fatalf(
			"MarkRejected() error = %v, want ErrInvalidTransaction",
			err,
		)
	}
}

func TestMarkFailed(t *testing.T) {
	transaction := newTestTransaction(
		t,
		domain.TransactionKindBet,
		newTransactionMoney(t, "10.00", domain.CurrencyBRL),
		"",
	)

	now := transactionTestNow().Add(time.Minute)

	if err := transaction.MarkFailed(
		"PROVIDER_TIMEOUT",
		now,
	); err != nil {
		t.Fatalf(
			"MarkFailed() error = %v",
			err,
		)
	}

	if transaction.Status() != domain.TransactionStatusFailed {
		t.Fatalf(
			"Status() = %q, want %q",
			transaction.Status(),
			domain.TransactionStatusFailed,
		)
	}

	if transaction.FailureCode() != "PROVIDER_TIMEOUT" {
		t.Fatalf(
			"FailureCode() = %q, want %q",
			transaction.FailureCode(),
			"PROVIDER_TIMEOUT",
		)
	}
}

func TestTerminalTransactionsAreImmutable(t *testing.T) {
	tests := []struct {
		name string
		kind domain.TransactionKind
	}{
		{
			name: "processed",
			kind: domain.TransactionKindBet,
		},
		{
			name: "rejected",
			kind: domain.TransactionKindBet,
		},
		{
			name: "failed",
			kind: domain.TransactionKindBet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transaction := newTestTransaction(
				t,
				tt.kind,
				newTransactionMoney(t, "10.00", domain.CurrencyBRL),
				"",
			)

			switch tt.name {
			case "processed":
				if err := transaction.MarkProcessed(
					newTransactionMoney(
						t,
						"90.00",
						domain.CurrencyBRL,
					),
					transactionTestNow().Add(time.Minute),
				); err != nil {
					t.Fatalf(
						"MarkProcessed() error = %v",
						err,
					)
				}

			case "rejected":
				if err := transaction.MarkRejected(
					"REJECTED",
					transactionTestNow().Add(time.Minute),
				); err != nil {
					t.Fatalf(
						"MarkRejected() error = %v",
						err,
					)
				}

			case "failed":
				if err := transaction.MarkFailed(
					"FAILED",
					transactionTestNow().Add(time.Minute),
				); err != nil {
					t.Fatalf(
						"MarkFailed() error = %v",
						err,
					)
				}
			}

			originalStatus := transaction.Status()
			originalFailureCode := transaction.FailureCode()
			originalUpdatedAt := transaction.UpdatedAt()

			if err := transaction.MarkPendingReference(
				transactionTestNow().Add(2 * time.Minute),
			); !errors.Is(err, domain.ErrTerminalTransaction) {
				t.Fatalf(
					"MarkPendingReference() error = %v, want ErrTerminalTransaction",
					err,
				)
			}

			if err := transaction.MarkProcessed(
				newTransactionMoney(
					t,
					"80.00",
					domain.CurrencyBRL,
				),
				transactionTestNow().Add(2*time.Minute),
			); !errors.Is(err, domain.ErrTerminalTransaction) {
				t.Fatalf(
					"MarkProcessed() error = %v, want ErrTerminalTransaction",
					err,
				)
			}

			if err := transaction.MarkRejected(
				"OTHER",
				transactionTestNow().Add(2*time.Minute),
			); !errors.Is(err, domain.ErrTerminalTransaction) {
				t.Fatalf(
					"MarkRejected() error = %v, want ErrTerminalTransaction",
					err,
				)
			}

			if err := transaction.MarkFailed(
				"OTHER",
				transactionTestNow().Add(2*time.Minute),
			); !errors.Is(err, domain.ErrTerminalTransaction) {
				t.Fatalf(
					"MarkFailed() error = %v, want ErrTerminalTransaction",
					err,
				)
			}

			if transaction.Status() != originalStatus {
				t.Fatalf(
					"Status() = %q, want %q",
					transaction.Status(),
					originalStatus,
				)
			}

			if transaction.FailureCode() != originalFailureCode {
				t.Fatalf(
					"FailureCode() = %q, want %q",
					transaction.FailureCode(),
					originalFailureCode,
				)
			}

			if !transaction.UpdatedAt().Equal(originalUpdatedAt) {
				t.Fatalf(
					"UpdatedAt() = %v, want %v",
					transaction.UpdatedAt(),
					originalUpdatedAt,
				)
			}
		})
	}
}

func TestMarkOperationsRejectZeroTimestamp(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	t.Run("pending reference", func(t *testing.T) {
		transaction := newTestTransaction(
			t,
			domain.TransactionKindRefund,
			money,
			"reference-001",
		)

		err := transaction.MarkPendingReference(time.Time{})

		if !errors.Is(err, domain.ErrInvalidTransaction) {
			t.Fatalf(
				"error = %v, want ErrInvalidTransaction",
				err,
			)
		}
	})

	t.Run("processed", func(t *testing.T) {
		transaction := newTestTransaction(
			t,
			domain.TransactionKindBet,
			money,
			"",
		)

		err := transaction.MarkProcessed(
			money,
			time.Time{},
		)

		if !errors.Is(err, domain.ErrInvalidTransaction) {
			t.Fatalf(
				"error = %v, want ErrInvalidTransaction",
				err,
			)
		}
	})

	t.Run("rejected", func(t *testing.T) {
		transaction := newTestTransaction(
			t,
			domain.TransactionKindBet,
			money,
			"",
		)

		err := transaction.MarkRejected(
			"REJECTED",
			time.Time{},
		)

		if !errors.Is(err, domain.ErrInvalidTransaction) {
			t.Fatalf(
				"error = %v, want ErrInvalidTransaction",
				err,
			)
		}
	})

	t.Run("failed", func(t *testing.T) {
		transaction := newTestTransaction(
			t,
			domain.TransactionKindBet,
			money,
			"",
		)

		err := transaction.MarkFailed(
			"FAILED",
			time.Time{},
		)

		if !errors.Is(err, domain.ErrInvalidTransaction) {
			t.Fatalf(
				"error = %v, want ErrInvalidTransaction",
				err,
			)
		}
	})
}

func TestRehydrateTransaction(t *testing.T) {
	transactionID := uuid.New()
	walletID := uuid.New()
	playerID := uuid.New()
	referenceTransactionID := uuid.New()

	money := newTransactionMoney(
		t,
		"25.00",
		domain.CurrencyBRL,
	)

	resultBalance := newTransactionMoney(
		t,
		"75.00",
		domain.CurrencyBRL,
	)

	createdAt := transactionTestNow()
	updatedAt := createdAt.Add(time.Hour)

	transaction, err := domain.RehydrateTransaction(
		transactionID,
		"external-001",
		"provider-001",
		"idempotency-001",
		"hash-001",
		walletID,
		playerID,
		"round-001",
		"game-001",
		domain.TransactionKindRefund,
		money,
		"reference-external-001",
		&referenceTransactionID,
		domain.TransactionStatusProcessed,
		"",
		&resultBalance,
		createdAt,
		updatedAt,
	)
	if err != nil {
		t.Fatalf(
			"RehydrateTransaction() error = %v",
			err,
		)
	}

	if transaction.ID() != transactionID {
		t.Fatalf(
			"ID() = %s, want %s",
			transaction.ID(),
			transactionID,
		)
	}

	if transaction.Status() != domain.TransactionStatusProcessed {
		t.Fatalf(
			"Status() = %q, want %q",
			transaction.Status(),
			domain.TransactionStatusProcessed,
		)
	}

	if transaction.ReferenceTransactionID() == nil {
		t.Fatal("ReferenceTransactionID() is nil")
	}

	if *transaction.ReferenceTransactionID() != referenceTransactionID {
		t.Fatalf(
			"ReferenceTransactionID() = %s, want %s",
			*transaction.ReferenceTransactionID(),
			referenceTransactionID,
		)
	}

	if transaction.ResultBalance() == nil {
		t.Fatal("ResultBalance() is nil")
	}

	if !transaction.ResultBalance().Equal(resultBalance) {
		t.Fatalf(
			"ResultBalance() = %s, want %s",
			transaction.ResultBalance(),
			resultBalance,
		)
	}

	if !transaction.CreatedAt().Equal(createdAt) {
		t.Fatalf(
			"CreatedAt() = %v, want %v",
			transaction.CreatedAt(),
			createdAt,
		)
	}

	if !transaction.UpdatedAt().Equal(updatedAt) {
		t.Fatalf(
			"UpdatedAt() = %v, want %v",
			transaction.UpdatedAt(),
			updatedAt,
		)
	}
}

func TestRehydrateTransactionRejectsInvalidStatus(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	_, err := domain.RehydrateTransaction(
		uuid.New(),
		"external-001",
		"provider-001",
		"idempotency-001",
		"hash-001",
		uuid.New(),
		uuid.New(),
		"round-001",
		"game-001",
		domain.TransactionKindBet,
		money,
		"",
		nil,
		domain.TransactionStatus("INVALID"),
		"",
		nil,
		transactionTestNow(),
		transactionTestNow(),
	)

	if !errors.Is(err, domain.ErrInvalidTransactionState) {
		t.Fatalf(
			"RehydrateTransaction() error = %v, want ErrInvalidTransactionState",
			err,
		)
	}
}

func TestRehydrateTransactionRejectsOpening(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	_, err := domain.RehydrateTransaction(
		uuid.New(),
		"external-001",
		"provider-001",
		"idempotency-001",
		"hash-001",
		uuid.New(),
		uuid.New(),
		"round-001",
		"game-001",
		domain.TransactionKindOpening,
		money,
		"",
		nil,
		domain.TransactionStatusPending,
		"",
		nil,
		transactionTestNow(),
		transactionTestNow(),
	)

	if !errors.Is(err, domain.ErrInvalidOpening) {
		t.Fatalf(
			"RehydrateTransaction() error = %v, want ErrInvalidOpening",
			err,
		)
	}
}

func TestRehydrateTransactionRejectsInvalidIdentity(t *testing.T) {
	money := newTransactionMoney(
		t,
		"10.00",
		domain.CurrencyBRL,
	)

	tests := []struct {
		name     string
		id       uuid.UUID
		walletID uuid.UUID
		playerID uuid.UUID
	}{
		{
			name:     "nil transaction id",
			id:       uuid.Nil,
			walletID: uuid.New(),
			playerID: uuid.New(),
		},
		{
			name:     "nil wallet id",
			id:       uuid.New(),
			walletID: uuid.Nil,
			playerID: uuid.New(),
		},
		{
			name:     "nil player id",
			id:       uuid.New(),
			walletID: uuid.New(),
			playerID: uuid.Nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.RehydrateTransaction(
				tt.id,
				"external-001",
				"provider-001",
				"idempotency-001",
				"hash-001",
				tt.walletID,
				tt.playerID,
				"round-001",
				"game-001",
				domain.TransactionKindBet,
				money,
				"",
				nil,
				domain.TransactionStatusPending,
				"",
				nil,
				transactionTestNow(),
				transactionTestNow(),
			)

			if !errors.Is(err, domain.ErrInvalidTransaction) {
				t.Fatalf(
					"error = %v, want ErrInvalidTransaction",
					err,
				)
			}
		})
	}
}
