// Package fake is an in-memory psp.Provider with the provider's real semantics (P2, AG7):
// CVUs are idempotent by our reference and aliases are unique; outgoing transfers are
// idempotent by our per-attempt reference and start pending until Complete; failures are
// scripted with FailNext (psp.ErrUncertain, psp.ErrRejected, ...). Tests and PSP_MODE=fake
// use it; local runs normally use the HTTP adapter against apps/bind-mock.
package fake

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

type cvu struct {
	account   psp.CVUAccount
	reference string
	disabled  bool
}

type outgoing struct {
	req    psp.SendTransferRequest
	result psp.TransferResult
	at     time.Time
}

// Provider is the fake PSP. Safe for concurrent use.
type Provider struct {
	mu        sync.Mutex
	now       func() time.Time
	cvus      map[string]*cvu // by provider ref
	incoming  map[string]psp.IncomingTransfer
	outgoing  map[string]*outgoing // by our reference
	failing   []error
	calls     int
	sequence  int
	notFounds map[string]bool
}

var _ psp.Provider = (*Provider)(nil)

// New returns an empty fake PSP.
func New() *Provider {
	return &Provider{
		now: func() time.Time { return time.Now().UTC() }, cvus: map[string]*cvu{},
		incoming: map[string]psp.IncomingTransfer{}, outgoing: map[string]*outgoing{}, notFounds: map[string]bool{},
	}
}

// FailNext makes the next calls return these errors, in order.
func (p *Provider) FailNext(errs ...error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failing = append(p.failing, errs...)
}

// Calls counts every call, failed or not.
func (p *Provider) Calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// AddIncoming records a transfer a payer sent to one of our CVUs (what the webhook announces).
func (p *Provider) AddIncoming(t psp.IncomingTransfer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.incoming[t.ID] = t
}

// MarkUnknown makes a destination (CBU, CVU or alias) unknown to ResolveDestination.
func (p *Provider) MarkUnknown(to string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.notFounds[strings.ToLower(to)] = true
}

// Complete moves an outgoing transfer to settled or rejected, as the bank would later.
func (p *Provider) Complete(reference string, status psp.TransferStatus) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	o, ok := p.outgoing[reference]
	if !ok {
		return psp.ErrNotFound
	}
	o.result.Status = status
	return nil
}

// CVUs returns the issued CVUs by provider ref (tests assert "no second CVU").
func (p *Provider) CVUs() map[string]psp.CVUAccount {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[string]psp.CVUAccount{}
	for ref, c := range p.cvus {
		out[ref] = c.account
	}
	return out
}

// begin counts the call and pops a scripted failure. Called with p.mu held.
func (p *Provider) begin() error {
	p.calls++
	if len(p.failing) == 0 {
		return nil
	}
	err := p.failing[0]
	p.failing = p.failing[1:]
	return err
}

// CreateCVU implements psp.Provider: the same reference returns the first CVU.
func (p *Provider) CreateCVU(_ context.Context, req psp.CreateCVURequest) (psp.CVUAccount, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.begin(); err != nil {
		return psp.CVUAccount{}, err
	}
	for _, c := range p.cvus {
		if c.reference == req.Reference {
			return c.account, nil
		}
	}
	p.sequence++
	alias := fmt.Sprintf("fake.cvu.%04d", p.sequence)
	if req.Alias != nil {
		alias = *req.Alias
	}
	if p.aliasTaken(alias, "") {
		return psp.CVUAccount{}, psp.ErrAliasTaken
	}
	account := psp.CVUAccount{CVU: fmt.Sprintf("0000322%015d", p.sequence), Alias: alias, ProviderRef: fmt.Sprintf("caja-%d", p.sequence)}
	p.cvus[account.ProviderRef] = &cvu{account: account, reference: req.Reference}
	return account, nil
}

func (p *Provider) aliasTaken(alias, exceptRef string) bool {
	for ref, c := range p.cvus {
		if ref != exceptRef && strings.EqualFold(c.account.Alias, alias) {
			return true
		}
	}
	return false
}

// UpdateAlias implements psp.Provider.
func (p *Provider) UpdateAlias(_ context.Context, providerRef, alias string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.begin(); err != nil {
		return err
	}
	c, ok := p.cvus[providerRef]
	switch {
	case !ok:
		return psp.ErrNotFound
	case c.disabled:
		return fmt.Errorf("%w: cvu disabled", psp.ErrRejected)
	case p.aliasTaken(alias, providerRef):
		return psp.ErrAliasTaken
	}
	c.account.Alias = alias
	return nil
}

// DisableCVU implements psp.Provider (disabling twice is fine, as at BIND).
func (p *Provider) DisableCVU(_ context.Context, providerRef string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.begin(); err != nil {
		return err
	}
	if c, ok := p.cvus[providerRef]; ok {
		c.disabled = true
	}
	return nil
}

// GetTransfer implements psp.Provider.
func (p *Provider) GetTransfer(_ context.Context, transferID string) (psp.IncomingTransfer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.begin(); err != nil {
		return psp.IncomingTransfer{}, err
	}
	t, ok := p.incoming[transferID]
	if !ok {
		return psp.IncomingTransfer{}, psp.ErrNotFound
	}
	return t, nil
}

// ResolveDestination implements psp.Provider with deterministic holders.
func (p *Provider) ResolveDestination(_ context.Context, to string) (psp.Destination, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.begin(); err != nil {
		return psp.Destination{}, err
	}
	if p.notFounds[strings.ToLower(to)] || strings.HasPrefix(strings.ToLower(to), "noexiste") {
		return psp.Destination{}, psp.ErrNotFound
	}
	d := psp.Destination{Type: psp.DestinationAlias, Value: to, HolderName: "Titular " + to, HolderCUIT: "20123456786", BankName: "Banco"}
	switch {
	case len(to) == 22 && strings.HasPrefix(to, "000"):
		d.Type, d.AccountNumber = psp.DestinationCVU, to
	case len(to) == 22:
		d.Type, d.AccountNumber = psp.DestinationCBU, to
	default:
		d.AccountNumber = fmt.Sprintf("0070999%015d", len(to))
	}
	return d, nil
}

// SendTransfer implements psp.Provider: the same reference returns the first transfer.
func (p *Provider) SendTransfer(_ context.Context, req psp.SendTransferRequest) (psp.TransferResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.begin(); err != nil {
		return psp.TransferResult{}, err
	}
	if o, ok := p.outgoing[req.Reference]; ok {
		return o.result, nil
	}
	p.sequence++
	o := &outgoing{req: req, at: p.now(), result: psp.TransferResult{ProviderID: fmt.Sprintf("trf-%d", p.sequence), Status: psp.TransferPending}}
	p.outgoing[req.Reference] = o
	return o.result, nil
}

// TransferByReference implements psp.Provider.
func (p *Provider) TransferByReference(_ context.Context, reference string) (psp.TransferResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.begin(); err != nil {
		return psp.TransferResult{}, err
	}
	o, ok := p.outgoing[reference]
	if !ok {
		return psp.TransferResult{}, psp.ErrNotFound
	}
	return o.result, nil
}

// Statement implements psp.Provider: incoming transfers and settled outgoing ones of the UTC day.
func (p *Provider) Statement(_ context.Context, currency money.Currency, day time.Time) ([]psp.StatementLine, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.begin(); err != nil {
		return nil, err
	}
	sameDay := func(t time.Time) bool { return t.UTC().Format(time.DateOnly) == day.Format(time.DateOnly) }
	var lines []psp.StatementLine
	for _, t := range p.incoming {
		if t.Amount.Currency == currency && sameDay(t.OccurredAt) {
			lines = append(lines, psp.StatementLine{TransferID: t.ID, Direction: psp.DirectionIn, Amount: t.Amount, OccurredAt: t.OccurredAt})
		}
	}
	for _, o := range p.outgoing {
		if o.result.Status == psp.TransferSettled && o.req.Amount.Currency == currency && sameDay(o.at) {
			lines = append(lines, psp.StatementLine{TransferID: o.result.ProviderID, Direction: psp.DirectionOut, Amount: o.req.Amount, OccurredAt: o.at})
		}
	}
	return lines, nil
}
