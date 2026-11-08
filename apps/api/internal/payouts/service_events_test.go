package payouts

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
)

// The golden files were written from the payout.updated payload of the code BEFORE the event
// stopped importing the generated API types: the mobile app parses this exact shape, so any
// byte that changes here breaks a client in the field.

func goldenPayout() Payout {
	at := time.Date(2026, 3, 14, 15, 9, 26, 535000000, time.UTC)
	approver := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	actor := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	from := StatusPendingApproval
	reason := "duplicated invoice"
	failure := "PSP_REJECTED"
	settled := at.Add(time.Hour)
	return Payout{
		ID:        uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		OrgID:     uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		AccountID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		Amount:    money.New(150000, money.ARS),
		Fee:       money.New(1250, money.ARS),
		Destination: psp.Destination{
			Type: psp.DestinationCBU, Value: "0000003100010000000001", AccountNumber: "0000003100010000000001",
			HolderName: "Proveedor \"Ñandú\" & Cía <SA>", HolderCUIT: "30712345678", BankName: "Banco Galicia",
		},
		Concept: "Factura A-0001 ñ € 🙂", Status: StatusFailed, Rail: RailBank,
		CreatedBy: actor, ApprovedBy: &approver, RejectReason: nil, FailureCode: &failure,
		Events: []Event{
			{ToStatus: StatusPendingApproval, ActorID: &actor, CreatedAt: at},
			{FromStatus: &from, ToStatus: StatusApproved, ActorID: &approver, Reason: &reason, CreatedAt: at.Add(time.Minute)},
			{FromStatus: &from, ToStatus: StatusFailed, CreatedAt: at.Add(2 * time.Minute)},
		},
		Attachments: []Attachment{{
			ID: uuid.MustParse("66666666-6666-6666-6666-666666666666"), FileName: "factura.pdf",
			ContentType: "application/pdf", SizeBytes: 48213, UploadedBy: actor, CreatedAt: at,
		}},
		CreatedAt: at, UpdatedAt: at.Add(2 * time.Minute), SettledAt: &settled,
	}
}

// bigPayout has enough attachments to exceed maxEventBytes, so its event goes out without the
// lists; it also has no bank name and no approver (the null branches).
func bigPayout() Payout {
	po := goldenPayout()
	po.Destination.BankName = ""
	po.ApprovedBy, po.FailureCode, po.SettledAt = nil, nil, nil
	po.Status, po.Rail = StatusPendingApproval, RailInternal
	po.Attachments = nil
	for i := range 60 {
		po.Attachments = append(po.Attachments, Attachment{
			ID:       uuid.NewSHA1(uuid.NameSpaceOID, []byte{byte(i)}),
			FileName: strings.Repeat("comprobante-de-pago-", 4) + ".pdf", ContentType: "image/png",
			SizeBytes: int64(i), UploadedBy: po.CreatedBy, CreatedAt: po.CreatedAt,
		})
	}
	return po
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if !bytes.Equal(got, bytes.TrimRight(want, "\n")) {
		t.Fatalf("payload of %s changed:\n got: %s\nwant: %s", name, got, want)
	}
}

func TestPayoutEventDataMatchesTheGoldenWireFormat(t *testing.T) {
	t.Parallel()
	bare := goldenPayout()
	bare.Events, bare.Attachments = nil, nil
	cases := []struct {
		golden string
		po     Payout
	}{
		{"payout_event.golden.json", goldenPayout()},
		{"payout_event_nil_lists.golden.json", bare},
		{"payout_event_trimmed.golden.json", bigPayout()},
	}
	for _, c := range cases {
		t.Run(c.golden, func(t *testing.T) {
			t.Parallel()
			raw, err := payoutEventData(c.po)
			if err != nil {
				t.Fatal(err)
			}
			if len(raw) > maxEventBytes {
				t.Fatalf("payload is %d bytes, over the NOTIFY budget %d", len(raw), maxEventBytes)
			}
			assertGolden(t, c.golden, raw)
		})
	}
}
