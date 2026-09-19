package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

func mustMoney(t *testing.T, amount string, currency domain.Currency) domain.Money {
	t.Helper()

	money, err := domain.NewMoney(amount, currency)
	if err != nil {
		t.Fatalf("NewMoney(%q, %q) error = %v", amount, currency, err)
	}

	return money
}

func TestNewLedgerEntryDebit(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	money := mustMoney(t, "10.00", domain.CurrencyBRL)
	balanceBefore := mustMoney(t, "100.00", domain.CurrencyBRL)
	balanceAfter := mustMoney(t, "90.00", domain.CurrencyBRL)

	id := uuid.New()
	walletID := uuid.New()
	transactionID := uuid.New()

	entry, err := domain.NewLedgerEntry(
		id,
		walletID,
		transactionID,
		domain.LedgerDirectionDebit,
		money,
		balanceBefore,
		balanceAfter,
		now,
	)
	if err != nil {
		t.Fatalf("NewLedgerEntry() error = %v", err)
	}

	if entry.ID() != id {
		t.Fatalf("ID() = %v, want %v", entry.ID(), id)
	}

	if entry.WalletID() != walletID {
		t.Fatalf("WalletID() = %v, want %v", entry.WalletID(), walletID)
	}

	if entry.TransactionID() != transactionID {
		t.Fatalf(
			"TransactionID() = %v, want %v",
			entry.TransactionID(),
			transactionID,
		)
	}

	if entry.Direction() != domain.LedgerDirectionDebit {
		t.Fatalf(
			"Direction() = %v, want %v",
			entry.Direction(),
			domain.LedgerDirectionDebit,
		)
	}

	if !entry.Money().Equal(money) {
		t.Fatalf("Money() = %v, want %v", entry.Money(), money)
	}

	if !entry.BalanceBefore().Equal(balanceBefore) {
		t.Fatalf(
			"BalanceBefore() = %v, want %v",
			entry.BalanceBefore(),
			balanceBefore,
		)
	}

	if !entry.BalanceAfter().Equal(balanceAfter) {
		t.Fatalf(
			"BalanceAfter() = %v, want %v",
			entry.BalanceAfter(),
			balanceAfter,
		)
	}

	if !entry.CreatedAt().Equal(now) {
		t.Fatalf(
			"CreatedAt() = %v, want %v",
			entry.CreatedAt(),
			now,
		)
	}
}

func TestNewLedgerEntryCredit(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	money := mustMoney(t, "25.50", domain.CurrencyBRL)
	balanceBefore := mustMoney(t, "100.00", domain.CurrencyBRL)
	balanceAfter := mustMoney(t, "125.50", domain.CurrencyBRL)

	entry, err := domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirectionCredit,
		money,
		balanceBefore,
		balanceAfter,
		now,
	)
	if err != nil {
		t.Fatalf("NewLedgerEntry() error = %v", err)
	}

	if entry.Direction() != domain.LedgerDirectionCredit {
		t.Fatalf(
			"Direction() = %v, want %v",
			entry.Direction(),
			domain.LedgerDirectionCredit,
		)
	}
}

func TestNewLedgerEntryRejectsInvalidIdentifiers(t *testing.T) {
	now := time.Now().UTC()

	money := mustMoney(t, "10.00", domain.CurrencyBRL)
	balanceBefore := mustMoney(t, "100.00", domain.CurrencyBRL)
	balanceAfter := mustMoney(t, "90.00", domain.CurrencyBRL)

	tests := []struct {
		name          string
		id            uuid.UUID
		walletID      uuid.UUID
		transactionID uuid.UUID
	}{
		{
			name:          "nil entry id",
			id:            uuid.Nil,
			walletID:      uuid.New(),
			transactionID: uuid.New(),
		},
		{
			name:          "nil wallet id",
			id:            uuid.New(),
			walletID:      uuid.Nil,
			transactionID: uuid.New(),
		},
		{
			name:          "nil transaction id",
			id:            uuid.New(),
			walletID:      uuid.New(),
			transactionID: uuid.Nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewLedgerEntry(
				tt.id,
				tt.walletID,
				tt.transactionID,
				domain.LedgerDirectionDebit,
				money,
				balanceBefore,
				balanceAfter,
				now,
			)

			if !errors.Is(err, domain.ErrInvalidLedgerEntry) {
				t.Fatalf(
					"error = %v, want ErrInvalidLedgerEntry",
					err,
				)
			}
		})
	}
}

func TestNewLedgerEntryRejectsZeroTimestamp(t *testing.T) {
	money := mustMoney(t, "10.00", domain.CurrencyBRL)
	balanceBefore := mustMoney(t, "100.00", domain.CurrencyBRL)
	balanceAfter := mustMoney(t, "90.00", domain.CurrencyBRL)

	_, err := domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirectionDebit,
		money,
		balanceBefore,
		balanceAfter,
		time.Time{},
	)

	if !errors.Is(err, domain.ErrInvalidLedgerEntry) {
		t.Fatalf(
			"error = %v, want ErrInvalidLedgerEntry",
			err,
		)
	}
}

func TestNewLedgerEntryRejectsZeroMoney(t *testing.T) {
	now := time.Now().UTC()

	money := mustMoney(t, "0.00", domain.CurrencyBRL)
	balanceBefore := mustMoney(t, "100.00", domain.CurrencyBRL)
	balanceAfter := mustMoney(t, "100.00", domain.CurrencyBRL)

	_, err := domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirectionDebit,
		money,
		balanceBefore,
		balanceAfter,
		now,
	)

	if !errors.Is(err, domain.ErrInvalidLedgerEntry) {
		t.Fatalf(
			"error = %v, want ErrInvalidLedgerEntry",
			err,
		)
	}
}

func TestNewLedgerEntryRejectsNegativeMoney(t *testing.T) {
	now := time.Now().UTC()

	money, err := domain.MoneyFromMinorUnits(
		-1000,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("MoneyFromMinorUnits() error = %v", err)
	}

	balanceBefore := mustMoney(t, "100.00", domain.CurrencyBRL)
	balanceAfter := mustMoney(t, "110.00", domain.CurrencyBRL)

	_, err = domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirectionDebit,
		money,
		balanceBefore,
		balanceAfter,
		now,
	)

	if !errors.Is(err, domain.ErrInvalidLedgerEntry) {
		t.Fatalf(
			"error = %v, want ErrInvalidLedgerEntry",
			err,
		)
	}
}

func TestNewLedgerEntryRejectsCurrencyMismatch(t *testing.T) {
	now := time.Now().UTC()

	brlMoney := mustMoney(t, "10.00", domain.CurrencyBRL)
	brlBefore := mustMoney(t, "100.00", domain.CurrencyBRL)
	brlAfter := mustMoney(t, "90.00", domain.CurrencyBRL)

	usdMoney := mustMoney(t, "10.00", domain.Currency("USD"))

	_, err := domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirectionDebit,
		usdMoney,
		brlBefore,
		brlAfter,
		now,
	)

	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf(
			"error = %v, want ErrCurrencyMismatch",
			err,
		)
	}

	_, err = domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirectionDebit,
		brlMoney,
		brlBefore,
		usdMoney,
		now,
	)

	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf(
			"error = %v, want ErrCurrencyMismatch",
			err,
		)
	}
}

func TestNewLedgerEntryRejectsNegativeBalances(t *testing.T) {
	now := time.Now().UTC()

	money := mustMoney(t, "10.00", domain.CurrencyBRL)
	validBalance := mustMoney(t, "10.00", domain.CurrencyBRL)

	negativeBalance, err := domain.MoneyFromMinorUnits(
		-1,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("MoneyFromMinorUnits() error = %v", err)
	}

	_, err = domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirectionDebit,
		money,
		negativeBalance,
		validBalance,
		now,
	)

	if !errors.Is(err, domain.ErrInvalidLedgerEntry) {
		t.Fatalf(
			"error = %v, want ErrInvalidLedgerEntry",
			err,
		)
	}

	_, err = domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirectionCredit,
		money,
		validBalance,
		negativeBalance,
		now,
	)

	if !errors.Is(err, domain.ErrInvalidLedgerEntry) {
		t.Fatalf(
			"error = %v, want ErrInvalidLedgerEntry",
			err,
		)
	}
}

func TestNewLedgerEntryRejectsIncorrectDebitBalance(t *testing.T) {
	now := time.Now().UTC()

	money := mustMoney(t, "10.00", domain.CurrencyBRL)
	balanceBefore := mustMoney(t, "100.00", domain.CurrencyBRL)
	wrongBalanceAfter := mustMoney(t, "95.00", domain.CurrencyBRL)

	_, err := domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirectionDebit,
		money,
		balanceBefore,
		wrongBalanceAfter,
		now,
	)

	if !errors.Is(err, domain.ErrInvalidLedgerEntry) {
		t.Fatalf(
			"error = %v, want ErrInvalidLedgerEntry",
			err,
		)
	}
}

func TestNewLedgerEntryRejectsIncorrectCreditBalance(t *testing.T) {
	now := time.Now().UTC()

	money := mustMoney(t, "10.00", domain.CurrencyBRL)
	balanceBefore := mustMoney(t, "100.00", domain.CurrencyBRL)
	wrongBalanceAfter := mustMoney(t, "105.00", domain.CurrencyBRL)

	_, err := domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirectionCredit,
		money,
		balanceBefore,
		wrongBalanceAfter,
		now,
	)

	if !errors.Is(err, domain.ErrInvalidLedgerEntry) {
		t.Fatalf(
			"error = %v, want ErrInvalidLedgerEntry",
			err,
		)
	}
}

func TestNewLedgerEntryRejectsInvalidDirection(t *testing.T) {
	now := time.Now().UTC()

	money := mustMoney(t, "10.00", domain.CurrencyBRL)
	balanceBefore := mustMoney(t, "100.00", domain.CurrencyBRL)
	balanceAfter := mustMoney(t, "90.00", domain.CurrencyBRL)

	_, err := domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirection("INVALID"),
		money,
		balanceBefore,
		balanceAfter,
		now,
	)

	if !errors.Is(err, domain.ErrInvalidLedgerEntry) {
		t.Fatalf(
			"error = %v, want ErrInvalidLedgerEntry",
			err,
		)
	}
}

func TestNewLedgerEntryNormalizesCreatedAtToUTC(t *testing.T) {
	location := time.FixedZone("TEST", -3*60*60)

	localTime := time.Date(
		2026,
		9,
		18,
		12,
		0,
		0,
		0,
		location,
	)

	money := mustMoney(t, "10.00", domain.CurrencyBRL)
	balanceBefore := mustMoney(t, "100.00", domain.CurrencyBRL)
	balanceAfter := mustMoney(t, "90.00", domain.CurrencyBRL)

	entry, err := domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirectionDebit,
		money,
		balanceBefore,
		balanceAfter,
		localTime,
	)
	if err != nil {
		t.Fatalf("NewLedgerEntry() error = %v", err)
	}

	if entry.CreatedAt().Location() != time.UTC {
		t.Fatalf(
			"CreatedAt() location = %v, want UTC",
			entry.CreatedAt().Location(),
		)
	}

	if !entry.CreatedAt().Equal(localTime) {
		t.Fatalf(
			"CreatedAt() = %v, want instant %v",
			entry.CreatedAt(),
			localTime,
		)
	}
}

func TestNewLedgerEntryCreatesImmutableValue(t *testing.T) {
	now := time.Now().UTC()

	money := mustMoney(t, "10.00", domain.CurrencyBRL)
	balanceBefore := mustMoney(t, "100.00", domain.CurrencyBRL)
	balanceAfter := mustMoney(t, "90.00", domain.CurrencyBRL)

	entry, err := domain.NewLedgerEntry(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		domain.LedgerDirectionDebit,
		money,
		balanceBefore,
		balanceAfter,
		now,
	)
	if err != nil {
		t.Fatalf("NewLedgerEntry() error = %v", err)
	}

	copy := entry

	if copy.ID() != entry.ID() {
		t.Fatal("ledger entry ID changed")
	}

	if copy.WalletID() != entry.WalletID() {
		t.Fatal("ledger entry wallet ID changed")
	}

	if copy.TransactionID() != entry.TransactionID() {
		t.Fatal("ledger entry transaction ID changed")
	}

	if copy.Direction() != entry.Direction() {
		t.Fatal("ledger entry direction changed")
	}

	if !copy.Money().Equal(entry.Money()) {
		t.Fatal("ledger entry money changed")
	}

	if !copy.BalanceBefore().Equal(entry.BalanceBefore()) {
		t.Fatal("ledger entry balance before changed")
	}

	if !copy.BalanceAfter().Equal(entry.BalanceAfter()) {
		t.Fatal("ledger entry balance after changed")
	}

	if !copy.CreatedAt().Equal(entry.CreatedAt()) {
		t.Fatal("ledger entry created timestamp changed")
	}
}
