package payerview

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/payouts"
	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
)

// The pay operation answers a Payout; if this copy drops or renames a field, the payer sees a
// different payout than the company's payouts endpoints and realtime event show (G12).
func TestPayoutToAPIMatchesThePayoutView(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 3, 14, 15, 9, 26, 0, time.UTC)
	actor := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	from := payouts.StatusPendingApproval
	reason, failure := "duplicated invoice", "PSP_REJECTED"
	full := payouts.Payout{
		ID: uuid.New(), OrgID: uuid.New(), AccountID: uuid.New(),
		Amount: money.New(150000, money.ARS), Fee: money.New(1250, money.ARS),
		Destination: psp.Destination{
			Type: psp.DestinationCVU, Value: "lenapay.alias", AccountNumber: "0000003100010000000001",
			HolderName: "Ñandú SA", HolderCUIT: "30712345678", BankName: "Lenapay",
		},
		Concept: "Cuota 3", Status: payouts.StatusSettled, Rail: payouts.RailInternal,
		CreatedBy: actor, ApprovedBy: &actor, RejectReason: &reason, FailureCode: &failure,
		Events: []payouts.Event{
			{ToStatus: payouts.StatusPendingApproval, ActorID: &actor, CreatedAt: at},
			{FromStatus: &from, ToStatus: payouts.StatusSettled, Reason: &reason, CreatedAt: at},
		},
		Attachments: []payouts.Attachment{{ID: uuid.New(), FileName: "a.pdf", ContentType: "application/pdf", SizeBytes: 9, UploadedBy: actor, CreatedAt: at}},
		CreatedAt:   at, UpdatedAt: at, SettledAt: &at,
	}
	bare := full
	bare.Destination.BankName, bare.ApprovedBy, bare.SettledAt, bare.Events, bare.Attachments = "", nil, nil, nil, nil
	for _, po := range []payouts.Payout{full, bare} {
		view := payouts.NewPayoutView(po)
		fromView, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		fromAPI, err := json.Marshal(payoutToAPI(view))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(fromAPI, fromView) {
			t.Fatalf("api.Payout and payouts.PayoutView differ:\n api: %s\nview: %s", fromAPI, fromView)
		}
	}
}
