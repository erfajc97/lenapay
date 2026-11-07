// Package webhook signs and sends the "transfer.received" notifications the fake BIND posts to
// the API, in the format of contracts/openapi.yaml (receivePspTransferWebhook):
//
//	X-Signature: sha256=<hex HMAC-SHA256(secret, timestamp + "." + raw body)>
//	X-Signature-Timestamp: <unix seconds>
//
// The body only announces the transfer; the API re-reads it before crediting.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Headers of a signed webhook.
const (
	HeaderSignature = "X-Signature"
	HeaderTimestamp = "X-Signature-Timestamp"
)

// Event is the webhook body (PspTransferWebhook in the contract).
type Event struct {
	EventID    string    `json:"eventId"`
	Type       string    `json:"type"`
	TransferID string    `json:"transferId"`
	OccurredAt time.Time `json:"occurredAt"`
}

// TypeTransferReceived is the only event type the mock sends.
const TypeTransferReceived = "transfer.received"

// Sign returns the X-Signature value for body sent at timestamp.
func Sign(secret []byte, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Sender posts signed events with its own timed client.
type Sender struct {
	url    string
	secret []byte
	client *http.Client
	now    func() time.Time
}

// NewSender builds a sender; timeout bounds each delivery.
func NewSender(url string, secret []byte, timeout time.Duration, now func() time.Time) *Sender {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: timeout,
		IdleConnTimeout:       90 * time.Second,
	}
	return &Sender{url: url, secret: secret, client: &http.Client{Timeout: timeout, Transport: transport}, now: now}
}

// Send delivers ev once and returns the HTTP status (0 when the request never got an answer).
func (s *Sender) Send(ctx context.Context, ev Event) (int, error) {
	body, err := json.Marshal(ev)
	if err != nil {
		return 0, fmt.Errorf("webhook: encode: %w", err)
	}
	ts := strconv.FormatInt(s.now().Unix(), 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("webhook: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderSignature, Sign(s.secret, ts, body))
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("webhook: post: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10)) // drain so the connection is reused
	if resp.StatusCode >= http.StatusMultipleChoices {
		return resp.StatusCode, fmt.Errorf("webhook: answered %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}
