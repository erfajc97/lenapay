package payins

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

var errBadSignature = errors.New("payins: bad webhook signature")

// webhookEvent is the signed body (PspTransferWebhook). Only TransferID is used to credit:
// the transfer is re-read at the PSP (MO4, AG16).
type webhookEvent struct {
	EventID    string    `json:"eventId"`
	Type       string    `json:"type"`
	TransferID string    `json:"transferId"`
	OccurredAt time.Time `json:"occurredAt"`
}

// eventTransferReceived is the only event type of the contract.
const eventTransferReceived = "transfer.received"

// verifySignature checks X-Signature = "sha256=" + hex(HMAC-SHA256(secret, timestamp + "." +
// raw body)) in constant time, and that the timestamp is within tolerance of now either way
// (a replay of an old delivery is refused even with a valid MAC).
func verifySignature(secret []byte, d WebhookDelivery, now time.Time, tolerance time.Duration) error {
	ts, err := strconv.ParseInt(d.Timestamp, 10, 64)
	if err != nil || len(secret) == 0 {
		return errBadSignature
	}
	if age := now.Sub(time.Unix(ts, 0)); age > tolerance || age < -tolerance {
		return errBadSignature
	}
	hexSig, ok := strings.CutPrefix(d.Signature, "sha256=")
	if !ok {
		return errBadSignature
	}
	got, err := hex.DecodeString(hexSig)
	if err != nil {
		return errBadSignature
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(d.Timestamp + "."))
	mac.Write(d.RawBody)
	if !hmac.Equal(got, mac.Sum(nil)) {
		return errBadSignature
	}
	return nil
}

// decodeEvent applies the contract's schema to the signed body: strict keys, required fields,
// lengths and the event type.
func decodeEvent(raw []byte) (webhookEvent, bool) {
	var ev webhookEvent
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ev); err != nil || !errors.Is(dec.Decode(&struct{}{}), io.EOF) {
		return webhookEvent{}, false
	}
	valid := ev.Type == eventTransferReceived && !ev.OccurredAt.IsZero() &&
		len(ev.EventID) >= 1 && len(ev.EventID) <= 100 && len(ev.TransferID) >= 1 && len(ev.TransferID) <= 100
	return ev, valid
}
