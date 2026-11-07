package ids

import "testing"

// Ids must be v7 so primary keys sort by creation time (keyset pagination relies on it).
func TestNewIsVersion7AndOrdered(t *testing.T) {
	first, second := New(), New()
	if first.Version() != 7 {
		t.Fatalf("version = %d, want 7", first.Version())
	}
	if first.String() >= second.String() {
		t.Fatalf("ids not increasing: %s then %s", first, second)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := Parse("not-a-uuid"); err == nil {
		t.Fatal("expected an error for a malformed id")
	}
}
