//go:build integration

package payins_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"lenapay/apps/api/internal/accounts"
	"lenapay/apps/api/internal/fees"
	"lenapay/apps/api/internal/ledger"
	"lenapay/apps/api/internal/notifications"
	notifyfake "lenapay/apps/api/internal/notifications/provider/fake"
	"lenapay/apps/api/internal/orgs"
	"lenapay/apps/api/internal/payers"
	"lenapay/apps/api/internal/payins"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/audit"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/config"
	"lenapay/apps/api/internal/platform/db/dbtest"
	"lenapay/apps/api/internal/platform/idempotency"
	"lenapay/apps/api/internal/platform/ids"
	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/platform/realtime"
	"lenapay/apps/api/internal/psp"
	pspfake "lenapay/apps/api/internal/psp/provider/fake"
)

var (
	pg      *dbtest.Postgres
	cuitSeq atomic.Int64
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

const secret = "webhook-secret-0123456789"

type recorder struct {
	mu   sync.Mutex
	jobs []river.JobArgs
}

func (r *recorder) InsertTx(_ context.Context, _ pgx.Tx, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, args)
	return &rivertype.JobInsertResult{}, nil
}

func (r *recorder) kinds() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]int{}
	for _, j := range r.jobs {
		out[j.Kind()]++
	}
	return out
}

// directory is orgs.Get / ListMembers over the rows the test inserted.
type directory struct{ pool *pgxpool.Pool }

func (d directory) Get(ctx context.Context, p auth.Principal) (orgs.Organization, error) {
	var o orgs.Organization
	err := d.pool.QueryRow(ctx, `SELECT id, legal_name, cuit FROM organizations WHERE id = $1`, p.OrgID).Scan(&o.ID, &o.LegalName, &o.CUIT)
	return o, err
}

func (d directory) ListMembers(ctx context.Context, p auth.Principal) ([]orgs.Member, error) {
	rows, err := d.pool.Query(ctx, `SELECT user_id FROM memberships WHERE org_id = $1 ORDER BY user_id`, p.OrgID)
	if err != nil {
		return nil, err
	}
	users, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	out := make([]orgs.Member, 0, len(users))
	for _, u := range users {
		out = append(out, orgs.Member{UserID: u})
	}
	return out, err
}

type env struct {
	pool    *pgxpool.Pool
	clk     *clock.Fake
	svc     payins.Service
	ledger  ledger.Service
	psp     *pspfake.Provider
	jobs    *recorder
	p       auth.Principal
	other   auth.Principal
	account uuid.UUID
	payer   payers.Payer
	cvu     payers.CVU
}

func setup(t *testing.T) env {
	t.Helper()
	ctx := t.Context()
	pool := pg.NewDatabase(t)
	clk := clock.NewFake(time.Date(2026, 11, 7, 15, 0, 0, 0, time.UTC))
	led := ledger.NewService(ledger.Deps{Pool: pool, Clock: clk})
	feeSvc := fees.NewService(fees.Deps{Pool: pool})
	if err := led.EnsureSystemAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if err := feeSvc.EnsureDefaultPlan(ctx); err != nil {
		t.Fatal(err)
	}
	idem := idempotency.NewStore(clk, idempotency.DefaultTTL)
	acc := accounts.NewService(accounts.Deps{Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Idempotency: idem, Ledger: led})
	e := env{pool: pool, clk: clk, ledger: led, psp: pspfake.New(), jobs: &recorder{}}
	dir := directory{pool: pool}
	payerSvc := payers.NewService(payers.Deps{
		Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Idempotency: idem, Jobs: e.jobs, PSP: e.psp,
		Accounts: acc, Orgs: dir, ShareBaseURL: "https://lenapay.test/pagar",
	})
	notifySvc := notifications.NewService(notifications.Deps{Pool: pool, Clock: clk, Jobs: e.jobs, Sender: notifyfake.NewSender(false)})
	e.svc = payins.NewService(payins.Deps{
		Pool: pool, Clock: clk, Audit: audit.NewLogger(clk), Jobs: e.jobs, PSP: e.psp, Payers: payerSvc, Accounts: acc,
		Members: dir, Ledger: led, Fees: feeSvc, Notifications: notifySvc,
		WebhookSecret: config.Secret(secret), Tolerance: 5 * time.Minute,
	})
	e.p, e.other = e.newOrg(t, 2), e.newOrg(t, 1)
	a, err := acc.Create(ctx, e.p, accounts.CreateInput{Name: "Torre Palermo"})
	if err != nil {
		t.Fatal(err)
	}
	e.account = a.ID
	if e.payer, err = payerSvc.Create(ctx, e.p, payers.CreateInput{AccountID: a.ID, Name: "Juan Pérez", DocType: payers.DocDNI, DocNumber: "30111222"}); err != nil {
		t.Fatal(err)
	}
	if e.cvu, err = payerSvc.CreateCVU(ctx, e.p, e.payer.ID, payers.CreateCVUInput{Currency: money.ARS, IdempotencyKey: "client-key-0000000001"}); err != nil {
		t.Fatal(err)
	}
	e.jobs.jobs = nil
	return e
}

// newOrg creates an approved company with n members (all of them are notified of a payin).
func (e env) newOrg(t *testing.T, members int) auth.Principal {
	t.Helper()
	org := ids.New()
	var first uuid.UUID
	for i := range members {
		user := ids.New()
		e.exec(t, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`, user, user.String()+"@t.local")
		if i == 0 {
			first = user
			e.exec(t, `INSERT INTO organizations (id, kind, legal_name, cuit, kyc_status, created_by) VALUES ($1, 'company', 'Desarrollos Norte SA', $2, 'approved', $3)`,
				org, fmt.Sprintf("30%09d", cuitSeq.Add(1)), user)
		}
		e.exec(t, `INSERT INTO memberships (org_id, user_id, role) VALUES ($1, $2, 'admin')`, org, user)
	}
	return auth.Principal{UserID: first, SessionID: ids.New(), OrgID: org, Role: auth.RoleAdmin, KYCStatus: auth.KYCApproved}
}

func (e env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (e env) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func (e env) delivery(eventID, transferID string) payins.WebhookDelivery {
	body := []byte(`{"eventId":"` + eventID + `","type":"transfer.received","transferId":"` + transferID + `","occurredAt":"2026-11-07T15:00:00Z"}`)
	ts := strconv.FormatInt(e.clk.Now().Unix(), 10)
	return payins.WebhookDelivery{RawBody: body, Timestamp: ts, Signature: sign(ts, body)}
}

// sign is the PSP's signature: sha256=hex(HMAC-SHA256(secret, timestamp + "." + body)).
func sign(ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// incoming registers a transfer at the fake PSP, as a payer paying the CVU would.
func (e env) incoming(id string, minor int64) psp.IncomingTransfer {
	t := psp.IncomingTransfer{
		ID: id, CVU: e.cvu.Number, Amount: money.New(minor, money.ARS), OriginName: "Ana Gómez",
		OriginCUIT: "27111111110", OriginCBU: "0070999800000000000425", OccurredAt: e.clk.Now(),
	}
	e.psp.AddIncoming(t)
	return t
}

// AG16: a signed event is stored once and queued once; a redelivery queues nothing.
func TestReceiveWebhookStoresOnceAndQueues(t *testing.T) {
	t.Parallel()
	e := setup(t)
	d := e.delivery("evt_1", "trf-1")
	for range 2 {
		if err := e.svc.ReceiveWebhook(t.Context(), d); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM psp_webhook_events WHERE provider_event_id = 'evt_1' AND signature_ok AND processed_at IS NULL`); n != 1 {
		t.Fatalf("events = %d", n)
	}
	if k := e.jobs.kinds(); k["payins.process"] != 1 || len(e.jobs.jobs) != 1 {
		t.Fatalf("jobs %v", k)
	}
	args := e.jobs.jobs[0].(payins.ProcessPayinArgs)
	if args.TransferID != "trf-1" || args.EventID == uuid.Nil {
		t.Fatalf("args %+v", args)
	}
	forged := d
	forged.RawBody = []byte(`{"eventId":"evt_2","type":"transfer.received","transferId":"trf-1","occurredAt":"2026-11-07T15:00:00Z"}`)
	if err := e.svc.ReceiveWebhook(t.Context(), forged); !apperr.IsCode(err, apperr.CodeWebhookSignature) {
		t.Fatalf("forged body: %v", err)
	}
	invalid := e.delivery("evt_3", "")
	if err := e.svc.ReceiveWebhook(t.Context(), invalid); !apperr.IsCode(err, apperr.CodeValidation) {
		t.Fatalf("invalid body: %v", err)
	}
	if n := e.count(t, `SELECT count(*) FROM psp_webhook_events`); n != 1 {
		t.Fatalf("a refused webhook was stored: %d", n)
	}
}

// R-IN-1/2, MO4: the credit uses the PSP's amount and CVU, net of the fee, posted once; the
// payin, the outbox, the realtime event and the event's settlement commit together.
func TestProcessTransferCreditsOnce(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := t.Context()
	listener, err := e.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Release()
	if _, err := listener.Exec(ctx, "LISTEN "+realtime.Channel); err != nil {
		t.Fatal(err)
	}

	gross := int64(15_000_050) // ARS 150.000,50
	e.incoming("trf-1", gross)
	if err := e.svc.ReceiveWebhook(ctx, e.delivery("evt_1", "trf-1")); err != nil {
		t.Fatal(err)
	}
	eventID := e.jobs.jobs[0].(payins.ProcessPayinArgs).EventID
	if err := e.svc.ProcessTransfer(ctx, eventID, "trf-1"); err != nil {
		t.Fatal(err)
	}
	fee, err := fees.Compute(fees.OperationPayin, money.New(gross, money.ARS), fees.DefaultRates()[money.ARS])
	if err != nil {
		t.Fatal(err)
	}
	var payinID uuid.UUID
	var feeMinor, netMinor int64
	var status string
	err = e.pool.QueryRow(ctx, `SELECT id, fee_minor, net_minor, status FROM payins WHERE provider_transfer_id = 'trf-1' AND org_id = $1 AND payer_id = $2 AND origin_cuit = '27111111110'`,
		e.p.OrgID, e.payer.ID).Scan(&payinID, &feeMinor, &netMinor, &status)
	if err != nil || status != "credited" || feeMinor != fee.Minor || netMinor != gross-fee.Minor || fee.Minor == 0 {
		t.Fatalf("payin fee %d net %d status %s err %v (want fee %d)", feeMinor, netMinor, status, err, fee.Minor)
	}
	balances, err := e.ledger.Balances(ctx, []uuid.UUID{e.account})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range balances {
		if b.Posted.Currency == money.ARS && b.Posted.Minor != netMinor {
			t.Fatalf("account posted %d, want net %d", b.Posted.Minor, netMinor)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM notifications WHERE kind = 'payin.credited' AND org_id = $1 AND data->>'payinId' = $2`, e.p.OrgID, payinID.String()); n != 2 {
		t.Fatalf("notifications = %d, want one per member (2)", n)
	}
	if k := e.jobs.kinds(); k["notifications.deliver"] != 2 {
		t.Fatalf("delivery jobs %v", k)
	}
	if n := e.count(t, `SELECT count(*) FROM psp_webhook_events WHERE id = $1 AND processed_at IS NOT NULL AND error_code IS NULL`, eventID); n != 1 {
		t.Fatal("event not settled")
	}
	if n := e.count(t, `SELECT count(*) FROM audit_log WHERE action = 'payin.credit' AND target_id = $1 AND actor_kind = 'psp'`, payinID); n != 1 {
		t.Fatal("credit not audited")
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	n, err := listener.Conn().WaitForNotification(waitCtx)
	if err != nil {
		t.Fatalf("no realtime event: %v", err)
	}
	var env struct {
		OrgID uuid.UUID `json:"orgId"`
		Event struct {
			Type string `json:"type"`
			Data struct {
				PayinID uuid.UUID `json:"payinId"`
				Payin   struct {
					PayerName, AccountName string
					Amount                 int64
				} `json:"payin"`
			} `json:"data"`
		} `json:"event"`
	}
	if err := json.Unmarshal([]byte(n.Payload), &env); err != nil || env.OrgID != e.p.OrgID || env.Event.Type != payins.EventPayinCredited ||
		env.Event.Data.PayinID != payinID || env.Event.Data.Payin.PayerName != "Juan Pérez" || env.Event.Data.Payin.AccountName != "Torre Palermo" ||
		env.Event.Data.Payin.Amount != gross {
		t.Fatalf("event %s (%v)", n.Payload, err)
	}

	// A second event for the same transfer (another event id) credits nothing more.
	if err := e.svc.ProcessTransfer(ctx, uuid.Nil, "trf-1"); err != nil {
		t.Fatal(err)
	}
	if c := e.count(t, `SELECT count(*) FROM payins`); c != 1 {
		t.Fatalf("payins = %d", c)
	}
	if c := e.count(t, `SELECT count(*) FROM ledger_transactions WHERE kind = 'payin'`); c != 1 {
		t.Fatalf("ledger transactions = %d", c)
	}

	got, err := e.svc.Get(ctx, e.p, payinID)
	if err != nil || got.Gross.Minor != gross || got.Net.Minor != netMinor || got.Status != payins.StatusCredited || got.CreditedAt == nil {
		t.Fatalf("get %+v %v", got, err)
	}
	if _, err := e.svc.Get(ctx, e.other, payinID); !apperr.IsCode(err, apperr.CodePayinNotFound) {
		t.Fatalf("foreign payin: %v", err)
	}
	day := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	if list, err := e.svc.CreditedBetween(ctx, money.ARS, day, day.AddDate(0, 0, 1)); err != nil || len(list) != 1 {
		t.Fatalf("credited between %d %v", len(list), err)
	}
	if list, _ := e.svc.CreditedBetween(ctx, money.USD, day, day.AddDate(0, 0, 1)); len(list) != 0 {
		t.Fatal("USD has no payins")
	}
}

// MO3: N jobs for the same transfer at once (redeliveries with different event ids) credit once.
func TestConcurrentProcessingCreditsOnce(t *testing.T) {
	t.Parallel()
	e := setup(t)
	e.incoming("trf-c", 100_000)
	var wg sync.WaitGroup
	errs := make([]error, 6)
	for i := range errs {
		wg.Go(func() { errs[i] = e.svc.ProcessTransfer(t.Context(), uuid.Nil, "trf-c") })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("job %d: %v", i, err)
		}
	}
	if c := e.count(t, `SELECT count(*) FROM payins`); c != 1 {
		t.Fatalf("payins = %d", c)
	}
	if c := e.count(t, `SELECT count(*) FROM ledger_transactions WHERE kind = 'payin'`); c != 1 {
		t.Fatalf("ledger transactions = %d", c)
	}
}

// MO4: an uncertain PSP answer credits nothing and is retried; definitive problems settle the
// event with a code and are not retried.
func TestProcessTransferFailures(t *testing.T) {
	t.Parallel()
	e := setup(t)
	ctx := t.Context()
	if err := e.svc.ReceiveWebhook(ctx, e.delivery("evt_1", "trf-unknown")); err != nil {
		t.Fatal(err)
	}
	eventID := e.jobs.jobs[0].(payins.ProcessPayinArgs).EventID

	e.incoming("trf-unknown", 1_000)
	e.psp.FailNext(psp.ErrUncertain)
	if err := e.svc.ProcessTransfer(ctx, eventID, "trf-unknown"); !errors.Is(err, psp.ErrUncertain) {
		t.Fatalf("uncertain: %v", err)
	}
	if c := e.count(t, `SELECT count(*) FROM payins`) + e.count(t, `SELECT count(*) FROM psp_webhook_events WHERE processed_at IS NOT NULL`); c != 0 {
		t.Fatal("an uncertain read changed state")
	}

	if err := e.svc.ProcessTransfer(ctx, eventID, "trf-never-sent"); err != nil {
		t.Fatalf("unknown transfer must settle, not retry: %v", err)
	}
	if c := e.count(t, `SELECT count(*) FROM psp_webhook_events WHERE id = $1 AND error_code = 'psp.transfer_not_found'`, eventID); c != 1 {
		t.Fatal("unknown transfer not recorded")
	}
	e.psp.AddIncoming(psp.IncomingTransfer{ID: "trf-foreign-cvu", CVU: "0000322000000000999999", Amount: money.New(500, money.ARS)})
	if err := e.svc.ProcessTransfer(ctx, uuid.Nil, "trf-foreign-cvu"); err != nil {
		t.Fatalf("unknown cvu must settle: %v", err)
	}
	e.psp.AddIncoming(psp.IncomingTransfer{ID: "trf-usd", CVU: e.cvu.Number, Amount: money.New(500, money.USD)})
	if err := e.svc.ProcessTransfer(ctx, uuid.Nil, "trf-usd"); err != nil {
		t.Fatalf("currency mismatch must settle: %v", err)
	}
	if c := e.count(t, `SELECT count(*) FROM payins`); c != 0 {
		t.Fatalf("payins = %d", c)
	}
}
