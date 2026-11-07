package db

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSortedIDs(t *testing.T) {
	t.Parallel()
	a := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	b := uuid.MustParse("00000000-0000-7000-8000-000000000002")
	got := SortedIDs([]uuid.UUID{b, a, b})
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("SortedIDs = %v", got)
	}
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	for code, want := range map[string]bool{"40001": true, "40P01": true, "23505": false} {
		if got := IsRetryable(&pgconn.PgError{Code: code}); got != want {
			t.Errorf("%s: %v", code, got)
		}
	}
	if IsRetryable(errors.New("x")) {
		t.Error("plain error is not retryable")
	}
}
