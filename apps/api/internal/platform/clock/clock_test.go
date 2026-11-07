package clock

import (
	"testing"
	"time"
)

func TestFakeIsFrozenUntilMoved(t *testing.T) {
	start := time.Date(2026, 11, 7, 12, 0, 0, 0, time.FixedZone("ART", -3*3600))
	f := NewFake(start)
	if got := f.Now(); !got.Equal(start) || got.Location() != time.UTC {
		t.Fatalf("Now = %v, want %v in UTC", got, start)
	}
	f.Advance(90 * time.Second)
	if got := f.Now(); !got.Equal(start.Add(90 * time.Second)) {
		t.Fatalf("after Advance Now = %v", got)
	}
	later := start.Add(24 * time.Hour)
	f.Set(later)
	if got := f.Now(); !got.Equal(later) {
		t.Fatalf("after Set Now = %v", got)
	}
}

func TestRealIsUTC(t *testing.T) {
	if Real.Now(Real{}).Location() != time.UTC {
		t.Fatal("Real clock must return UTC")
	}
}
