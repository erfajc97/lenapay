// Package fake holds in-memory implementations of the kyc ports for local runs and tests
// (P2: local runs never call the real vendor). Lane A gives them the vendor's real semantics:
// approve valid-looking data, reject the configured DNI list, idempotent by Reference.
package fake

import (
	"context"
	"io"
	"sync"

	"github.com/erfajc97/lenapay/apps/api/internal/kyc"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// Provider is the fake identity-verification vendor.
type Provider struct {
	mu           sync.Mutex
	rejectedDNIs map[string]bool
}

var _ kyc.Provider = (*Provider)(nil)

// NewProvider returns a fake vendor that rejects the given DNIs.
func NewProvider(rejectedDNIs []string) *Provider {
	p := &Provider{rejectedDNIs: map[string]bool{}}
	for _, dni := range rejectedDNIs {
		p.rejectedDNIs[dni] = true
	}
	return p
}

// VerifyPerson implements kyc.Provider.
func (p *Provider) VerifyPerson(context.Context, kyc.PersonCheck) (kyc.Verdict, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return kyc.Verdict{}, apperr.NotImplemented()
}

// VerifyCompany implements kyc.Provider.
func (p *Provider) VerifyCompany(context.Context, kyc.CompanyCheck) (kyc.Verdict, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return kyc.Verdict{}, apperr.NotImplemented()
}

// DocumentStore is an in-memory private document store.
type DocumentStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

var _ kyc.DocumentStore = (*DocumentStore)(nil)

// NewDocumentStore returns an empty store.
func NewDocumentStore() *DocumentStore { return &DocumentStore{objects: map[string][]byte{}} }

// Put implements kyc.DocumentStore.
func (s *DocumentStore) Put(_ context.Context, key, _ string, content io.Reader) error {
	b, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = b
	return nil
}
