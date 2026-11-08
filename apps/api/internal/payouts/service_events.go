package payouts

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/notifications"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts/store"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/realtime"
)

// Realtime event types of the contract (RealtimeEventType).
const (
	eventPayoutUpdated     = "payout.updated"
	eventApprovalRequested = "approval.requested"
	eventBalanceChanged    = "balance.changed"
)

// maxEventBytes keeps a payout event under the NOTIFY payload limit with room for the
// envelope; a bigger payout (many long attachment names) is sent without its lists.
const maxEventBytes = 6000

// payoutEvent is the data of payout.updated and approval.requested: ids for routing and the
// payout as the contract's Payout, so the app updates its cache without a refetch.
type payoutEvent struct {
	PayoutID uuid.UUID  `json:"payoutId"`
	Status   Status     `json:"status"`
	Payout   api.Payout `json:"payout"`
}

// publishPayout reads the payout inside tx (its new state, uncommitted) and queues typ for its
// org; Postgres delivers it at commit only.
func publishPayout(ctx context.Context, tx pgx.Tx, typ string, payoutID, orgID uuid.UUID) error {
	po, err := load(ctx, store.New(tx), payoutID, orgID)
	if err != nil {
		return err
	}
	ev := payoutEvent{PayoutID: po.ID, Status: po.Status, Payout: PayoutToAPI(po)}
	raw, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("payouts: event data: %w", err)
	}
	if len(raw) > maxEventBytes {
		ev.Payout.Events, ev.Payout.Attachments = []api.PayoutEvent{}, []api.PayoutAttachment{}
		if raw, err = json.Marshal(ev); err != nil {
			return fmt.Errorf("payouts: event data: %w", err)
		}
	}
	return realtime.Publish(ctx, tx, orgID, realtime.Event{Type: typ, Data: raw})
}

func publishBalance(ctx context.Context, tx pgx.Tx, orgID, accountID uuid.UUID) error {
	raw, err := json.Marshal(map[string]string{"accountId": accountID.String()})
	if err != nil {
		return fmt.Errorf("payouts: event data: %w", err)
	}
	return realtime.Publish(ctx, tx, orgID, realtime.Event{Type: eventBalanceChanged, Data: raw})
}

// notificationFor builds the outbox entry of a payout fact (ids and enums only, never PII).
func notificationFor(orgID uuid.UUID, users []uuid.UUID, kind string, payoutID uuid.UUID, status Status) notifications.Notification {
	return notifications.Notification{
		OrgID: &orgID, UserIDs: users, Kind: kind,
		Data: map[string]string{"payoutId": payoutID.String(), "status": string(status)},
	}
}
