package httpapi

import (
	"crypto/hmac"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/erfajc97/lenapay/apps/bind-mock/internal/bank"
	"github.com/erfajc97/lenapay/apps/bind-mock/internal/simulator"
	"github.com/erfajc97/lenapay/apps/bind-mock/internal/simulator/provider/webhook"
)

const secret = "test-webhook-secret-0123"

// receiver is the API side of the webhook: it records what arrived and checks the signature
// exactly as the contract describes it.
type receiver struct {
	mu     sync.Mutex
	events []webhook.Event
	badSig int
	status int
}

func (rc *receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	rc.mu.Lock()
	defer rc.mu.Unlock()
	want := webhook.Sign([]byte(secret), r.Header.Get(webhook.HeaderTimestamp), body)
	if !hmac.Equal([]byte(want), []byte(r.Header.Get(webhook.HeaderSignature))) {
		rc.badSig++
	}
	var ev webhook.Event
	_ = json.Unmarshal(body, &ev)
	rc.events = append(rc.events, ev)
	if rc.status == 0 {
		rc.status = http.StatusAccepted
	}
	w.WriteHeader(rc.status)
}

func (rc *receiver) received() []webhook.Event {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return append([]webhook.Event(nil), rc.events...)
}

type fixture struct {
	h    http.Handler
	bank *bank.Bank
	sim  *simulator.Simulator
	rcv  *receiver
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	now := func() time.Time { return time.Now().UTC() }
	b, err := bank.New(bank.Options{PendingDelay: time.Hour, Now: now, InitialBalanceMinor: map[string]int64{"ARS": 1_000_000_00, "USD": 1_000_00}})
	if err != nil {
		t.Fatal(err)
	}
	rcv := &receiver{}
	api := httptest.NewServer(rcv)
	t.Cleanup(api.Close)
	sim := simulator.New(t.Context(), b, webhook.NewSender(api.URL, []byte(secret), 2*time.Second, now), now)
	t.Cleanup(sim.StopAuto)
	return fixture{h: NewRouter(Deps{Bank: b, Simulator: sim, TimeoutDelay: 50 * time.Millisecond}), bank: b, sim: sim, rcv: rcv}
}

func (f fixture) do(t *testing.T, method, path, body string, headers ...string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(headers); i += 2 {
		r.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, r)
	var out map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec, out
}

func cajaBody(ref, currency string) string {
	return `{"referencia":"` + ref + `","moneda":"` + currency + `","titular":{"nombre":"Juan Pérez","cuit":"20123456786"},"requiereCvu":true}`
}

func TestHealth(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if rec, body := f.do(t, http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("status %d body %v", rec.Code, body)
	}
	if rec, _ := f.do(t, http.MethodPost, "/healthz", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /healthz status %d", rec.Code)
	}
}

func TestCajaLifecycle(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	path := PrefixCobro + "/cajas"
	rec, caja := f.do(t, http.MethodPost, path, cajaBody("ref-1", "ARS"), "Idempotency-Key", "ref-1")
	if rec.Code != http.StatusCreated || len(caja["cvu"].(string)) != 22 || caja["estado"] != bank.CajaActive {
		t.Fatalf("create: %d %v", rec.Code, caja)
	}
	rec, again := f.do(t, http.MethodPost, path, cajaBody("ref-1", "ARS"), "Idempotency-Key", "ref-1")
	if rec.Code != http.StatusOK || again["id"] != caja["id"] {
		t.Fatalf("replay: %d %v", rec.Code, again)
	}
	for name, tc := range map[string]struct{ body, key string }{
		"key differs from referencia": {cajaBody("ref-2", "ARS"), "other"},
		"unknown field":               {strings.Replace(cajaBody("ref-2", "ARS"), `"moneda"`, `"x":1,"moneda"`, 1), ""},
		"requiereCvu false":           {strings.Replace(cajaBody("ref-2", "ARS"), "true", "false", 1), ""},
		"bad currency":                {cajaBody("ref-2", "EUR"), ""},
	} {
		if rec, body := f.do(t, http.MethodPost, path, tc.body, "Idempotency-Key", tc.key); rec.Code != http.StatusBadRequest || body["codigo"] != codeInvalid {
			t.Errorf("%s: %d %v", name, rec.Code, body)
		}
	}
	id := caja["id"].(string)
	if rec, body := f.do(t, http.MethodPatch, path+"/"+id, `{"alias":"torre.norte"}`); rec.Code != http.StatusOK || body["alias"] != "torre.norte" {
		t.Fatalf("patch: %d %v", rec.Code, body)
	}
	_, other := f.do(t, http.MethodPost, path, cajaBody("ref-3", "USD"))
	if rec, body := f.do(t, http.MethodPatch, path+"/"+other["id"].(string), `{"alias":"TORRE.NORTE"}`); rec.Code != http.StatusConflict || body["codigo"] != codeAliasTaken {
		t.Fatalf("alias taken: %d %v", rec.Code, body)
	}
	if rec, _ := f.do(t, http.MethodDelete, path+"/"+id, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec, body := f.do(t, http.MethodGet, path+"/"+id, ""); rec.Code != http.StatusOK || body["estado"] != bank.CajaDisabled {
		t.Fatalf("get after delete: %d %v", rec.Code, body)
	}
	if rec, body := f.do(t, http.MethodDelete, path+"/caja-nope", ""); rec.Code != http.StatusNotFound || body["codigo"] != codeCajaNotFound {
		t.Fatalf("delete unknown: %d %v", rec.Code, body)
	}
}

func transferBody(origin, to, amount, currency string) string {
	return `{"origin_id":"` + origin + `","to":{"cbu":"` + to + `"},"value":{"currency":"` + currency + `","amount":` + amount +
		`},"description":"Pago","concept":"VAR","emails":["ops@lenapay.test"]}`
}

func TestTransfersReadBackAndScriptedFailures(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	to := bank.AccountNumberFor("4242")
	send := transferBase + "/transaction-requests"
	rec, tr := f.do(t, http.MethodPost, send, transferBody("o-1", to, "1500.25", "ARS"))
	charge := tr["charge"].(map[string]any)["value"].(map[string]any)
	if rec.Code != http.StatusOK || tr["status"] != bank.StatusCompleted || charge["amount"] != 1500.25 {
		t.Fatalf("send: %d %v", rec.Code, tr)
	}
	if !strings.Contains(rec.Body.String(), `"amount":1500.25`) {
		t.Fatalf("amount must be a JSON number with 2 decimals: %s", rec.Body)
	}
	for _, id := range []string{tr["id"].(string), "o-1"} { // BIND reads by its id or by origin_id
		if rec, got := f.do(t, http.MethodGet, transferBase+"/"+id, ""); rec.Code != http.StatusOK || got["id"] != tr["id"] {
			t.Fatalf("read %s: %d %v", id, rec.Code, got)
		}
	}
	if rec, got := f.do(t, http.MethodGet, transferBase+"/nope", ""); rec.Code != http.StatusUnprocessableEntity || got["codigo"] != codeTransferNotFound {
		t.Fatalf("unknown transfer: %d %v", rec.Code, got)
	}
	if rec, got := f.do(t, http.MethodPost, send, transferBody("o-2", to, "10.96", "ARS")); rec.Code != http.StatusServiceUnavailable || got["codigo"] != codeUnavailable {
		t.Fatalf(".96: %d %v", rec.Code, got)
	}
	if rec, _ := f.do(t, http.MethodPost, send, transferBody("o-3", to, "10.99", "ARS")); rec.Code != http.StatusInternalServerError {
		t.Fatalf(".99: %d", rec.Code)
	}
	if rec, got := f.do(t, http.MethodGet, transferBase+"/o-3", ""); rec.Code != http.StatusOK || got["status"] != bank.StatusCompleted {
		t.Fatalf("a 500 after executing still moved the money: %d %v", rec.Code, got)
	}
	if rec, _ := f.do(t, http.MethodPost, send, transferBody("o-4", to, "10.98", "ARS")); rec.Code != http.StatusGatewayTimeout {
		t.Fatalf(".98 (hang then 504): %d", rec.Code)
	}
	if rec, _ := f.do(t, http.MethodPost, "/_mock/next-transfer", `{"outcome":"pending"}`); rec.Code != http.StatusNoContent {
		t.Fatal("next-transfer")
	}
	_, pending := f.do(t, http.MethodPost, send, transferBody("o-5", to, "10", "ARS"))
	if pending["status"] != bank.StatusInProgress {
		t.Fatalf("scripted pending: %v", pending)
	}
	if rec, got := f.do(t, http.MethodPost, "/_mock/transfers/o-5/complete", `{"status":"FAILED"}`); rec.Code != http.StatusOK || got["status"] != bank.StatusFailed {
		t.Fatalf("complete: %d %v", rec.Code, got)
	}
	if rec, _ := f.do(t, http.MethodPost, send, transferBody("o-6", to, "10", "ARS")[:40]); rec.Code != http.StatusBadRequest {
		t.Fatal("truncated body")
	}
	if rec, _ := f.do(t, http.MethodPost, send, strings.Replace(transferBody("o-7", to, "10", "ARS"), `["ops@lenapay.test"]`, `[]`, 1)); rec.Code != http.StatusBadRequest {
		t.Fatal("empty emails must be rejected like BIND does")
	}
	if rec, _ := f.do(t, http.MethodPost, send, transferBody("o-8", to, "10.001", "ARS")); rec.Code != http.StatusBadRequest {
		t.Fatal("three decimals must be rejected")
	}
}

func TestResolveAndBalance(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	number := bank.AccountNumberFor("4242")
	rec, got := f.do(t, http.MethodGet, accountsBase+"/cbu/"+number, "")
	owner := got["owner"].(map[string]any)
	if rec.Code != http.StatusOK || owner["display_name"] == "" || got["account_routing"].(map[string]any)["address"] != number {
		t.Fatalf("resolve: %d %v", rec.Code, got)
	}
	if rec, got := f.do(t, http.MethodGet, accountsBase+"/alias/noexiste.alguien", ""); rec.Code != http.StatusNotFound || got["codigo"] != codeAccountNotFound {
		t.Fatalf("not found: %d %v", rec.Code, got)
	}
	if rec, _ := f.do(t, http.MethodGet, accountsBase+"/cbu/123", ""); rec.Code != http.StatusBadRequest {
		t.Fatal("invalid cbu")
	}
	if rec, got := f.do(t, http.MethodGet, PrefixCollect+"/Balance?currency=USD", ""); rec.Code != http.StatusOK || got["saldo"] != 1000.0 || got["moneda"] != "USD" {
		t.Fatalf("balance: %d %v", rec.Code, got)
	}
	if rec, _ := f.do(t, http.MethodGet, PrefixCollect+"/Balance?currency=EUR", ""); rec.Code != http.StatusBadRequest {
		t.Fatal("EUR balance")
	}
}

// The demo path: a payer pays a CVU, the API receives a signed webhook that only names the
// transfer, and the transfer reads back with the payer's data.
func TestIncomingTransferSendsASignedWebhook(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, caja := f.do(t, http.MethodPost, PrefixCobro+"/cajas", cajaBody("ref-1", "ARS"))
	cvu := caja["cvu"].(string)
	rec, got := f.do(t, http.MethodPost, "/_mock/incoming-transfer",
		`{"cvu":"`+cvu+`","amount":"150000.00","originName":"Ana Gómez","originCuit":"27111111110"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("incoming: %d %s", rec.Code, rec.Body)
	}
	tr := got["transfer"].(map[string]any)
	hook := got["webhook"].(map[string]any)
	if hook["status"] != float64(http.StatusAccepted) || hook["error"] != nil {
		t.Fatalf("webhook delivery %v", hook)
	}
	events := f.rcv.received()
	if len(events) != 1 || events[0].TransferID != tr["id"] || events[0].Type != webhook.TypeTransferReceived || events[0].EventID == "" || f.rcv.badSig != 0 {
		t.Fatalf("events %+v badSig %d", events, f.rcv.badSig)
	}
	rec, list := f.do(t, http.MethodGet, accountsBase+"/transfers?type=CREDIT&transferenceId="+tr["id"].(string), "")
	items := list["items"].([]any)
	if rec.Code != http.StatusOK || len(items) != 1 || items[0].(map[string]any)["cvu"] != cvu ||
		items[0].(map[string]any)["counterparty"].(map[string]any)["name"] != "Ana Gómez" {
		t.Fatalf("listing: %d %v", rec.Code, list)
	}
	// A redelivery repeats the SAME event id: the API must dedupe it.
	if rec, _ := f.do(t, http.MethodPost, "/_mock/webhooks/"+tr["id"].(string)+"/redeliver", ""); rec.Code != http.StatusOK {
		t.Fatalf("redeliver: %d", rec.Code)
	}
	if events := f.rcv.received(); len(events) != 2 || events[1].EventID != events[0].EventID {
		t.Fatalf("redelivery event ids %+v", events)
	}
	if rec, got := f.do(t, http.MethodPost, "/_mock/incoming-transfer", `{"cvu":"`+bank.AccountNumberFor("1")+`","amount":"1"}`); rec.Code != http.StatusNotFound || got["codigo"] != codeCajaNotFound {
		t.Fatalf("unknown cvu: %d %v", rec.Code, got)
	}
	if rec, _ := f.do(t, http.MethodPost, "/_mock/incoming-transfer", `{"cvu":"`+cvu+`","amount":"1.001"}`); rec.Code != http.StatusBadRequest {
		t.Fatal("three decimals")
	}
	if rec, _ := f.do(t, http.MethodGet, accountsBase+"/transfers?fromDate=yesterday", ""); rec.Code != http.StatusBadRequest {
		t.Fatal("bad date")
	}
	_, state := f.do(t, http.MethodGet, "/_mock/state", "")
	if len(state["deliveries"].([]any)) != 2 {
		t.Fatalf("state %v", state)
	}
}

// A webhook the API refuses is recorded; the money stays (the provider retries later).
func TestWebhookFailureIsRecorded(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.rcv.status = http.StatusServiceUnavailable
	_, caja := f.do(t, http.MethodPost, PrefixCobro+"/cajas", cajaBody("ref-1", "USD"))
	rec, got := f.do(t, http.MethodPost, "/_mock/incoming-transfer", `{"alias":"`+caja["alias"].(string)+`","amount":"25"}`)
	hook := got["webhook"].(map[string]any)
	if rec.Code != http.StatusCreated || hook["status"] != float64(http.StatusServiceUnavailable) || hook["error"] == nil {
		t.Fatalf("%d %v", rec.Code, got)
	}
}

func TestAutoPayins(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if rec, _ := f.do(t, http.MethodPost, "/_mock/auto-payins", `{"enabled":true,"everySeconds":0}`); rec.Code != http.StatusBadRequest {
		t.Fatal("0 seconds must be rejected")
	}
	if rec, _ := f.do(t, http.MethodPost, "/_mock/auto-payins", `{"enabled":true,"everySeconds":3601}`); rec.Code != http.StatusBadRequest {
		t.Fatal("3601 seconds must be rejected")
	}
	f.do(t, http.MethodPost, PrefixCobro+"/cajas", cajaBody("ref-1", "ARS"))
	if rec, got := f.do(t, http.MethodPost, "/_mock/auto-payins", `{"enabled":true,"everySeconds":1}`); rec.Code != http.StatusOK || got["enabled"] != true {
		t.Fatalf("start: %d %v", rec.Code, got)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(f.rcv.received()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no auto payin within 5s")
		}
		pause()
	}
	if rec, got := f.do(t, http.MethodPost, "/_mock/auto-payins", `{"enabled":false}`); rec.Code != http.StatusOK || got["enabled"] != false {
		t.Fatalf("stop: %d %v", rec.Code, got)
	}
	if _, got := f.do(t, http.MethodGet, "/_mock/auto-payins", ""); got["enabled"] != false {
		t.Fatal("status after stop")
	}
	if rec, _ := f.do(t, http.MethodPost, "/_mock/reset", ""); rec.Code != http.StatusNoContent || len(f.bank.Snapshot().Cajas) != 0 {
		t.Fatal("reset")
	}
}

// pause waits one polling interval (polling with a deadline, never a fixed sleep: AG18).
func pause() { <-time.After(10 * time.Millisecond) }

// On-us over HTTP: a transfer to one of our CVUs (with origin_debit) answers like any debit,
// and the receiving CVU's webhook reaches the API with the credit's id.
func TestTransferToOwnCVUSendsTheIncomingWebhook(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, company := f.do(t, http.MethodPost, PrefixCobro+"/cajas", cajaBody("ref-company", "ARS"))
	_, person := f.do(t, http.MethodPost, PrefixCobro+"/cajas", cajaBody("ref-person", "ARS"))
	body := strings.Replace(transferBody("o-1", company["cvu"].(string), "1500", "ARS"), `"origin_id":"o-1"`,
		`"origin_id":"o-1","origin_debit":{"cvu":"`+person["cvu"].(string)+`"}`, 1)
	rec, debit := f.do(t, http.MethodPost, transferBase+"/transaction-requests", body)
	if rec.Code != http.StatusOK || debit["status"] != bank.StatusCompleted {
		t.Fatalf("send: %d %v", rec.Code, debit)
	}
	f.sim.Wait()
	events := f.rcv.received()
	if len(events) != 1 || f.rcv.badSig != 0 {
		t.Fatalf("events %+v badSig %d", events, f.rcv.badSig)
	}
	rec, credit := f.do(t, http.MethodGet, transferBase+"/"+events[0].TransferID, "")
	if rec.Code != http.StatusOK || credit["type"] != bank.TypeCredit || credit["cvu"] != company["cvu"] ||
		credit["counterparty"].(map[string]any)["cbu"] != person["cvu"] {
		t.Fatalf("credit: %d %v", rec.Code, credit)
	}
	malformed := strings.Replace(strings.Replace(body, `"o-1"`, `"o-2"`, 1), person["cvu"].(string), "123", 1)
	if rec, _ := f.do(t, http.MethodPost, transferBase+"/transaction-requests", malformed); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed origin_debit: %d", rec.Code)
	}
}
