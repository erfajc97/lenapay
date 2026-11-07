// Package fake is the log-only notifications.Sender for local runs and tests (P2): it never
// sends anything. Lane C makes it log ids and template names (and the code itself only when
// DEV_INBOX_ENABLED, R-ID-1).
package fake

import (
	"context"
	"sync"

	"github.com/erfajc97/lenapay/apps/api/internal/notifications"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// Sender is the fake sender.
type Sender struct {
	mu       sync.Mutex
	devInbox bool
}

var _ notifications.Sender = (*Sender)(nil)

// NewSender returns the fake sender; devInbox exposes e-mailed codes in the log (development).
func NewSender(devInbox bool) *Sender { return &Sender{devInbox: devInbox} }

func (s *Sender) stub() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.devInbox
	return apperr.NotImplemented()
}

// SendEmail implements notifications.Sender.
func (s *Sender) SendEmail(context.Context, notifications.Email) error { return s.stub() }

// SendPush implements notifications.Sender.
func (s *Sender) SendPush(context.Context, notifications.Push) error { return s.stub() }
