package money

import (
	"errors"
	"math"
	"testing"
)

func TestParseCurrency(t *testing.T) {
	for _, code := range []string{"ARS", "USD"} {
		if c, err := ParseCurrency(code); err != nil || string(c) != code {
			t.Errorf("ParseCurrency(%q) = %q, %v", code, c, err)
		}
	}
	for _, code := range []string{"", "ars", "EUR", "US"} {
		if _, err := ParseCurrency(code); !errors.Is(err, ErrUnknownCurrency) {
			t.Errorf("ParseCurrency(%q) err = %v, want ErrUnknownCurrency", code, err)
		}
	}
}

func TestAddAndSub(t *testing.T) {
	tests := []struct {
		name    string
		a, b    Amount
		add     int64
		sub     int64
		wantErr error
	}{
		{name: "plain", a: New(1050, ARS), b: New(25, ARS), add: 1075, sub: 1025},
		{name: "to negative", a: New(10, USD), b: New(25, USD), add: 35, sub: -15},
		{name: "currency mismatch", a: New(1, ARS), b: New(1, USD), wantErr: ErrCurrencyMismatch},
		// Boundary pair (FT1): MaxInt64 itself is reachable, one more cent overflows.
		{name: "last value that fits", a: New(math.MaxInt64-1, ARS), b: New(1, ARS), add: math.MaxInt64, sub: math.MaxInt64 - 2},
		{name: "overflow", a: New(math.MaxInt64, ARS), b: New(1, ARS), wantErr: ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sum, err := tt.a.Add(tt.b)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Add err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || sum.Minor != tt.add || sum.Currency != tt.a.Currency {
				t.Fatalf("Add = %v, %v; want %d", sum, err, tt.add)
			}
			diff, err := tt.a.Sub(tt.b)
			if err != nil || diff.Minor != tt.sub {
				t.Fatalf("Sub = %v, %v; want %d", diff, err, tt.sub)
			}
		})
	}
}

func TestAddNegativeOverflow(t *testing.T) {
	if _, err := New(math.MinInt64, ARS).Add(New(-1, ARS)); !errors.Is(err, ErrOverflow) {
		t.Fatalf("err = %v, want ErrOverflow", err)
	}
	if _, err := New(math.MinInt64, ARS).Neg(); !errors.Is(err, ErrOverflow) {
		t.Fatalf("Neg err = %v, want ErrOverflow", err)
	}
}

// Fee rounding must be explicit and identical everywhere (MO1). Fixtures put the third
// decimal at >= 5 so floor, half-up and ceil disagree (FT13).
func TestApplyBps(t *testing.T) {
	tests := []struct {
		name     string
		minor    int64
		bps      int64
		rounding Rounding
		want     int64
	}{
		// 1 234.56 ARS × 1.25% = 15.432 → down 15.43, half-up 15.43, up 15.44
		{"1.25% down", 123456, 125, RoundDown, 1543},
		{"1.25% half-up below half", 123456, 125, RoundHalfUp, 1543},
		{"1.25% up", 123456, 125, RoundUp, 1544},
		// 10.30 × 0.5% = 0.0515 → 5.15 cents: down 5, half-up 5, up 6
		{"0.5% half-up .15", 1030, 50, RoundHalfUp, 5},
		// 11.00 × 0.5% = 5.5 cents → 6; 10.99 × 0.5% = 5.495 cents → 5
		{"exact half rounds up", 1100, 50, RoundHalfUp, 6},
		{"just below half", 1099, 50, RoundHalfUp, 5},
		{"exact, no rounding", 10000, 100, RoundUp, 100},
		{"zero amount", 0, 300, RoundUp, 0},
		{"zero bps", 99999, 0, RoundUp, 0},
		{"100%", 4242, BpsDenominator, RoundDown, 4242},
		// Large amounts use 128-bit intermediates: MaxInt64 × 100% must not overflow.
		{"max amount", math.MaxInt64, BpsDenominator, RoundDown, math.MaxInt64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyBps(New(tt.minor, ARS), tt.bps, tt.rounding)
			if err != nil {
				t.Fatalf("ApplyBps err = %v", err)
			}
			if got.Minor != tt.want || got.Currency != ARS {
				t.Fatalf("ApplyBps(%d, %d) = %d, want %d", tt.minor, tt.bps, got.Minor, tt.want)
			}
		})
	}
}

func TestApplyBpsRejects(t *testing.T) {
	cases := []struct {
		name  string
		minor int64
		bps   int64
		round Rounding
		want  error
	}{
		{"negative amount", -1, 100, RoundUp, ErrNegative},
		{"negative bps", 100, -1, RoundUp, ErrInvalidBps},
		{"bps above 100%", 100, BpsDenominator + 1, RoundUp, ErrInvalidBps},
	}
	for _, c := range cases {
		if _, err := ApplyBps(New(c.minor, ARS), c.bps, c.round); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
	if _, err := ApplyBps(New(100, ARS), 100, Rounding(99)); err == nil {
		t.Error("unknown rounding must fail")
	}
}

func TestSplitLargestRemainder(t *testing.T) {
	tests := []struct {
		name    string
		total   int64
		weights []int64
		want    []int64
	}{
		// 100 / 3: floors 33,33,33 → the leftover cent goes to the first (tie → lowest index).
		{"thirds", 100, []int64{1, 1, 1}, []int64{34, 33, 33}},
		// 1000 × (1/6, 2/6, 3/6) = 166.67, 333.33, 500 → largest remainder (.67) gets the cent.
		{"weighted", 1000, []int64{1, 2, 3}, []int64{167, 333, 500}},
		{"exact", 900, []int64{1, 2}, []int64{300, 600}},
		{"zero weight gets nothing", 5, []int64{0, 1, 1}, []int64{0, 3, 2}},
		{"zero total", 0, []int64{1, 1}, []int64{0, 0}},
		{"huge total no overflow", math.MaxInt64, []int64{1, 1}, []int64{math.MaxInt64/2 + 1, math.MaxInt64 / 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, err := Split(New(tt.total, USD), tt.weights)
			if err != nil {
				t.Fatalf("Split err = %v", err)
			}
			var sum int64
			for i, p := range parts {
				if p.Minor != tt.want[i] || p.Currency != USD {
					t.Fatalf("parts = %v, want %v", parts, tt.want)
				}
				sum += p.Minor
			}
			if sum != tt.total {
				t.Fatalf("parts sum to %d, want %d", sum, tt.total)
			}
		})
	}
}

func TestSplitRejects(t *testing.T) {
	if _, err := Split(New(-1, ARS), []int64{1}); !errors.Is(err, ErrNegative) {
		t.Errorf("negative total err = %v", err)
	}
	if _, err := Split(New(1, ARS), []int64{1, -1}); !errors.Is(err, ErrInvalidWeights) {
		t.Errorf("negative weight err = %v", err)
	}
	if _, err := Split(New(1, ARS), []int64{0, 0}); !errors.Is(err, ErrInvalidWeights) {
		t.Errorf("zero weights err = %v", err)
	}
	if _, err := Split(New(1, ARS), nil); !errors.Is(err, ErrInvalidWeights) {
		t.Errorf("no weights err = %v", err)
	}
}

func TestString(t *testing.T) {
	cases := map[string]Amount{
		"ARS 12.34":                 New(1234, ARS),
		"USD -0.05":                 New(-5, USD),
		"ARS 0.00":                  Zero(ARS),
		"ARS -92233720368547758.08": New(math.MinInt64, ARS),
	}
	for want, a := range cases {
		if got := a.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

func TestPredicates(t *testing.T) {
	if !Zero(ARS).IsZero() || New(1, ARS).IsZero() {
		t.Error("IsZero")
	}
	if !New(1, ARS).IsPositive() || New(0, ARS).IsPositive() {
		t.Error("IsPositive")
	}
	if !New(-1, ARS).IsNegative() || New(0, ARS).IsNegative() {
		t.Error("IsNegative")
	}
}
