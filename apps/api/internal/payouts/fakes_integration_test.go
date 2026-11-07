//go:build integration

package payouts_test

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/notifications"
	"github.com/erfajc97/lenapay/apps/api/internal/orgs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// fakePSP is a scriptable PSP with the provider's semantics the payouts rely on: transfers
// are keyed by our reference, and the scripted outcome decides what SendTransfer and
// TransferByReference answer. Only the methods payouts call are meaningful.
type fakePSP struct {
	mu      sync.Mutex
	dest    psp.Destination
	resolve error
	send    psp.TransferResult
	sendErr error
	lookup  psp.TransferResult
	lookErr error
	sent    []psp.SendTransferRequest
	lookups []string
}

func newFakePSP() *fakePSP {
	return &fakePSP{
		dest: psp.Destination{
			Type: psp.DestinationCBU, AccountNumber: "0170099220000067797370", HolderName: "Proveedora Andes SRL",
			HolderCUIT: "30711122233", BankName: "Banco Galicia",
		},
		send: psp.TransferResult{ProviderID: "bind-tr-1", Status: psp.TransferSettled},
	}
}

func (f *fakePSP) script(send psp.TransferResult, sendErr error, lookup psp.TransferResult, lookErr error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.send, f.sendErr, f.lookup, f.lookErr = send, sendErr, lookup, lookErr
}

func (f *fakePSP) sentCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func (f *fakePSP) ResolveDestination(context.Context, string) (psp.Destination, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dest, f.resolve
}

func (f *fakePSP) SendTransfer(_ context.Context, req psp.SendTransferRequest) (psp.TransferResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, req)
	return f.send, f.sendErr
}

func (f *fakePSP) TransferByReference(_ context.Context, ref string) (psp.TransferResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lookups = append(f.lookups, ref)
	return f.lookup, f.lookErr
}

func (f *fakePSP) CreateCVU(context.Context, psp.CreateCVURequest) (psp.CVUAccount, error) {
	return psp.CVUAccount{}, apperr.NotImplemented()
}

func (f *fakePSP) UpdateAlias(context.Context, string, string) error { return apperr.NotImplemented() }

func (f *fakePSP) DisableCVU(context.Context, string) error { return apperr.NotImplemented() }

func (f *fakePSP) GetTransfer(context.Context, string) (psp.IncomingTransfer, error) {
	return psp.IncomingTransfer{}, apperr.NotImplemented()
}

func (f *fakePSP) Statement(context.Context, money.Currency, time.Time) ([]psp.StatementLine, error) {
	return nil, apperr.NotImplemented()
}

// fakeAccounts answers the accounts the test created (others are not found, like the real one).
type fakeAccounts struct {
	byID map[uuid.UUID]accounts.Account
}

func (f fakeAccounts) Get(_ context.Context, orgID, accountID uuid.UUID) (accounts.Account, error) {
	a, ok := f.byID[accountID]
	if !ok || a.OrgID != orgID {
		return accounts.Account{}, apperr.New(apperr.CodeAccountNotFound, nil)
	}
	return a, nil
}

type fakeMembers struct{ members map[uuid.UUID][]orgs.Member }

func (f fakeMembers) ListMembers(_ context.Context, p auth.Principal) ([]orgs.Member, error) {
	return f.members[p.OrgID], nil
}

// recordingNotifier writes nothing but remembers what the payout asked for inside its tx.
type recordingNotifier struct {
	mu   sync.Mutex
	sent []notifications.Notification
}

func (r *recordingNotifier) Notify(_ context.Context, _ pgx.Tx, n notifications.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, n)
	return nil
}

func (r *recordingNotifier) kinds() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.sent))
	for _, n := range r.sent {
		out = append(out, n.Kind)
	}
	return out
}
