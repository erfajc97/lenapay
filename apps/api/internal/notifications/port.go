package notifications

import "context"

// Sender delivers e-mails and pushes (provider/fake logs them; SES / Expo push later).
type Sender interface {
	SendEmail(ctx context.Context, e Email) error
	SendPush(ctx context.Context, p Push) error
}

// Email is a templated e-mail; copy lives in the template, never in code.
type Email struct {
	To       string
	Template string
	Data     map[string]string
}

// Push is one push message to some device tokens.
type Push struct {
	Tokens []string
	Kind   string
	Data   map[string]string
}
