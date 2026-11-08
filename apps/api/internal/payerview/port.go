package payerview

import (
	"context"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/erplink"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
)

// ERP answers whether companies are ERP-linked and a buyer's installments (erplink.Service).
type ERP interface {
	LinkedOrgs(ctx context.Context, orgIDs []uuid.UUID) (map[uuid.UUID]bool, error)
	BuyerInstallments(ctx context.Context, orgID uuid.UUID, buyerRef *string) (bool, []erplink.Installment, error)
}

// Payouts creates the person's payout from the personal account (payouts.Service).
type Payouts interface {
	Create(ctx context.Context, p auth.Principal, in payouts.CreateInput) (payouts.Payout, error)
}
