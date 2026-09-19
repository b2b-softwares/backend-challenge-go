package domain_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

func testNow() time.Time {
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

func newTestWallet(t *testing.T) domain.Wallet {
	t.Helper()

	walletID := uuid.New()
	playerID := uuid.New()

	balance, err := domain.MoneyFromMinorUnits(
		0,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("MoneyFromMinorUnits() error = %v", err)
	}

	wallet, err := domain.NewWallet(
		walletID,
		playerID,
		balance,
		testNow(),
	)
	if err != nil {
		t.Fatalf("NewWallet() error = %v", err)
	}

	return wallet
}

func TestNewWallet(t *testing.T) {
	walletID := uuid.New()
	playerID := uuid.New()

	initialBalance, err := domain.MoneyFromMinorUnits(
		0,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("MoneyFromMinorUnits() error = %v", err)
	}

	now := testNow()

	wallet, err := domain.NewWallet(
		walletID,
		playerID,
		initialBalance,
		now,
	)
	if err != nil {
		t.Fatalf("NewWallet() error = %v", err)
	}

	if wallet.ID() != walletID {
		t.Fatalf(
			"ID() = %s, want %s",
			wallet.ID(),
			walletID,
		)
	}

	if wallet.PlayerID() != playerID {
		t.Fatalf(
			"PlayerID() = %s, want %s",
			wallet.PlayerID(),
			playerID,
		)
	}

	if wallet.Currency() != domain.CurrencyBRL {
		t.Fatalf(
			"Currency() = %s, want %s",
			wallet.Currency(),
			domain.CurrencyBRL,
		)
	}

	if wallet.Balance().Amount() != 0 {
		t.Fatalf(
			"Balance().Amount() = %d, want 0",
			wallet.Balance().Amount(),
		)
	}

	if wallet.Balance().Currency() != domain.CurrencyBRL {
		t.Fatalf(
			"Balance().Currency() = %s, want %s",
			wallet.Balance().Currency(),
			domain.CurrencyBRL,
		)
	}

	if wallet.Version() != 1 {
		t.Fatalf(
			"Version() = %d, want 1",
			wallet.Version(),
		)
	}

	if !wallet.CreatedAt().Equal(now) {
		t.Fatalf(
			"CreatedAt() = %v, want %v",
			wallet.CreatedAt(),
			now,
		)
	}

	if !wallet.UpdatedAt().Equal(now) {
		t.Fatalf(
			"UpdatedAt() = %v, want %v",
			wallet.UpdatedAt(),
			now,
		)
	}
}

func TestNewWalletGeneratesUniqueIDs(t *testing.T) {
	playerID := uuid.New()

	firstBalance, err := domain.ZeroMoney(domain.CurrencyBRL)
	if err != nil {
		t.Fatalf("ZeroMoney() error = %v", err)
	}

	first, err := domain.NewWallet(
		uuid.New(),
		playerID,
		firstBalance,
		testNow(),
	)
	if err != nil {
		t.Fatalf("first NewWallet() error = %v", err)
	}

	secondBalance, err := domain.ZeroMoney(domain.CurrencyBRL)
	if err != nil {
		t.Fatalf("ZeroMoney() error = %v", err)
	}

	second, err := domain.NewWallet(
		uuid.New(),
		playerID,
		secondBalance,
		testNow(),
	)
	if err != nil {
		t.Fatalf("second NewWallet() error = %v", err)
	}

	if first.ID() == second.ID() {
		t.Fatal("NewWallet() generated duplicate wallet IDs")
	}
}

func TestNewWalletRejectsNilID(t *testing.T) {
	playerID := uuid.New()

	balance, err := domain.ZeroMoney(domain.CurrencyBRL)
	if err != nil {
		t.Fatalf("ZeroMoney() error = %v", err)
	}

	_, err = domain.NewWallet(
		uuid.Nil,
		playerID,
		balance,
		testNow(),
	)

	if !errors.Is(err, domain.ErrInvalidWallet) {
		t.Fatalf(
			"NewWallet() error = %v, want ErrInvalidWallet",
			err,
		)
	}
}

func TestNewWalletInvalidPlayerID(t *testing.T) {
	balance, err := domain.ZeroMoney(domain.CurrencyBRL)
	if err != nil {
		t.Fatalf("ZeroMoney() error = %v", err)
	}

	_, err = domain.NewWallet(
		uuid.New(),
		uuid.Nil,
		balance,
		testNow(),
	)

	if !errors.Is(err, domain.ErrInvalidWallet) {
		t.Fatalf(
			"NewWallet() error = %v, want ErrInvalidWallet",
			err,
		)
	}
}

func TestNewWalletRejectsNegativeInitialBalance(t *testing.T) {
	balance, err := domain.MoneyFromMinorUnits(
		-1,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("MoneyFromMinorUnits() error = %v", err)
	}

	_, err = domain.NewWallet(
		uuid.New(),
		uuid.New(),
		balance,
		testNow(),
	)

	if !errors.Is(err, domain.ErrInvalidWallet) {
		t.Fatalf(
			"NewWallet() error = %v, want ErrInvalidWallet",
			err,
		)
	}
}

func TestNewWalletRejectsZeroTimestamp(t *testing.T) {
	balance, err := domain.ZeroMoney(domain.CurrencyBRL)
	if err != nil {
		t.Fatalf("ZeroMoney() error = %v", err)
	}

	_, err = domain.NewWallet(
		uuid.New(),
		uuid.New(),
		balance,
		time.Time{},
	)

	if !errors.Is(err, domain.ErrInvalidWallet) {
		t.Fatalf(
			"NewWallet() error = %v, want ErrInvalidWallet",
			err,
		)
	}
}

func TestNewWalletInitialBalanceIsZero(t *testing.T) {
	wallet := newTestWallet(t)

	if !wallet.Balance().IsZero() {
		t.Fatal("Balance().IsZero() = false, want true")
	}

	if wallet.Version() != 1 {
		t.Fatalf(
			"Version() = %d, want 1",
			wallet.Version(),
		)
	}
}

func TestRehydrateWallet(t *testing.T) {
	walletID := uuid.New()
	playerID := uuid.New()

	balance, err := domain.MoneyFromMinorUnits(
		12345,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("MoneyFromMinorUnits() error = %v", err)
	}

	createdAt := testNow()
	updatedAt := createdAt.Add(time.Hour)

	wallet, err := domain.RehydrateWallet(
		walletID,
		playerID,
		domain.CurrencyBRL,
		balance,
		7,
		createdAt,
		updatedAt,
	)
	if err != nil {
		t.Fatalf("RehydrateWallet() error = %v", err)
	}

	if wallet.ID() != walletID {
		t.Fatalf(
			"ID() = %s, want %s",
			wallet.ID(),
			walletID,
		)
	}

	if wallet.PlayerID() != playerID {
		t.Fatalf(
			"PlayerID() = %s, want %s",
			wallet.PlayerID(),
			playerID,
		)
	}

	if wallet.Currency() != domain.CurrencyBRL {
		t.Fatalf(
			"Currency() = %s, want %s",
			wallet.Currency(),
			domain.CurrencyBRL,
		)
	}

	if wallet.Balance().Amount() != 12345 {
		t.Fatalf(
			"Balance().Amount() = %d, want 12345",
			wallet.Balance().Amount(),
		)
	}

	if wallet.Version() != 7 {
		t.Fatalf(
			"Version() = %d, want 7",
			wallet.Version(),
		)
	}

	if !wallet.CreatedAt().Equal(createdAt) {
		t.Fatalf(
			"CreatedAt() = %v, want %v",
			wallet.CreatedAt(),
			createdAt,
		)
	}

	if !wallet.UpdatedAt().Equal(updatedAt) {
		t.Fatalf(
			"UpdatedAt() = %v, want %v",
			wallet.UpdatedAt(),
			updatedAt,
		)
	}
}

func TestRehydrateWalletRejectsNilID(t *testing.T) {
	playerID := uuid.New()

	balance, err := domain.ZeroMoney(domain.CurrencyBRL)
	if err != nil {
		t.Fatalf("ZeroMoney() error = %v", err)
	}

	_, err = domain.RehydrateWallet(
		uuid.Nil,
		playerID,
		domain.CurrencyBRL,
		balance,
		1,
		testNow(),
		testNow(),
	)

	if !errors.Is(err, domain.ErrInvalidWallet) {
		t.Fatalf(
			"RehydrateWallet() error = %v, want ErrInvalidWallet",
			err,
		)
	}
}

func TestRehydrateWalletRejectsNilPlayerID(t *testing.T) {
	walletID := uuid.New()

	balance, err := domain.ZeroMoney(domain.CurrencyBRL)
	if err != nil {
		t.Fatalf("ZeroMoney() error = %v", err)
	}

	_, err = domain.RehydrateWallet(
		walletID,
		uuid.Nil,
		domain.CurrencyBRL,
		balance,
		1,
		testNow(),
		testNow(),
	)

	if !errors.Is(err, domain.ErrInvalidWallet) {
		t.Fatalf(
			"RehydrateWallet() error = %v, want ErrInvalidWallet",
			err,
		)
	}
}

func TestRehydrateWalletRejectsZeroVersion(t *testing.T) {
	balance, err := domain.ZeroMoney(domain.CurrencyBRL)
	if err != nil {
		t.Fatalf("ZeroMoney() error = %v", err)
	}

	_, err = domain.RehydrateWallet(
		uuid.New(),
		uuid.New(),
		domain.CurrencyBRL,
		balance,
		0,
		testNow(),
		testNow(),
	)

	if !errors.Is(err, domain.ErrInvalidWallet) {
		t.Fatalf(
			"RehydrateWallet() error = %v, want ErrInvalidWallet",
			err,
		)
	}
}

func TestRehydrateWalletRejectsNegativeBalance(t *testing.T) {
	balance, err := domain.MoneyFromMinorUnits(
		-1,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("MoneyFromMinorUnits() error = %v", err)
	}

	_, err = domain.RehydrateWallet(
		uuid.New(),
		uuid.New(),
		domain.CurrencyBRL,
		balance,
		1,
		testNow(),
		testNow(),
	)

	if !errors.Is(err, domain.ErrInvalidWallet) {
		t.Fatalf(
			"RehydrateWallet() error = %v, want ErrInvalidWallet",
			err,
		)
	}
}

func TestRehydrateWalletRejectsCurrencyMismatch(t *testing.T) {
	balance, err := domain.MoneyFromMinorUnits(
		1000,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("MoneyFromMinorUnits() error = %v", err)
	}

	_, err = domain.RehydrateWallet(
		uuid.New(),
		uuid.New(),
		domain.Currency("USD"),
		balance,
		1,
		testNow(),
		testNow(),
	)

	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf(
			"RehydrateWallet() error = %v, want ErrCurrencyMismatch",
			err,
		)
	}
}

func TestRehydrateWalletRejectsZeroCreatedAt(t *testing.T) {
	balance, err := domain.ZeroMoney(domain.CurrencyBRL)
	if err != nil {
		t.Fatalf("ZeroMoney() error = %v", err)
	}

	_, err = domain.RehydrateWallet(
		uuid.New(),
		uuid.New(),
		domain.CurrencyBRL,
		balance,
		1,
		time.Time{},
		testNow(),
	)

	if !errors.Is(err, domain.ErrInvalidWallet) {
		t.Fatalf(
			"RehydrateWallet() error = %v, want ErrInvalidWallet",
			err,
		)
	}
}

func TestRehydrateWalletRejectsZeroUpdatedAt(t *testing.T) {
	balance, err := domain.ZeroMoney(domain.CurrencyBRL)
	if err != nil {
		t.Fatalf("ZeroMoney() error = %v", err)
	}

	_, err = domain.RehydrateWallet(
		uuid.New(),
		uuid.New(),
		domain.CurrencyBRL,
		balance,
		1,
		testNow(),
		time.Time{},
	)

	if !errors.Is(err, domain.ErrInvalidWallet) {
		t.Fatalf(
			"RehydrateWallet() error = %v, want ErrInvalidWallet",
			err,
		)
	}
}

func TestWalletCredit(t *testing.T) {
	wallet := newTestWallet(t)

	amount, err := domain.NewMoney(
		"100.00",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	err = wallet.Credit(amount, testNow().Add(time.Minute))
	if err != nil {
		t.Fatalf("Credit() error = %v", err)
	}

	if wallet.Balance().Amount() != 10000 {
		t.Fatalf(
			"Balance().Amount() = %d, want 10000",
			wallet.Balance().Amount(),
		)
	}

	if wallet.Version() != 2 {
		t.Fatalf(
			"Version() = %d, want 2",
			wallet.Version(),
		)
	}
}

func TestWalletDebit(t *testing.T) {
	wallet := newTestWallet(t)

	initial, err := domain.NewMoney(
		"100.00",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	if err := wallet.Credit(initial, testNow()); err != nil {
		t.Fatalf("Credit() error = %v", err)
	}

	debit, err := domain.NewMoney(
		"30.00",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	if err := wallet.Debit(debit, testNow().Add(time.Minute)); err != nil {
		t.Fatalf("Debit() error = %v", err)
	}

	if wallet.Balance().Amount() != 7000 {
		t.Fatalf(
			"Balance().Amount() = %d, want 7000",
			wallet.Balance().Amount(),
		)
	}

	if wallet.Version() != 3 {
		t.Fatalf(
			"Version() = %d, want 3",
			wallet.Version(),
		)
	}
}

func TestWalletDebitRejectsInsufficientBalance(t *testing.T) {
	wallet := newTestWallet(t)

	initial, err := domain.NewMoney(
		"100.00",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	if err := wallet.Credit(initial, testNow()); err != nil {
		t.Fatalf("Credit() error = %v", err)
	}

	debit, err := domain.NewMoney(
		"100.01",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	err = wallet.Debit(debit, testNow().Add(time.Minute))
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf(
			"Debit() error = %v, want ErrInsufficientBalance",
			err,
		)
	}

	if wallet.Balance().Amount() != 10000 {
		t.Fatalf(
			"Balance().Amount() = %d, want 10000",
			wallet.Balance().Amount(),
		)
	}

	if wallet.Version() != 2 {
		t.Fatalf(
			"Version() = %d, want 2",
			wallet.Version(),
		)
	}
}

func TestWalletDebitCanReachZero(t *testing.T) {
	wallet := newTestWallet(t)

	initial, err := domain.NewMoney(
		"100.00",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	if err := wallet.Credit(initial, testNow()); err != nil {
		t.Fatalf("Credit() error = %v", err)
	}

	if err := wallet.Debit(initial, testNow().Add(time.Minute)); err != nil {
		t.Fatalf("Debit() error = %v", err)
	}

	if wallet.Balance().Amount() != 0 {
		t.Fatalf(
			"Balance().Amount() = %d, want 0",
			wallet.Balance().Amount(),
		)
	}
}

func TestWalletCreditRejectsZero(t *testing.T) {
	wallet := newTestWallet(t)

	zero, err := domain.ZeroMoney(domain.CurrencyBRL)
	if err != nil {
		t.Fatalf("ZeroMoney() error = %v", err)
	}

	initialVersion := wallet.Version()
	initialBalance := wallet.Balance().Amount()

	err = wallet.Credit(zero, testNow().Add(time.Minute))
	if !errors.Is(err, domain.ErrInvalidMoney) {
		t.Fatalf(
			"Credit() error = %v, want ErrInvalidMoney",
			err,
		)
	}

	if wallet.Balance().Amount() != initialBalance {
		t.Fatalf(
			"Balance().Amount() = %d, want %d",
			wallet.Balance().Amount(),
			initialBalance,
		)
	}

	if wallet.Version() != initialVersion {
		t.Fatalf(
			"Version() = %d, want %d",
			wallet.Version(),
			initialVersion,
		)
	}
}

func TestWalletDebitRejectsZero(t *testing.T) {
	wallet := newTestWallet(t)

	zero, err := domain.ZeroMoney(domain.CurrencyBRL)
	if err != nil {
		t.Fatalf("ZeroMoney() error = %v", err)
	}

	initialVersion := wallet.Version()
	initialBalance := wallet.Balance().Amount()

	err = wallet.Debit(zero, testNow().Add(time.Minute))
	if !errors.Is(err, domain.ErrInvalidMoney) {
		t.Fatalf(
			"Debit() error = %v, want ErrInvalidMoney",
			err,
		)
	}

	if wallet.Balance().Amount() != initialBalance {
		t.Fatalf(
			"Balance().Amount() = %d, want %d",
			wallet.Balance().Amount(),
			initialBalance,
		)
	}

	if wallet.Version() != initialVersion {
		t.Fatalf(
			"Version() = %d, want %d",
			wallet.Version(),
			initialVersion,
		)
	}
}

func TestWalletCreditRejectsCurrencyMismatch(t *testing.T) {
	wallet := newTestWallet(t)

	amount, err := domain.NewMoney(
		"10.00",
		domain.Currency("USD"),
	)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	err = wallet.Credit(amount, testNow().Add(time.Minute))
	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf(
			"Credit() error = %v, want ErrCurrencyMismatch",
			err,
		)
	}

	if wallet.Balance().Amount() != 0 {
		t.Fatalf(
			"Balance().Amount() = %d, want 0",
			wallet.Balance().Amount(),
		)
	}

	if wallet.Version() != 1 {
		t.Fatalf(
			"Version() = %d, want 1",
			wallet.Version(),
		)
	}
}

func TestWalletDebitRejectsCurrencyMismatch(t *testing.T) {
	wallet := newTestWallet(t)

	amount, err := domain.NewMoney(
		"10.00",
		domain.Currency("USD"),
	)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	err = wallet.Debit(amount, testNow().Add(time.Minute))
	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf(
			"Debit() error = %v, want ErrCurrencyMismatch",
			err,
		)
	}

	if wallet.Balance().Amount() != 0 {
		t.Fatalf(
			"Balance().Amount() = %d, want 0",
			wallet.Balance().Amount(),
		)
	}

	if wallet.Version() != 1 {
		t.Fatalf(
			"Version() = %d, want 1",
			wallet.Version(),
		)
	}
}

func TestWalletCreditOverflow(t *testing.T) {
	playerID := uuid.New()

	initialBalance, err := domain.MoneyFromMinorUnits(
		math.MaxInt64,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("MoneyFromMinorUnits() error = %v", err)
	}

	wallet, err := domain.RehydrateWallet(
		uuid.New(),
		playerID,
		domain.CurrencyBRL,
		initialBalance,
		1,
		testNow(),
		testNow(),
	)
	if err != nil {
		t.Fatalf("RehydrateWallet() error = %v", err)
	}

	oneCent, err := domain.MoneyFromMinorUnits(
		1,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("MoneyFromMinorUnits() error = %v", err)
	}

	err = wallet.Credit(oneCent, testNow().Add(time.Minute))
	if !errors.Is(err, domain.ErrInvalidMoney) {
		t.Fatalf(
			"Credit() error = %v, want ErrInvalidMoney",
			err,
		)
	}

	if wallet.Balance().Amount() != math.MaxInt64 {
		t.Fatalf(
			"Balance().Amount() = %d, want %d",
			wallet.Balance().Amount(),
			math.MaxInt64,
		)
	}

	if wallet.Version() != 1 {
		t.Fatalf(
			"Version() = %d, want 1",
			wallet.Version(),
		)
	}
}

func TestWalletStateIsNotChangedAfterFailedOperations(t *testing.T) {
	wallet := newTestWallet(t)

	initial, err := domain.NewMoney(
		"50.00",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	if err := wallet.Credit(initial, testNow()); err != nil {
		t.Fatalf("Credit() error = %v", err)
	}

	versionBefore := wallet.Version()
	balanceBefore := wallet.Balance().Amount()

	tooMuch, err := domain.NewMoney(
		"50.01",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	err = wallet.Debit(
		tooMuch,
		testNow().Add(time.Minute),
	)
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf(
			"Debit() error = %v, want ErrInsufficientBalance",
			err,
		)
	}

	if wallet.Balance().Amount() != balanceBefore {
		t.Fatalf(
			"Balance().Amount() = %d, want %d",
			wallet.Balance().Amount(),
			balanceBefore,
		)
	}

	if wallet.Version() != versionBefore {
		t.Fatalf(
			"Version() = %d, want %d",
			wallet.Version(),
			versionBefore,
		)
	}
}

func TestWalletMultipleBalanceChangesIncrementVersion(t *testing.T) {
	wallet := newTestWallet(t)

	if wallet.Version() != 1 {
		t.Fatalf(
			"initial Version() = %d, want 1",
			wallet.Version(),
		)
	}

	amount100, err := domain.NewMoney(
		"100.00",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	if err := wallet.Credit(amount100, testNow()); err != nil {
		t.Fatalf("Credit() error = %v", err)
	}

	if wallet.Version() != 2 {
		t.Fatalf(
			"Version() = %d, want 2",
			wallet.Version(),
		)
	}

	amount30, err := domain.NewMoney(
		"30.00",
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	if err := wallet.Debit(
		amount30,
		testNow().Add(time.Minute),
	); err != nil {
		t.Fatalf("Debit() error = %v", err)
	}

	if wallet.Version() != 3 {
		t.Fatalf(
			"Version() = %d, want 3",
			wallet.Version(),
		)
	}

	if wallet.Balance().Amount() != 7000 {
		t.Fatalf(
			"Balance().Amount() = %d, want 7000",
			wallet.Balance().Amount(),
		)
	}
}
