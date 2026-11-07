package log

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

// A15: every line of a request carries its id, and secrets or PII never reach the output.
func TestLinesCarryRequestIDAndRedact(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)
	ctx := WithRequestID(t.Context(), "req-0000000001")
	logger.InfoContext(ctx, "login", "authorization", "Bearer abc", "cvu", "0000003100000000000001", "user_id", "u1")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("not JSON: %s", buf.String())
	}
	if line["request_id"] != "req-0000000001" || line["user_id"] != "u1" {
		t.Fatalf("line %v", line)
	}
	for _, k := range []string{"authorization", "cvu"} {
		if line[k] != "[REDACTED]" {
			t.Errorf("%s = %v, want [REDACTED]", k, line[k])
		}
	}
}

func TestNoRequestIDOutsideARequest(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	New(&buf, slog.LevelInfo).With("k", "v").WithGroup("g").InfoContext(t.Context(), "boot", "x", 1)
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if _, ok := line["request_id"]; ok || RequestID(t.Context()) != "" {
		t.Fatalf("line %v", line)
	}
}
