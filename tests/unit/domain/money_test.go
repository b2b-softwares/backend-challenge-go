package domain_test

import (
	"errors"
	"math"
	"testing"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		currency domain.Currency
		want     int64
	}{
		{
			name:     "positive amount",
			amount:   "10.00",
			currency: domain.CurrencyBRL,
			want:     1000,
		},
		{
			name:     "zero amount",
			amount:   "0.00",
			currency: domain.CurrencyBRL,
			want:     0,
		},
		{
			name:     "large valid amount",
			amount:   "92233720368547758.07",
			currency: domain.CurrencyBRL,
			want:     math.MaxInt64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := domain.NewMoney(tt.amount, tt.currency)
			if err != nil {
				t.Fatalf("NewMoney() error = %v", err)
			}

			if money.Amount() != tt.want {
				t.Fatalf("Amount() = %d, want %d", money.Amount(), tt.want)
			}

			if money.Currency() != tt.currency {
				t.Fatalf("Currency() = %s, want %s", money.Currency(), tt.currency)
			}
		})
	}
}

func TestNewMoneyInvalidValues(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		currency domain.Currency
	}{
		{
			name:     "empty",
			amount:   "",
			currency: domain.CurrencyBRL,
		},
		{
			name:     "negative",
			amount:   "-10.00",
			currency: domain.CurrencyBRL,
		},
		{
			name:     "positive sign",
			amount:   "+10.00",
			currency: domain.CurrencyBRL,
		},
		{
			name:     "missing decimal places",
			amount:   "10",
			currency: domain.CurrencyBRL,
		},
		{
			name:     "one decimal place",
			amount:   "10.0",
			currency: domain.CurrencyBRL,
		},
		{
			name:     "three decimal places",
			amount:   "10.000",
			currency: domain.CurrencyBRL,
		},
		{
			name:     "scientific notation",
			amount:   "1e2",
			currency: domain.CurrencyBRL,
		},
		{
			name:     "NaN",
			amount:   "NaN",
			currency: domain.CurrencyBRL,
		},
		{
			name:     "Infinity",
			amount:   "Infinity",
			currency: domain.CurrencyBRL,
		},
		{
			name:     "lowercase currency",
			amount:   "10.00",
			currency: domain.Currency("brl"),
		},
		{
			name:     "invalid currency length",
			amount:   "10.00",
			currency: domain.Currency("BR"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewMoney(tt.amount, tt.currency)
			if err == nil {
				t.Fatal("NewMoney() expected error, got nil")
			}

			if !errors.Is(err, domain.ErrInvalidMoney) &&
				!errors.Is(err, domain.ErrInvalidCurrency) {
				t.Fatalf(
					"NewMoney() error = %v, want ErrInvalidMoney or ErrInvalidCurrency",
					err,
				)
			}
		})
	}
}

func TestMoneyString(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		want   string
	}{
		{
			name:   "zero",
			amount: 0,
			want:   "0.00",
		},
		{
			name:   "positive",
			amount: 12345,
			want:   "123.45",
		},
		{
			name:   "small positive",
			amount: 1,
			want:   "0.01",
		},
		{
			name:   "negative",
			amount: -12345,
			want:   "-123.45",
		},
		{
			name:   "minimum int64",
			amount: math.MinInt64,
			want:   "-92233720368547758.08",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := domain.MoneyFromMinorUnits(
				tt.amount,
				domain.CurrencyBRL,
			)
			if err != nil {
				t.Fatalf("MoneyFromMinorUnits() error = %v", err)
			}

			if got := money.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMoneyAdd(t *testing.T) {
	left, err := domain.NewMoney("10.00", domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	right, err := domain.NewMoney("5.50", domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	result, err := left.Add(right)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if result.Amount() != 1550 {
		t.Fatalf("Add() = %d, want 1550", result.Amount())
	}

	if left.Amount() != 1000 {
		t.Fatalf("Add() mutated original money: got %d", left.Amount())
	}
}

func TestMoneySub(t *testing.T) {
	left, err := domain.NewMoney("10.00", domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	right, err := domain.NewMoney("5.50", domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	result, err := left.Sub(right)
	if err != nil {
		t.Fatalf("Sub() error = %v", err)
	}

	if result.Amount() != 450 {
		t.Fatalf("Sub() = %d, want 450", result.Amount())
	}
}

func TestMoneySubCanProduceNegativeInternalValue(t *testing.T) {
	left, err := domain.MoneyFromMinorUnits(100, domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	right, err := domain.MoneyFromMinorUnits(200, domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	result, err := left.Sub(right)
	if err != nil {
		t.Fatalf("Sub() error = %v", err)
	}

	if result.Amount() != -100 {
		t.Fatalf("Sub() = %d, want -100", result.Amount())
	}
}

func TestMoneyCurrencyMismatch(t *testing.T) {
	brl, err := domain.MoneyFromMinorUnits(1000, domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	usd, err := domain.MoneyFromMinorUnits(1000, domain.Currency("USD"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = brl.Add(usd)
	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("Add() error = %v, want ErrCurrencyMismatch", err)
	}

	_, err = brl.Sub(usd)
	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("Sub() error = %v, want ErrCurrencyMismatch", err)
	}

	_, err = brl.Compare(usd)
	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("Compare() error = %v, want ErrCurrencyMismatch", err)
	}
}

func TestMoneyAddOverflow(t *testing.T) {
	left, err := domain.MoneyFromMinorUnits(math.MaxInt64, domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	right, err := domain.MoneyFromMinorUnits(1, domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	_, err = left.Add(right)
	if !errors.Is(err, domain.ErrInvalidMoney) {
		t.Fatalf("Add() error = %v, want ErrInvalidMoney", err)
	}
}

func TestMoneyAddNegativeOverflow(t *testing.T) {
	left, err := domain.MoneyFromMinorUnits(math.MinInt64, domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	right, err := domain.MoneyFromMinorUnits(-1, domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	_, err = left.Add(right)
	if !errors.Is(err, domain.ErrInvalidMoney) {
		t.Fatalf("Add() error = %v, want ErrInvalidMoney", err)
	}
}

func TestMoneySubMinInt64Overflow(t *testing.T) {
	left, err := domain.MoneyFromMinorUnits(0, domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	right, err := domain.MoneyFromMinorUnits(math.MinInt64, domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	_, err = left.Sub(right)
	if !errors.Is(err, domain.ErrInvalidMoney) {
		t.Fatalf("Sub() error = %v, want ErrInvalidMoney", err)
	}
}

func TestMoneyNegateMinInt64Overflow(t *testing.T) {
	money, err := domain.MoneyFromMinorUnits(
		math.MinInt64,
		domain.CurrencyBRL,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = money.Negate()
	if !errors.Is(err, domain.ErrInvalidMoney) {
		t.Fatalf("Negate() error = %v, want ErrInvalidMoney", err)
	}
}

func TestMoneyEqual(t *testing.T) {
	first, err := domain.MoneyFromMinorUnits(1000, domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	second, err := domain.MoneyFromMinorUnits(1000, domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	differentAmount, err := domain.MoneyFromMinorUnits(1001, domain.CurrencyBRL)
	if err != nil {
		t.Fatal(err)
	}

	differentCurrency, err := domain.MoneyFromMinorUnits(
		1000,
		domain.Currency("USD"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !first.Equal(second) {
		t.Fatal("Equal() = false, want true")
	}

	if first.Equal(differentAmount) {
		t.Fatal("Equal() = true for different amounts")
	}

	if first.Equal(differentCurrency) {
		t.Fatal("Equal() = true for different currencies")
	}
}
