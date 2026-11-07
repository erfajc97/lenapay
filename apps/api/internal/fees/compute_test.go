package fees_test

import (
	"errors"
	"math"
	"testing"

	"github.com/erfajc97/lenapay/apps/api/internal/fees"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// The fixtures pin the rounding direction (FT13): a third decimal of exactly 5 rounds up, just
// below it rounds down, so a switch to floor, ceil or banker's rounding fails a case.
func TestCompute(t *testing.T) {
	t.Parallel()
	rates := fees.Rates{PayinBps: 50, PayoutFixedMinor: 15_000, PayoutBps: 20}
	cases := []struct {
		name  string
		op    fees.Operation
		minor int64
		want  int64
	}{
		{"payin 0.5% of 1.01 = 0.00505 rounds up to 0.01", fees.OperationPayin, 101, 1},
		{"payin 0.5% of 0.99 = 0.00495 rounds down to 0", fees.OperationPayin, 99, 0},
		{"payin 0.5% of 0.03 = 0.00015 is 0, not ceil", fees.OperationPayin, 3, 0},
		{"payin 0.5% of 10000.00", fees.OperationPayin, 1_000_000, 5_000},
		{"payout 150.00 + 0.2% of 123.45 = 0.2469 rounds to 0.25", fees.OperationPayout, 12_345, 15_025},
		{"payout 150.00 + 0.2% of 1.25 = 0.0025 rounds to 0", fees.OperationPayout, 125, 15_000},
		{"payout 150.00 + 0.2% of 2.50 = 0.005 rounds up to 0.01", fees.OperationPayout, 250, 15_001},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := fees.Compute(tc.op, money.New(tc.minor, money.ARS), rates)
			if err != nil {
				t.Fatal(err)
			}
			if got != money.New(tc.want, money.ARS) {
				t.Fatalf("fee = %v, want %d ARS minor", got, tc.want)
			}
		})
	}
}

func TestComputeRejectsWhatCannotBeCharged(t *testing.T) {
	t.Parallel()
	if _, err := fees.Compute(fees.OperationPayin, money.New(100, money.ARS), fees.Rates{PayinBps: 10_001}); !errors.Is(err, money.ErrInvalidBps) {
		t.Fatalf("bps above 100 percent: err = %v, want ErrInvalidBps", err)
	}
	if _, err := fees.Compute(fees.OperationPayin, money.New(100, money.ARS), fees.Rates{PayinBps: 10_000}); err != nil {
		t.Fatalf("bps = 100 percent is the last valid value: %v", err)
	}
	if _, err := fees.Compute(fees.OperationPayout, money.New(1, money.ARS), fees.Rates{PayoutFixedMinor: math.MaxInt64, PayoutBps: 10_000}); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("fixed + pct overflow: err = %v, want ErrOverflow", err)
	}
	if _, err := fees.Compute("refund", money.New(1, money.ARS), fees.Rates{}); err == nil {
		t.Fatal("unknown operation: want an error")
	}
}

func TestDefaultRatesCoverEveryCurrency(t *testing.T) {
	t.Parallel()
	rates := fees.DefaultRates()
	for _, c := range []money.Currency{money.ARS, money.USD} {
		r, ok := rates[c]
		if !ok || r.PayinBps < 0 || r.PayinBps > money.BpsDenominator || r.PayoutBps < 0 || r.PayoutBps > money.BpsDenominator || r.PayoutFixedMinor < 0 {
			t.Fatalf("default rates for %s = %+v (present %v)", c, r, ok)
		}
	}
	if len(rates) != 2 {
		t.Fatalf("default rates for %d currencies, want 2", len(rates))
	}
}
