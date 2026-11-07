//go:build integration

package payouts_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/erfajc97/lenapay/apps/api/internal/payouts"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/realtime"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/upload"
)

func pdf(n int) payouts.AttachmentUpload {
	return payouts.AttachmentUpload{FileName: "factura.pdf", Content: bytes.NewReader(append([]byte("%PDF-1.7 "), bytes.Repeat([]byte{'a'}, n)...))}
}

// Attachments: the magic bytes decide the type, the file lands under the org's key, Get lists
// them, at most MaxAttachments, a foreign payout is not found and nothing is stored for it.
func TestUploadAttachments(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	e.fund(t, e.account, 2_000_000)
	po, err := e.svc.Create(ctx, e.operator(e.org), createInput(e.account, amountMinor, key()))
	if err != nil {
		t.Fatal(err)
	}
	if len(po.Attachments) != 0 {
		t.Fatalf("attachments = %+v", po.Attachments)
	}
	a, err := e.svc.UploadAttachment(ctx, e.operator(e.org), po.ID, payouts.AttachmentUpload{
		FileName: `C:\facturas\..\factura 12.pdf`, Content: strings.NewReader("%PDF-1.7 factura"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if a.ContentType != upload.TypePDF || a.FileName != "factura 12.pdf" || a.SizeBytes != 16 || a.UploadedBy != e.creator {
		t.Fatalf("attachment = %+v", a)
	}
	stored, err := e.docs.Get("org/" + e.org.String() + "/payouts/" + po.ID.String() + "/" + a.ID.String())
	if err != nil || string(stored) != "%PDF-1.7 factura" {
		t.Fatalf("stored = %q (%v)", stored, err)
	}
	got := e.get(t, po.ID)
	if len(got.Attachments) != 1 || got.Attachments[0].ID != a.ID {
		t.Fatalf("Get attachments = %+v", got.Attachments)
	}

	// A renamed HTML file is refused whatever its name says.
	_, err = e.svc.UploadAttachment(ctx, e.operator(e.org), po.ID, payouts.AttachmentUpload{FileName: "x.pdf", Content: strings.NewReader("<html><script>")})
	wantCode(t, err, apperr.CodePayoutAttachmentInvalid)
	// Another org's payout is not found (T), and a viewer cannot upload.
	_, err = e.svc.UploadAttachment(ctx, e.operator(e.otherOrg), po.ID, pdf(1))
	wantCode(t, err, apperr.CodePayoutNotFound)
	viewer := e.operator(e.org)
	viewer.Role = auth.RoleViewer
	_, err = e.svc.UploadAttachment(ctx, viewer, po.ID, pdf(1))
	wantCode(t, err, apperr.CodeForbidden)
	_, err = e.svc.UploadAttachment(ctx, e.operator(e.org), po.ID, pdf(upload.MaxBytes))
	wantCode(t, err, apperr.CodeBodyTooLarge)

	for range payouts.MaxAttachments - 1 {
		if _, err := e.svc.UploadAttachment(ctx, e.operator(e.org), po.ID, pdf(10)); err != nil {
			t.Fatal(err)
		}
	}
	_, err = e.svc.UploadAttachment(ctx, e.operator(e.org), po.ID, pdf(10))
	wantCode(t, err, apperr.CodePayoutAttachmentInvalid)
	if n := len(e.get(t, po.ID).Attachments); n != payouts.MaxAttachments {
		t.Fatalf("attachments = %d", n)
	}
	var audited int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action = 'payout.attachment_upload' AND target_id = $1`, po.ID).Scan(&audited); err != nil || audited != payouts.MaxAttachments {
		t.Fatalf("audit rows = %d (%v)", audited, err)
	}
	_, err = e.svc.UploadAttachment(ctx, e.operator(e.org), ids.New(), pdf(1))
	wantCode(t, err, apperr.CodePayoutNotFound)
}

// Every payout change publishes payout.updated carrying the payout itself (the app updates its
// cache from the event), and a new company payout also publishes approval.requested.
func TestPayoutEventsCarryThePayout(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	conn, err := e.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+realtime.Channel); err != nil {
		t.Fatal(err)
	}
	e.fund(t, e.account, 2_000_000)
	po, err := e.svc.Create(ctx, e.operator(e.org), createInput(e.account, amountMinor, key()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Approve(ctx, e.approverP(), po.ID, key()); err != nil {
		t.Fatal(err)
	}
	type event struct {
		Event struct {
			Type string `json:"type"`
			Data struct {
				PayoutID string         `json:"payoutId"`
				Status   string         `json:"status"`
				Payout   map[string]any `json:"payout"`
			} `json:"data"`
		} `json:"event"`
	}
	var types, statuses []string
	for len(types) < 4 {
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		n, err := conn.Conn().WaitForNotification(wctx)
		cancel()
		if err != nil {
			t.Fatalf("after %v: %v", types, err)
		}
		var ev event
		if err := json.Unmarshal([]byte(n.Payload), &ev); err != nil {
			t.Fatal(err)
		}
		types = append(types, ev.Event.Type)
		if ev.Event.Type == "balance.changed" {
			continue
		}
		if ev.Event.Data.PayoutID != po.ID.String() || ev.Event.Data.Payout["id"] != po.ID.String() {
			t.Fatalf("event %s data = %+v", ev.Event.Type, ev.Event.Data)
		}
		if _, ok := ev.Event.Data.Payout["attachments"]; !ok {
			t.Fatalf("payout without attachments key: %v", ev.Event.Data.Payout)
		}
		statuses = append(statuses, ev.Event.Type+":"+ev.Event.Data.Payout["status"].(string))
	}
	want := []string{"approval.requested:pending_approval", "payout.updated:pending_approval", "payout.updated:approved"}
	if strings.Join(statuses, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v (all %v)", statuses, types)
	}
}
