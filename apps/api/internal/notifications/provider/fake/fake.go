// Package fake is the log-only notifications.Sender for local runs and tests (P2): it never
// sends anything. It logs the template or kind, the recipient's e-mail domain and the data keys;
// the values (codes, invite tokens) only when the dev inbox is on (DEV_INBOX_ENABLED, R-ID-1,
// refused in production). It also keeps what it "sent" so tests can read it.
package fake

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"

	"lenapay/apps/api/internal/notifications"
)

// Sender is the fake sender. Safe for concurrent use.
type Sender struct {
	mu       sync.Mutex
	devInbox bool
	emails   []notifications.Email
	pushes   []notifications.Push
	failNext []error
}

var _ notifications.Sender = (*Sender)(nil)

// NewSender returns the fake sender; devInbox exposes e-mailed codes in the log (development).
func NewSender(devInbox bool) *Sender { return &Sender{devInbox: devInbox} }

// FailNext makes the next sends return these errors, in order.
func (s *Sender) FailNext(errs ...error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failNext = append(s.failNext, errs...)
}

// Emails returns the e-mails sent so far.
func (s *Sender) Emails() []notifications.Email {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.emails)
}

// Pushes returns the pushes sent so far.
func (s *Sender) Pushes() []notifications.Push {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.pushes)
}

func (s *Sender) popFailure() error {
	if len(s.failNext) == 0 {
		return nil
	}
	err := s.failNext[0]
	s.failNext = s.failNext[1:]
	return err
}

// SendEmail implements notifications.Sender.
func (s *Sender) SendEmail(ctx context.Context, e notifications.Email) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.popFailure(); err != nil {
		return err
	}
	s.emails = append(s.emails, e)
	_, domain, _ := strings.Cut(e.To, "@")
	attrs := []any{"template", e.Template, "to_domain", domain, "data_keys", slices.Sorted(maps.Keys(e.Data))}
	if s.devInbox {
		attrs = append(attrs, "dev_inbox", e.Data)
	}
	slog.InfoContext(ctx, "email (fake sender)", attrs...)
	return nil
}

// SendPush implements notifications.Sender.
func (s *Sender) SendPush(ctx context.Context, p notifications.Push) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.popFailure(); err != nil {
		return err
	}
	s.pushes = append(s.pushes, p)
	slog.InfoContext(ctx, "push (fake sender)", "kind", p.Kind, "devices", len(p.Tokens), "data", p.Data)
	return nil
}
