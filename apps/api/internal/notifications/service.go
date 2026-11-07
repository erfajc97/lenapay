package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/notifications/store"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
)

// Service is the notifications API used by the handler and by other features.
type Service interface {
	// RegisterPushToken stores (or moves) an Expo push token to the caller.
	RegisterPushToken(ctx context.Context, p auth.Principal, token string, platform Platform) error
	// Notify writes the outbox inside the caller's transaction and queues one delivery job per
	// entry in that same transaction: the push exists if and only if the fact committed.
	Notify(ctx context.Context, tx pgx.Tx, n Notification) error
	// Deliver is the job that pushes one outbox entry through the Sender (at least once).
	Deliver(ctx context.Context, notificationID uuid.UUID) error
	// RequeueStale re-queues pending entries older than age whose job was lost (periodic sweep).
	RequeueStale(ctx context.Context, age time.Duration) (int, error)
	// SendEmailCode implements identity.CodeSender.
	SendEmailCode(ctx context.Context, email, code string) error
	// SendInvite implements orgs.InviteMailer.
	SendInvite(ctx context.Context, email, orgName, token string) error
}

// Deps are the collaborators of the notifications service.
type Deps struct {
	Pool   *pgxpool.Pool
	Clock  clock.Clock
	Jobs   jobs.Enqueuer
	Sender Sender
}

type service struct {
	deps Deps
}

// NewService returns the notifications service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// Limits of the outbox (DB CHECKs and the delivery job's attempts).
const (
	tokenMax       = 200
	kindMax        = 60
	maxAttempts    = 10
	sweepBatchSize = 500
)

// Email templates (copy lives with the templates, never in code: G13).
const (
	TemplateEmailCode = "email_code"
	TemplateInvite    = "org_invite"
)

func (s *service) RegisterPushToken(ctx context.Context, p auth.Principal, token string, platform Platform) error {
	if token == "" || len(token) > tokenMax {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "token"})
	}
	if platform != PlatformIOS && platform != PlatformAndroid {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "platform"})
	}
	err := store.New(s.deps.Pool).UpsertPushToken(ctx, store.UpsertPushTokenParams{
		ID: ids.New(), UserID: p.UserID, Token: token, Platform: string(platform),
	})
	if err != nil {
		return fmt.Errorf("register push token: %w", err)
	}
	return nil
}

func (s *service) Notify(ctx context.Context, tx pgx.Tx, n Notification) error {
	if n.Kind == "" || len(n.Kind) > kindMax {
		return fmt.Errorf("notify: invalid kind %q", n.Kind)
	}
	data, err := json.Marshal(n.Data)
	if err != nil {
		return fmt.Errorf("notify: data: %w", err)
	}
	if n.Data == nil {
		data = []byte(`{}`)
	}
	q := store.New(tx)
	for _, userID := range n.UserIDs {
		id := ids.New()
		if err := q.InsertNotification(ctx, store.InsertNotificationParams{ID: id, OrgID: n.OrgID, UserID: userID, Kind: n.Kind, Data: data}); err != nil {
			return fmt.Errorf("notify: insert: %w", err)
		}
		if _, err := s.deps.Jobs.InsertTx(ctx, tx, DeliverArgs{NotificationID: id}, nil); err != nil {
			return fmt.Errorf("notify: queue delivery: %w", err)
		}
	}
	return nil
}

// Deliver pushes one pending entry to the user's devices, outside any transaction (the push is
// a network call). A send failure counts an attempt and returns the error so River retries;
// the entry turns failed after maxAttempts. No device is not an error: the entry is done.
func (s *service) Deliver(ctx context.Context, notificationID uuid.UUID) error {
	q := store.New(s.deps.Pool)
	n, err := q.GetNotification(ctx, notificationID)
	if db.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("deliver: read: %w", err)
	}
	if n.Status != "pending" {
		return nil // already sent or given up: River delivered the job twice
	}
	tokens, err := q.ListPushTokensForUser(ctx, n.UserID)
	if err != nil {
		return fmt.Errorf("deliver: tokens: %w", err)
	}
	if len(tokens) > 0 {
		push := Push{Kind: n.Kind, Data: map[string]string{}}
		_ = json.Unmarshal(n.Data, &push.Data) // written by Notify from a map[string]string
		for _, t := range tokens {
			push.Tokens = append(push.Tokens, t.Token)
		}
		if sendErr := s.deps.Sender.SendPush(ctx, push); sendErr != nil {
			if err := q.MarkNotificationAttemptFailed(ctx, store.MarkNotificationAttemptFailedParams{ID: n.ID, MaxAttempts: maxAttempts}); err != nil {
				return fmt.Errorf("deliver: count attempt: %w", err)
			}
			return fmt.Errorf("deliver: send: %w", sendErr)
		}
	}
	now := s.deps.Clock.Now()
	if _, err := q.MarkNotificationSent(ctx, store.MarkNotificationSentParams{ID: n.ID, SentAt: &now}); err != nil {
		return fmt.Errorf("deliver: mark sent: %w", err)
	}
	return nil
}

func (s *service) RequeueStale(ctx context.Context, age time.Duration) (int, error) {
	stale, err := store.New(s.deps.Pool).ListStalePendingNotifications(ctx, store.ListStalePendingNotificationsParams{
		CreatedBefore: s.deps.Clock.Now().Add(-age), PageLimit: sweepBatchSize,
	})
	if err != nil || len(stale) == 0 {
		return 0, err
	}
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		for _, id := range stale {
			// Unique by args: an entry whose job is still queued or retrying is not queued twice.
			if _, err := s.deps.Jobs.InsertTx(ctx, tx, DeliverArgs{NotificationID: id}, nil); err != nil {
				return fmt.Errorf("requeue %s: %w", id, err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(stale), nil
}

func (s *service) SendEmailCode(ctx context.Context, email, code string) error {
	return s.deps.Sender.SendEmail(ctx, Email{To: email, Template: TemplateEmailCode, Data: map[string]string{"code": code}})
}

func (s *service) SendInvite(ctx context.Context, email, orgName, token string) error {
	return s.deps.Sender.SendEmail(ctx, Email{To: email, Template: TemplateInvite, Data: map[string]string{"orgName": orgName, "token": token}})
}
