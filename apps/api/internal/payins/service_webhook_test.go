package payins

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

func sign(secret, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// AG16: valid signature; the body changed after signing; another secret; the last accepted and
// the first rejected timestamp age, both ways; malformed headers.
func TestVerifySignature(t *testing.T) {
	t.Parallel()
	const secret = "webhook-secret-0123456789"
	now := time.Unix(1_800_000_000, 0)
	tolerance := 5 * time.Minute
	body := []byte(`{"eventId":"evt_1","type":"transfer.received","transferId":"trf-1","occurredAt":"2027-01-15T08:00:00Z"}`)
	at := func(d time.Duration) string { return strconv.FormatInt(now.Add(d).Unix(), 10) }
	cases := []struct {
		name string
		d    WebhookDelivery
		ok   bool
	}{
		{"valid", WebhookDelivery{RawBody: body, Timestamp: at(0), Signature: sign(secret, at(0), body)}, true},
		{"exactly the tolerance old", WebhookDelivery{RawBody: body, Timestamp: at(-tolerance), Signature: sign(secret, at(-tolerance), body)}, true},
		{"one second past the tolerance", WebhookDelivery{RawBody: body, Timestamp: at(-tolerance - time.Second), Signature: sign(secret, at(-tolerance-time.Second), body)}, false},
		{"exactly the tolerance ahead", WebhookDelivery{RawBody: body, Timestamp: at(tolerance), Signature: sign(secret, at(tolerance), body)}, true},
		{"one second too far ahead", WebhookDelivery{RawBody: body, Timestamp: at(tolerance + time.Second), Signature: sign(secret, at(tolerance+time.Second), body)}, false},
		{"body changed after signing", WebhookDelivery{RawBody: append([]byte(" "), body...), Timestamp: at(0), Signature: sign(secret, at(0), body)}, false},
		{"another secret", WebhookDelivery{RawBody: body, Timestamp: at(0), Signature: sign("another-secret-0123456789", at(0), body)}, false},
		{"timestamp not signed", WebhookDelivery{RawBody: body, Timestamp: at(-time.Second), Signature: sign(secret, at(0), body)}, false},
		{"no sha256= prefix", WebhookDelivery{RawBody: body, Timestamp: at(0), Signature: sign(secret, at(0), body)[7:]}, false},
		{"not hex", WebhookDelivery{RawBody: body, Timestamp: at(0), Signature: "sha256=zz"}, false},
		{"no timestamp", WebhookDelivery{RawBody: body, Signature: sign(secret, "", body)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := verifySignature([]byte(secret), tc.d, now, tolerance)
			if tc.ok != (err == nil) || (err != nil && !errors.Is(err, errBadSignature)) {
				t.Fatalf("err = %v, want ok=%v", err, tc.ok)
			}
		})
	}
	if verifySignature(nil, cases[0].d, now, tolerance) == nil {
		t.Fatal("an empty secret must never verify")
	}
}

func TestDecodeEvent(t *testing.T) {
	t.Parallel()
	valid := `{"eventId":"evt_1","type":"transfer.received","transferId":"trf-1","occurredAt":"2027-01-15T08:00:00Z"}`
	if ev, ok := decodeEvent([]byte(valid)); !ok || ev.TransferID != "trf-1" || ev.EventID != "evt_1" {
		t.Fatalf("valid: %+v %v", ev, ok)
	}
	long := `"` + strings.Repeat("x", 101) + `"` // the contract caps ids at 100
	for name, body := range map[string]string{
		"unknown field":     `{"eventId":"evt_1","type":"transfer.received","transferId":"trf-1","occurredAt":"2027-01-15T08:00:00Z","amount":1}`,
		"other type":        `{"eventId":"evt_1","type":"transfer.sent","transferId":"trf-1","occurredAt":"2027-01-15T08:00:00Z"}`,
		"no transfer":       `{"eventId":"evt_1","type":"transfer.received","occurredAt":"2027-01-15T08:00:00Z"}`,
		"event id too long": `{"eventId":` + long + `,"type":"transfer.received","transferId":"trf-1","occurredAt":"2027-01-15T08:00:00Z"}`,
		"no occurredAt":     `{"eventId":"evt_1","type":"transfer.received","transferId":"trf-1"}`,
		"trailing object":   valid + `{}`,
		"not json":          `nope`,
		"transfer too long": `{"eventId":"evt_1","type":"transfer.received","transferId":` + long + `,"occurredAt":"2027-01-15T08:00:00Z"}`,
	} {
		if _, ok := decodeEvent([]byte(body)); ok {
			t.Errorf("%s accepted", name)
		}
	}
}
