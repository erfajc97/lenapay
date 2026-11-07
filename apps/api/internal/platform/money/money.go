// Package money is the only money type of the API (P4, A6, MO1): an int64 amount in minor
// units plus an ISO currency. There is no float anywhere: arithmetic is integer, overflow is an
// error, and every division states its rounding.
package money

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
)

// Currency is an ISO 4217 code supported by Lenapay.
type Currency string

// Supported currencies (DB CHECK: currency IN ('ARS','USD')).
const (
	ARS Currency = "ARS"
	USD Currency = "USD"
)

// BpsDenominator is 100% expressed in basis points.
const BpsDenominator = 10_000

// Errors returned by this package; callers map them to catalog codes.
var (
	ErrCurrencyMismatch = errors.New("money: currency mismatch")
	ErrUnknownCurrency  = errors.New("money: unknown currency")
	ErrOverflow         = errors.New("money: int64 overflow")
	ErrNegative         = errors.New("money: negative amount not allowed here")
	ErrInvalidWeights   = errors.New("money: weights must be non-negative with a positive sum")
	ErrInvalidBps       = errors.New("money: basis points out of range")
)

// ParseCurrency validates a currency code.
func ParseCurrency(code string) (Currency, error) {
	switch Currency(code) {
	case ARS, USD:
		return Currency(code), nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownCurrency, code)
}

// Amount is a quantity of money in minor units (cents).
type Amount struct {
	Minor    int64
	Currency Currency
}

// New builds an Amount.
func New(minor int64, currency Currency) Amount {
	return Amount{Minor: minor, Currency: currency}
}

// Zero returns a zero amount in currency.
func Zero(currency Currency) Amount { return Amount{Currency: currency} }

// IsZero reports whether the amount is 0.
func (a Amount) IsZero() bool { return a.Minor == 0 }

// IsPositive reports whether the amount is > 0.
func (a Amount) IsPositive() bool { return a.Minor > 0 }

// IsNegative reports whether the amount is < 0.
func (a Amount) IsNegative() bool { return a.Minor < 0 }

// String renders "ARS 12.34"-style text for logs and errors only (never for parsing).
func (a Amount) String() string {
	sign, minor := "", a.Minor
	if minor < 0 {
		sign = "-"
	}
	abs := absUint(minor)
	return fmt.Sprintf("%s %s%d.%02d", a.Currency, sign, abs/100, abs%100)
}

// Add returns a + b; both must share the currency.
func (a Amount) Add(b Amount) (Amount, error) {
	if a.Currency != b.Currency {
		return Amount{}, fmt.Errorf("%w: %s + %s", ErrCurrencyMismatch, a.Currency, b.Currency)
	}
	sum := a.Minor + b.Minor
	// Overflow happened when both operands share a sign and the result does not.
	if (a.Minor > 0 && b.Minor > 0 && sum < 0) || (a.Minor < 0 && b.Minor < 0 && sum >= 0) {
		return Amount{}, ErrOverflow
	}
	return Amount{Minor: sum, Currency: a.Currency}, nil
}

// Sub returns a - b; both must share the currency.
func (a Amount) Sub(b Amount) (Amount, error) {
	neg, err := b.Neg()
	if err != nil {
		return Amount{}, err
	}
	return a.Add(neg)
}

// Neg returns -a.
func (a Amount) Neg() (Amount, error) {
	if a.Minor == math.MinInt64 {
		return Amount{}, ErrOverflow
	}
	return Amount{Minor: -a.Minor, Currency: a.Currency}, nil
}

// Rounding says how a division result that is not whole becomes minor units. Amounts handled
// here are never negative, so "down" and "toward zero" are the same.
type Rounding int

// Rounding modes. HalfUp rounds .5 away from zero.
const (
	RoundDown Rounding = iota + 1
	RoundHalfUp
	RoundUp
)

// ApplyBps returns amount × bps / 10 000 rounded as requested (fees: MO1 — one function, one
// rounding, used by quote, hold and capture alike).
func ApplyBps(amount Amount, bps int64, rounding Rounding) (Amount, error) {
	if amount.Minor < 0 {
		return Amount{}, ErrNegative
	}
	if bps < 0 || bps > BpsDenominator {
		return Amount{}, ErrInvalidBps
	}
	q, err := mulDiv(uint64(amount.Minor), uint64(bps), BpsDenominator, rounding)
	if err != nil {
		return Amount{}, err
	}
	return Amount{Minor: q, Currency: amount.Currency}, nil
}

// Split divides total into len(weights) parts proportional to weights with the
// largest-remainder method: every part is floored, then the cents left over go one by one to
// the parts with the largest remainders (ties: lowest index). The parts always sum to total.
func Split(total Amount, weights []int64) ([]Amount, error) {
	if total.Minor < 0 {
		return nil, ErrNegative
	}
	var sum uint64
	for _, w := range weights {
		if w < 0 {
			return nil, ErrInvalidWeights
		}
		next, carry := bits.Add64(sum, uint64(w), 0)
		if carry != 0 {
			return nil, ErrOverflow
		}
		sum = next
	}
	if sum == 0 {
		return nil, ErrInvalidWeights
	}

	parts := make([]Amount, len(weights))
	remainders := make([]uint64, len(weights))
	var assigned int64
	for i, w := range weights {
		hi, lo := bits.Mul64(uint64(total.Minor), uint64(w))           //nolint:gosec // total.Minor >= 0 and w >= 0 checked above
		quo, rem := bits.Div64(hi, lo, sum)                            // quo <= total.Minor because w <= sum: no overflow
		parts[i] = Amount{Minor: int64(quo), Currency: total.Currency} //nolint:gosec // quo <= total.Minor (w <= sum)
		remainders[i] = rem
		assigned += int64(quo) //nolint:gosec // quo <= total.Minor (w <= sum)
	}
	for left := total.Minor - assigned; left > 0; left-- {
		best := -1
		for i, rem := range remainders {
			if best == -1 || rem > remainders[best] {
				best = i
			}
		}
		parts[best].Minor++
		remainders[best] = 0
	}
	return parts, nil
}

// mulDiv computes a*b/d with 128-bit intermediate precision and the given rounding.
func mulDiv(a, b, d uint64, rounding Rounding) (int64, error) {
	hi, lo := bits.Mul64(a, b)
	if hi >= d {
		return 0, ErrOverflow
	}
	quo, rem := bits.Div64(hi, lo, d)
	switch rounding {
	case RoundDown:
	case RoundUp:
		if rem > 0 {
			quo++
		}
	case RoundHalfUp:
		if rem >= d-rem { // rem*2 >= d without overflowing
			quo++
		}
	default:
		return 0, fmt.Errorf("money: unknown rounding %d", rounding)
	}
	if quo > math.MaxInt64 {
		return 0, ErrOverflow
	}
	return int64(quo), nil
}

func absUint(v int64) uint64 {
	if v < 0 {
		return uint64(-(v + 1)) + 1 //nolint:gosec // -(v+1) >= 0 for every negative int64
	}
	return uint64(v)
}
