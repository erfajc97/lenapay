package accounts

import (
	"context"

	"github.com/google/uuid"
)

// ReceivingCVUs lists the account-owned CVUs of an org (implemented by payers.Service, which
// owns the cvus table, P5).
type ReceivingCVUs interface {
	ReceivingCVUs(ctx context.Context, orgID uuid.UUID) ([]ReceivingCVU, error)
}

// ReceivingCVUsFunc adapts a function to ReceivingCVUs. cmd/api uses it to break the
// accounts ↔ payers construction cycle (payers needs accounts.Service first).
type ReceivingCVUsFunc func(ctx context.Context, orgID uuid.UUID) ([]ReceivingCVU, error)

// ReceivingCVUs implements ReceivingCVUs.
func (f ReceivingCVUsFunc) ReceivingCVUs(ctx context.Context, orgID uuid.UUID) ([]ReceivingCVU, error) {
	return f(ctx, orgID)
}
