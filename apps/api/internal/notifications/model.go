package notifications

import "github.com/google/uuid"

// Platform of a push token.
type Platform string

// Push platforms.
const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

// Notification is one outbox entry per user (ids and enums in Data, never PII).
type Notification struct {
	OrgID   *uuid.UUID
	UserIDs []uuid.UUID
	Kind    string // e.g. payin.credited, approval.requested, payout.settled
	Data    map[string]string
}
