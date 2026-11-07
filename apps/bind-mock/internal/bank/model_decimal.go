package bank

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidAmount is a malformed or non-positive amount.
var ErrInvalidAmount = errors.New("bank: invalid amount")

// Decimal is an amount in minor units that travels as BIND sends it: a JSON number with two
// decimals ("value":{"amount":1500.25}). It is parsed digit by digit, never through a float.
type Decimal int64

// MarshalJSON writes the number literal 1500.25.
func (d Decimal) MarshalJSON() ([]byte, error) { return []byte(FormatMinor(int64(d))), nil }

// UnmarshalJSON accepts a JSON number or a quoted decimal with at most two fraction digits.
func (d *Decimal) UnmarshalJSON(b []byte) error {
	minor, err := ParseMinor(string(bytes.Trim(b, `"`)))
	if err != nil {
		return err
	}
	*d = Decimal(minor)
	return nil
}

// ParseMinor turns "1500", "1500.5" or "1500.25" into minor units (more than two decimals,
// signs, exponents or an empty string are rejected).
func ParseMinor(s string) (int64, error) {
	units, frac, hasFrac := strings.Cut(s, ".")
	if units == "" || len(frac) > 2 || (hasFrac && frac == "") || !digits(units) || !digits(frac) || len(units) > 15 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	}
	u, err := strconv.ParseInt(units, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	}
	f := int64(0)
	if frac != "" {
		f, _ = strconv.ParseInt((frac + "0")[:2], 10, 64) // digits() already checked
	}
	return u*100 + f, nil
}

// FormatMinor renders minor units as "1500.25" (negative amounts keep their sign).
func FormatMinor(minor int64) string {
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	return fmt.Sprintf("%s%d.%02d", sign, minor/100, minor%100)
}

func digits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
