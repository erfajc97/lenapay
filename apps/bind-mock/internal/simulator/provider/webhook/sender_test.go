package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The signature is the contract's: sha256=hex(HMAC(secret, timestamp + "." + raw body)).
func TestSignMatchesTheContract(t *testing.T) {
	t.Parallel()
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte("1700000000.{}"))
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got := Sign([]byte("secret"), "1700000000", []byte("{}")); got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if Sign([]byte("secret"), "1700000001", []byte("{}")) == want || Sign([]byte("other"), "1700000000", []byte("{}")) == want {
		t.Fatal("timestamp and secret must change the signature")
	}
}

func TestSendSignsTheExactBytes(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0).UTC()
	var gotSig, gotTS string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig, gotTS = r.Header.Get(HeaderSignature), r.Header.Get(HeaderTimestamp)
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)
	s := NewSender(srv.URL, []byte("secret"), time.Second, func() time.Time { return now })
	status, err := s.Send(t.Context(), Event{EventID: "evt_1", Type: TypeTransferReceived, TransferID: "trf-1", OccurredAt: now})
	if err != nil || status != http.StatusAccepted || gotTS != "1700000000" || gotSig != Sign([]byte("secret"), gotTS, gotBody) {
		t.Fatalf("status %d err %v ts %s sig %s", status, err, gotTS, gotSig)
	}
}

func TestSendReportsRefusalsAndUnreachableAPIs(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	t.Cleanup(srv.Close)
	s := NewSender(srv.URL, []byte("secret"), time.Second, time.Now)
	if status, err := s.Send(t.Context(), Event{}); err == nil || status != http.StatusUnauthorized {
		t.Fatalf("refused: %d %v", status, err)
	}
	srv.Close()
	if status, err := s.Send(t.Context(), Event{}); err == nil || status != 0 {
		t.Fatalf("unreachable: %d %v", status, err)
	}
}
