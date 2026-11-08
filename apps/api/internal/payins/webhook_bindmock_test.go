//go:build bindmock

// Cross-module contract test: the webhook apps/bind-mock signs is the one this package
// verifies, and the transfer it announces re-reads through the BIND adapter. Run with the mock
// up and its webhook pointed at this test's listener:
//
//	BINDMOCK_HTTP_ADDR=127.0.0.1:18090 BINDMOCK_WEBHOOK_URL=http://127.0.0.1:18099/hook go run ./cmd/bind-mock   (apps/bind-mock)
//	BINDMOCK_URL=http://127.0.0.1:18090 BINDMOCK_WEBHOOK_LISTEN=127.0.0.1:18099 go test -tags=bindmock ./internal/payins/
package payins

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
	"lenapay/apps/api/internal/psp/provider/bind"
)

func TestBindMockWebhookVerifies(t *testing.T) {
	base, listen := os.Getenv("BINDMOCK_URL"), os.Getenv("BINDMOCK_WEBHOOK_LISTEN")
	if base == "" || listen == "" {
		t.Skip("BINDMOCK_URL / BINDMOCK_WEBHOOK_LISTEN not set")
	}
	secret := os.Getenv("PSP_WEBHOOK_SECRET")
	if secret == "" {
		secret = "dev-only-psp-webhook-secret" // both processes' development default
	}
	deliveries := make(chan WebhookDelivery, 1)
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		deliveries <- WebhookDelivery{RawBody: body, Signature: r.Header.Get(headerSignature), Timestamp: r.Header.Get(headerTimestamp)}
		w.WriteHeader(http.StatusAccepted)
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	adapter := bind.New(bind.Options{BaseURL: base, Timeout: 5 * time.Second})
	cvu, err := adapter.CreateCVU(t.Context(), psp.CreateCVURequest{Reference: uuid.NewString(), Currency: money.USD, HolderName: "Desarrollos Norte SA"})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"cvu": cvu.CVU, "amount": "250.75", "originName": "Ana Gómez"})
	resp, err := http.Post(base+"/_mock/incoming-transfer", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Transfer struct {
			ID string `json:"id"`
		} `json:"transfer"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()

	select {
	case d := <-deliveries:
		if err := verifySignature([]byte(secret), d, time.Now(), 5*time.Minute); err != nil {
			t.Fatalf("signature: %v", err)
		}
		ev, ok := decodeEvent(d.RawBody)
		if !ok || ev.TransferID != out.Transfer.ID {
			t.Fatalf("event %+v ok=%v", ev, ok)
		}
		got, err := adapter.GetTransfer(t.Context(), ev.TransferID)
		if err != nil || got.CVU != cvu.CVU || got.Amount != money.New(25075, money.USD) || got.OriginName != "Ana Gómez" {
			t.Fatalf("re-read %+v %v", got, err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no webhook within 10s")
	}
}
