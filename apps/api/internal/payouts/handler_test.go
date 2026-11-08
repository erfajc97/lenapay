package payouts

import (
	"bytes"
	"encoding/json"
	"testing"
)

// The HTTP responses (api.Payout) and the realtime event (PayoutView) are two doors of the same
// payout: if the copy into the generated type drops or renames a field, the app's cache and the
// API disagree (G12).
func TestPayoutToAPIIsByteIdenticalToTheView(t *testing.T) {
	t.Parallel()
	bare := goldenPayout()
	bare.Events, bare.Attachments = nil, nil
	for _, po := range []Payout{goldenPayout(), bigPayout(), bare} {
		view := NewPayoutView(po)
		fromView, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		fromAPI, err := json.Marshal(payoutToAPI(view))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(fromAPI, fromView) {
			t.Fatalf("api.Payout and PayoutView differ:\n api: %s\nview: %s", fromAPI, fromView)
		}
	}
}
