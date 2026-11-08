package simulator

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"lenapay/apps/bind-mock/internal/bank"
)

// The simulator's background work: the on-us webhook deliveries (tracked by Wait) and the
// auto-payin loop (StartAuto / StopAuto, which waits for it to end).

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
