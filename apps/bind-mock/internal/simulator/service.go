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

	"github.com/erfajc97/lenapay/apps/bind-mock/internal/bank"
	"github.com/erfajc97/lenapay/apps/bind-mock/internal/simulator/provider/webhook"
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

// onCredit delivers the webhook of an on-us credit. The bank calls it while locked, so the
// delivery runs on its own goroutine, after the outgoing transfer's answer, as at the bank.
func (s *Simulator) onCredit(t bank.Transfer) {
	s.onUs.Add(1)
	go func() {
		defer s.onUs.Done()
		s.deliver(s.base, t)
	}()
}

// Wait blocks until the on-us webhooks started so far were attempted (shutdown and tests).
func (s *Simulator) Wait() { s.onUs.Wait() }

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

// AutoStatus says whether random payins are running.
type AutoStatus struct {
	Enabled      bool `json:"enabled"`
	EverySeconds int  `json:"everySeconds"`
}

// Status returns the auto-payin state.
func (s *Simulator) Status() AutoStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return AutoStatus{Enabled: s.cancel != nil, EverySeconds: int(s.every / time.Second)}
}

// StartAuto starts (or restarts with a new interval) random payins until StopAuto or ctx ends.
func (s *Simulator) StartAuto(ctx context.Context, every time.Duration) {
	s.StopAuto()
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	s.cancel, s.every, s.stopped = cancel, every, make(chan struct{})
	go s.loop(ctx, every, s.stopped)
}

// StopAuto stops random payins and waits for the loop to end.
func (s *Simulator) StopAuto() {
	s.mu.Lock()
	cancel, stopped := s.cancel, s.stopped
	s.cancel, s.every, s.stopped = nil, 0, nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		<-stopped
	}
}

func (s *Simulator) loop(ctx context.Context, every time.Duration, stopped chan struct{}) {
	defer close(stopped)
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, _, err := s.RandomPayin(ctx); err != nil && !errors.Is(err, errNoActiveCVU) {
				slog.WarnContext(ctx, "auto payin failed", "err", err.Error())
			}
		}
	}
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
