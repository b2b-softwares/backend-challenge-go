package domain

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Currency string

const CurrencyBRL Currency = "BRL"

type Money struct {
	amountMinor int64
	currency    Currency
}

func NewMoney(amount string, currency Currency) (Money, error) {
	if !validateCurrency(currency) {
		return Money{}, ErrInvalidCurrency
	}

	minor, err := parseCents(amount)
	if err != nil {
		return Money{}, fmt.Errorf("%w: %v", ErrInvalidMoney, err)
	}

	return Money{
		amountMinor: minor,
		currency:    currency,
	}, nil
}

func MoneyFromMinorUnits(amountMinor int64, currency Currency) (Money, error) {
	if !validateCurrency(currency) {
		return Money{}, ErrInvalidCurrency
	}

	return Money{
		amountMinor: amountMinor,
		currency:    currency,
	}, nil
}

func ZeroMoney(currency Currency) (Money, error) {
	return MoneyFromMinorUnits(0, currency)
}

func (m Money) Amount() int64 {
	return m.amountMinor
}

func (m Money) Currency() Currency {
	return m.currency
}

func (m Money) IsZero() bool {
	return m.amountMinor == 0
}

func (m Money) IsPositive() bool {
	return m.amountMinor > 0
}

func (m Money) IsNegative() bool {
	return m.amountMinor < 0
}

func (m Money) Add(other Money) (Money, error) {
	if err := m.ensureCompatible(other); err != nil {
		return Money{}, err
	}

	if other.amountMinor > 0 &&
		m.amountMinor > math.MaxInt64-other.amountMinor {
		return Money{}, fmt.Errorf("%w: addition overflow", ErrInvalidMoney)
	}

	if other.amountMinor < 0 &&
		m.amountMinor < math.MinInt64-other.amountMinor {
		return Money{}, fmt.Errorf("%w: addition overflow", ErrInvalidMoney)
	}

	return Money{
		amountMinor: m.amountMinor + other.amountMinor,
		currency:    m.currency,
	}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if err := m.ensureCompatible(other); err != nil {
		return Money{}, err
	}

	if other.amountMinor == math.MinInt64 {
		return Money{}, fmt.Errorf(
			"%w: subtraction overflow",
			ErrInvalidMoney,
		)
	}

	return m.Add(Money{
		amountMinor: -other.amountMinor,
		currency:    other.currency,
	})
}

func (m Money) Negate() (Money, error) {
	if m.amountMinor == math.MinInt64 {
		return Money{}, fmt.Errorf(
			"%w: negation overflow",
			ErrInvalidMoney,
		)
	}

	return Money{
		amountMinor: -m.amountMinor,
		currency:    m.currency,
	}, nil
}

func (m Money) Compare(other Money) (int, error) {
	if err := m.ensureCompatible(other); err != nil {
		return 0, err
	}

	switch {
	case m.amountMinor < other.amountMinor:
		return -1, nil
	case m.amountMinor > other.amountMinor:
		return 1, nil
	default:
		return 0, nil
	}
}

func (m Money) Equal(other Money) bool {
	return m.currency == other.currency &&
		m.amountMinor == other.amountMinor
}

func (m Money) String() string {
	amount := m.amountMinor

	if amount < 0 {
		if amount == math.MinInt64 {
			unsigned := uint64(-(amount + 1))
			unsigned++

			whole := unsigned / 100
			fraction := unsigned % 100

			return fmt.Sprintf("-%d.%02d", whole, fraction)
		}

		amount = -amount

		whole := amount / 100
		fraction := amount % 100

		return fmt.Sprintf("-%d.%02d", whole, fraction)
	}

	whole := amount / 100
	fraction := amount % 100

	return fmt.Sprintf("%d.%02d", whole, fraction)
}

func (m Money) ensureCompatible(other Money) error {
	if !validateCurrency(m.currency) ||
		!validateCurrency(other.currency) {
		return ErrInvalidCurrency
	}

	if m.currency != other.currency {
		return ErrCurrencyMismatch
	}

	return nil
}

func validateCurrency(currency Currency) bool {
	value := string(currency)

	if len(value) != 3 {
		return false
	}

	for i := 0; i < len(value); i++ {
		if value[i] < 'A' || value[i] > 'Z' {
			return false
		}
	}

	return true
}

func parseCents(value string) (int64, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return 0, fmt.Errorf("amount is empty")
	}

	if strings.ContainsAny(value, "eE") {
		return 0, fmt.Errorf("scientific notation is not allowed")
	}

	if strings.ContainsAny(value, "+-") {
		return 0, fmt.Errorf("signed external values are not allowed")
	}

	parts := strings.Split(value, ".")

	if len(parts) != 2 {
		return 0, fmt.Errorf(
			"amount must contain exactly two decimal places",
		)
	}

	whole := parts[0]
	fraction := parts[1]

	if whole == "" {
		return 0, fmt.Errorf("whole part is required")
	}

	if len(fraction) != 2 {
		return 0, fmt.Errorf(
			"amount must contain exactly two decimal places",
		)
	}

	for i := 0; i < len(whole); i++ {
		if whole[i] < '0' || whole[i] > '9' {
			return 0, fmt.Errorf("invalid whole part")
		}
	}

	for i := 0; i < len(fraction); i++ {
		if fraction[i] < '0' || fraction[i] > '9' {
			return 0, fmt.Errorf("invalid fractional part")
		}
	}

	wholeValue, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("whole part overflow")
	}

	fractionValue :=
		int64(fraction[0]-'0')*10 +
			int64(fraction[1]-'0')

	if wholeValue > math.MaxInt64/100 {
		return 0, fmt.Errorf("amount overflow")
	}

	result := wholeValue * 100

	if result > math.MaxInt64-fractionValue {
		return 0, fmt.Errorf("amount overflow")
	}

	return result + fractionValue, nil
}
