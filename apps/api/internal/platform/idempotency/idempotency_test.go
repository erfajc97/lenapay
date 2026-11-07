package idempotency

import (
	"testing"

	"github.com/google/uuid"
)

// Equal requests hash equally and any changed field changes the hash (the 409 depends on it).
func TestHashRequest(t *testing.T) {
	t.Parallel()
	type req struct {
		Account string
		Minor   int64
	}
	a, _ := HashRequest(req{"a1", 1000})
	b, _ := HashRequest(req{"a1", 1000})
	c, _ := HashRequest(req{"a1", 1001})
	if a != b || a == c {
		t.Fatalf("hashes: %x %x %x", a, b, c)
	}
	if _, err := HashRequest(make(chan int)); err == nil {
		t.Fatal("unencodable request must fail")
	}
}

func TestScopeIsPerOrgAndOperation(t *testing.T) {
	t.Parallel()
	org := uuid.MustParse("01900000-0000-7000-8000-000000000001")
	if got := Scope(org, "payouts.create"); got != "org:01900000-0000-7000-8000-000000000001:payouts.create" {
		t.Fatalf("scope %q", got)
	}
}
