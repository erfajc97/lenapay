package payins

import (
	"io"
	"net/http"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/httpx"
)

// Headers of a signed PSP webhook (contract: receivePspTransferWebhook).
const (
	headerSignature = "X-Signature"
	headerTimestamp = "X-Signature-Timestamp"
)

// WebhookHandler serves POST /webhooks/psp/transfers (operationId receivePspTransferWebhook),
// hand-mounted because the signature covers the raw body (AG2, AG16). The guard has already
// applied the Signed rule and the body-limit middleware caps the body.
type WebhookHandler struct {
	svc Service
}

// NewWebhookHandler returns the webhook handler.
func NewWebhookHandler(svc Service) *WebhookHandler { return &WebhookHandler{svc: svc} }

// ServeHTTP implements http.Handler: 202 once the event is stored (or was already known).
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, r, err) // over the limit → request.too_large
		return
	}
	err = h.svc.ReceiveWebhook(r.Context(), WebhookDelivery{
		RawBody: body, Signature: r.Header.Get(headerSignature), Timestamp: r.Header.Get(headerTimestamp),
	})
	if err != nil {
		httpx.WriteError(w, r, err) // a 5xx makes the PSP redeliver
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
