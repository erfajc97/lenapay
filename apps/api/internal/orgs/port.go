package orgs

import (
	"context"

	"github.com/google/uuid"
)

// Users finds registered users (implemented by identity.Service).
type Users interface {
	UserIDByEmail(ctx context.Context, email string) (uuid.UUID, error)
}

// InviteMailer emails invitation links (implemented by notifications.Service).
type InviteMailer interface {
	SendInvite(ctx context.Context, email, orgName, token string) error
}
