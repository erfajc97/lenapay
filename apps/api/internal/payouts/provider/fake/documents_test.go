package fake

import (
	"bytes"
	"testing"
)

func TestDiskDocumentStoreRoundTripAndContainment(t *testing.T) {
	t.Parallel()
	s := NewDiskDocumentStore(t.TempDir())
	if err := s.Put(t.Context(), "org/a/payouts/b/c", "application/pdf", bytes.NewReader([]byte("%PDF-1"))); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("org/a/payouts/b/c")
	if err != nil || string(got) != "%PDF-1" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	for _, key := range []string{"", "../escape", "org/../../escape"} {
		if err := s.Put(t.Context(), key, "", bytes.NewReader(nil)); err == nil {
			t.Errorf("Put(%q) escaped the root", key)
		}
	}
}
