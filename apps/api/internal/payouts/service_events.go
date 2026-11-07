package payouts

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/erfajc97/lenapay/apps/api/internal/notifications"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/realtime"
)

// Realtime event types of the contract (RealtimeEventType).
const (
	eventPayoutStatusChanged = "payout.status_changed"
	eventApprovalRequested   = "approval.requested"
	eventBalanceChanged      = "balance.changed"
)

// publish queues an org event inside tx; Postgres delivers it at commit (ids only).
func publish(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, typ string, payoutID uuid.UUID, status Status) error {
	return publishData(ctx, tx, orgID, typ, map[string]string{"payoutId": payoutID.String(), "status": string(status)})
}

func publishBalance(ctx context.Context, tx pgx.Tx, orgID, accountID uuid.UUID) error {
	return publishData(ctx, tx, orgID, eventBalanceChanged, map[string]string{"accountId": accountID.String()})
}

func publishData(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, typ string, data map[string]string) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("payouts: event data: %w", err)
	}
	return realtime.Publish(ctx, tx, orgID, realtime.Event{Type: typ, Data: raw})
}

// notificationFor builds the outbox entry of a payout fact (ids and enums only, never PII).
func notificationFor(orgID uuid.UUID, users []uuid.UUID, kind string, payoutID uuid.UUID, status Status) notifications.Notification {
	return notifications.Notification{
		OrgID: &orgID, UserIDs: users, Kind: kind,
		Data: map[string]string{"payoutId": payoutID.String(), "status": string(status)},
	}
}
