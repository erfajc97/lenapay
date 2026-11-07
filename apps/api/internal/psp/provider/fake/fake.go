// Package fake is an in-memory psp.Provider for unit and integration tests (lane C gives it
// the provider's real semantics: idempotent by our reference, async settlement, scriptable
// failures including ErrUncertain). Local runs use the HTTP adapter against apps/bind-mock.
package fake

import (
	"context"
	"sync"
	"time"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// Provider is the fake PSP.
type Provider struct {
	mu sync.Mutex
}

var _ psp.Provider = (*Provider)(nil)

// New returns an empty fake PSP.
func New() *Provider { return &Provider{} }

func (p *Provider) stub() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return apperr.NotImplemented()
}

// CreateCVU implements psp.Provider.
func (p *Provider) CreateCVU(context.Context, psp.CreateCVURequest) (psp.CVUAccount, error) {
	return psp.CVUAccount{}, p.stub()
}

// UpdateAlias implements psp.Provider.
func (p *Provider) UpdateAlias(context.Context, string, string) error { return p.stub() }

// DisableCVU implements psp.Provider.
func (p *Provider) DisableCVU(context.Context, string) error { return p.stub() }

// GetTransfer implements psp.Provider.
func (p *Provider) GetTransfer(context.Context, string) (psp.IncomingTransfer, error) {
	return psp.IncomingTransfer{}, p.stub()
}

// ResolveDestination implements psp.Provider.
func (p *Provider) ResolveDestination(context.Context, string) (psp.Destination, error) {
	return psp.Destination{}, p.stub()
}

// SendTransfer implements psp.Provider.
func (p *Provider) SendTransfer(context.Context, psp.SendTransferRequest) (psp.TransferResult, error) {
	return psp.TransferResult{}, p.stub()
}

// TransferByReference implements psp.Provider.
func (p *Provider) TransferByReference(context.Context, string) (psp.TransferResult, error) {
	return psp.TransferResult{}, p.stub()
}

// Statement implements psp.Provider.
func (p *Provider) Statement(context.Context, money.Currency, time.Time) ([]psp.StatementLine, error) {
	return nil, p.stub()
}
