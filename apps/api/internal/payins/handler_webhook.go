package payins

import (
	"net/http"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/httpx"
)

// WebhookHandler serves POST /webhooks/psp/transfers (operationId receivePspTransferWebhook),
// hand-mounted because the signature covers the raw body (AG2, AG16). The guard has already
// applied the Signed rule; the body is capped by the body-limit middleware. It answers 501
// until lane C reads the raw body and calls Service.ReceiveWebhook.
type WebhookHandler struct {
	svc Service
}

// NewWebhookHandler returns the webhook handler.
func NewWebhookHandler(svc Service) *WebhookHandler { return &WebhookHandler{svc: svc} }

// ServeHTTP implements http.Handler.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = h.svc
	httpx.WriteError(w, r, apperr.NotImplemented())
}
