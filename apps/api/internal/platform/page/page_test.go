package page

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/apperr"
)

func ptr[T any](v T) *T { return &v }

func TestParseRequestLimits(t *testing.T) {
	cases := []struct {
		name  string
		limit *int
		want  int
		code  apperr.Code
	}{
		{"default", nil, DefaultLimit, ""},
		{"floor", ptr(1), 1, ""},
		{"below floor", ptr(0), 0, apperr.CodeValidation},
		{"ceiling", ptr(MaxLimit), MaxLimit, ""},
		{"above ceiling", ptr(MaxLimit + 1), 0, apperr.CodeValidation},
	}
	for _, c := range cases {
		req, err := ParseRequest(nil, c.limit)
		if apperr.CodeOf(err) != c.code {
			t.Errorf("%s: err = %v, want code %q", c.name, err, c.code)
			continue
		}
		if c.code == "" && req.Limit != c.want {
			t.Errorf("%s: limit = %d, want %d", c.name, req.Limit, c.want)
		}
	}
}

func TestCursorRoundTrip(t *testing.T) {
	pos := Position{CreatedAt: time.Date(2026, 11, 7, 10, 30, 0, 123456000, time.UTC), ID: uuid.MustParse("01900000-0000-7000-8000-000000000042")}
	req, err := ParseRequest(ptr(Encode(pos)), nil)
	if err != nil || req.After == nil || !req.After.CreatedAt.Equal(pos.CreatedAt) || req.After.ID != pos.ID {
		t.Fatalf("round trip = %+v, %v", req.After, err)
	}
	for _, bad := range []string{"%%%", "bm90LWEtY3Vyc29y", Encode(pos)[:5]} {
		if _, err := ParseRequest(ptr(bad), nil); apperr.CodeOf(err) != apperr.CodeValidation {
			t.Errorf("cursor %q: err = %v", bad, err)
		}
	}
}

func TestBuild(t *testing.T) {
	type row struct{ n int }
	at := func(r row) Position {
		return Position{CreatedAt: time.Unix(int64(r.n), 0).UTC(), ID: uuid.UUID{byte(r.n)}}
	}
	full := Build([]row{{3}, {2}, {1}}, 2, at)
	if len(full.Items) != 2 || full.NextCursor == nil {
		t.Fatalf("page with an extra row must have a cursor: %+v", full)
	}
	if pos, _ := Decode(*full.NextCursor); pos.CreatedAt.Unix() != 2 {
		t.Fatalf("cursor must point at the last returned row, got %v", pos)
	}
	last := Build([]row{{1}, {0}}, 2, at)
	if len(last.Items) != 2 || last.NextCursor != nil {
		t.Fatalf("last page must not have a cursor: %+v", last)
	}
	if empty := Build([]row{}, 2, at); len(empty.Items) != 0 || empty.NextCursor != nil {
		t.Fatalf("empty page: %+v", empty)
	}
	if (Request{Limit: 20}).FetchLimit() != 21 {
		t.Fatal("FetchLimit must ask for one extra row")
	}
}
