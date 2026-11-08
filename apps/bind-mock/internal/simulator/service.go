// Package simulator plays the payers of the demo: it credits a CVU at the fake bank and
// delivers the signed webhook to the API, once on demand (/_mock/incoming-transfer) or every N
// seconds with random amounts and payers (/_mock/auto-payins). It also delivers the webhook of
// every on-us credit the bank books (an outgoing transfer to one of our own CVUs).
package simulator

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"lenapay/apps/bind-mock/internal/bank"
	"lenapay/apps/bind-mock/internal/simulator/provider/webhook"
)

// errNoActiveCVU means there is nobody to pay yet (no active caja).
var errNoActiveCVU = errors.New("simulator: no active CVU")

// WebhookSender delivers one event.
type WebhookSender interface {
	Send(ctx context.Context, ev webhook.Event) (int, error)
}

// Simulator credits CVUs and notifies the API.
type Simulator struct {
	bank   *bank.Bank
	sender WebhookSender
	now    func() time.Time

	mu      sync.Mutex
	cancel  context.CancelFunc
	every   time.Duration
	stopped chan struct{}

	// onUs counts the on-us webhooks in flight (Wait drains them); they run on base, the
	// process context, since no request waits for them.
	onUs sync.WaitGroup
	base context.Context
}

// New returns a simulator and makes it the bank's credit listener. ctx is the process context
// the on-us webhooks are delivered on (detached from its cancellation: an in-flight delivery
// finishes during shutdown).
func New(ctx context.Context, b *bank.Bank, sender WebhookSender, now func() time.Time) *Simulator {
	s := &Simulator{bank: b, sender: sender, now: now, base: context.WithoutCancel(ctx)}
	b.SetCreditListener(s.onCredit)
	return s
}

// Receive credits a CVU and delivers its webhook. The transfer stands even when the webhook
// fails (the API can be down): the delivery is recorded and can be repeated with Redeliver.
func (s *Simulator) Receive(ctx context.Context, in bank.IncomingInput) (bank.Transfer, bank.Delivery, error) {
	t, err := s.bank.Incoming(in)
	if err != nil {
		return bank.Transfer{}, bank.Delivery{}, err
	}
	return t, s.deliver(ctx, t), nil
}

// Redeliver sends the webhook of a credit again with the SAME event id (the API must dedupe).
func (s *Simulator) Redeliver(ctx context.Context, transferID string) (bank.Delivery, error) {
	t, err := s.bank.GetTransfer(transferID)
	if err != nil || t.Type != bank.TypeCredit {
		return bank.Delivery{}, bank.ErrNotFound
	}
	return s.deliver(ctx, t), nil
}

func (s *Simulator) deliver(ctx context.Context, t bank.Transfer) bank.Delivery {
	ev := webhook.Event{
		EventID: s.bank.EventFor(t.ID), Type: webhook.TypeTransferReceived, TransferID: t.ID, OccurredAt: t.CreatedAt,
	}
	status, err := s.sender.Send(ctx, ev)
	d := bank.Delivery{EventID: ev.EventID, TransferID: t.ID, At: s.now(), Status: status}
	if err != nil {
		d.Error = err.Error()
		slog.WarnContext(ctx, "webhook not delivered", "transfer_id", t.ID, "status", status, "err", err.Error())
	}
	s.bank.RecordDelivery(d)
	return d
}

// RandomPayin pays a random active CVU a random amount from a random payer.
func (s *Simulator) RandomPayin(ctx context.Context) (bank.Transfer, bank.Delivery, error) {
	cvus := s.bank.ActiveCVUs()
	if len(cvus) == 0 {
		return bank.Transfer{}, bank.Delivery{}, errNoActiveCVU
	}
	// Demo randomness, never a secret (gosec G404 does not apply).
	caja := cvus[rand.IntN(len(cvus))]                //nolint:gosec // demo randomness
	amount := int64(1_000_00 + rand.IntN(249_000_00)) //nolint:gosec // demo randomness
	if caja.Currency == "USD" {
		amount = int64(10_00 + rand.IntN(1_990_00)) //nolint:gosec // demo randomness
	}
	amount -= amount % 100                                                                                    // round amounts never hit the scripted cents (.13, .96–.99)
	origin := bank.Counterparty{AccountNumber: bank.AccountNumberFor(strconv.Itoa(rand.IntN(1_000_000_000)))} //nolint:gosec // demo
	return s.Receive(ctx, bank.IncomingInput{CVU: caja.CVU, Amount: amount, Origin: origin})
}
