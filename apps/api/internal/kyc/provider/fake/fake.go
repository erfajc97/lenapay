// Package fake holds in-memory implementations of the kyc ports for local runs and tests
// (P2: local runs never call the real vendor), with the vendor's real semantics: approve
// valid-looking data, reject the configured DNI list, idempotent by Reference.
package fake

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/erfajc97/lenapay/apps/api/internal/kyc"
)

// Rejection reasons the fake vendor answers (stored as the case's decision reason).
const (
	ReasonDNIRejected = "dni_rejected"
	ReasonInvalidData = "invalid_data"
)

// ErrUnavailable is what a vendor outage looks like (scripted with FailNext).
var ErrUnavailable = errors.New("fake kyc: vendor unavailable")

var (
	dniPattern  = regexp.MustCompile(`^[0-9]{7,8}$`)
	cuitPattern = regexp.MustCompile(`^[0-9]{11}$`)
)

// Provider is the fake identity-verification vendor.
type Provider struct {
	mu           sync.Mutex
	rejectedDNIs map[string]bool
	verdicts     map[string]kyc.Verdict
	failNext     int
}

var _ kyc.Provider = (*Provider)(nil)

// NewProvider returns a fake vendor that rejects the given DNIs.
func NewProvider(rejectedDNIs []string) *Provider {
	p := &Provider{rejectedDNIs: map[string]bool{}, verdicts: map[string]kyc.Verdict{}}
	for _, dni := range rejectedDNIs {
		if dni = strings.TrimSpace(dni); dni != "" {
			p.rejectedDNIs[dni] = true
		}
	}
	return p
}

// FailNext makes the next n calls fail with ErrUnavailable (tests of the uncertain path).
func (p *Provider) FailNext(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failNext = n
}

// VerifyPerson implements kyc.Provider.
func (p *Provider) VerifyPerson(_ context.Context, check kyc.PersonCheck) (kyc.Verdict, error) {
	return p.verify(check.Reference, func() kyc.Verdict {
		switch {
		case !dniPattern.MatchString(check.DNI) || strings.TrimSpace(check.FullName) == "" || check.SelfieKey == "":
			return kyc.Verdict{Reason: ReasonInvalidData}
		case p.rejectedDNIs[check.DNI]:
			return kyc.Verdict{Reason: ReasonDNIRejected}
		}
		return kyc.Verdict{Approved: true}
	})
}

// VerifyCompany implements kyc.Provider.
func (p *Provider) VerifyCompany(_ context.Context, check kyc.CompanyCheck) (kyc.Verdict, error) {
	return p.verify(check.Reference, func() kyc.Verdict {
		if !cuitPattern.MatchString(check.CUIT) || strings.TrimSpace(check.LegalName) == "" {
			return kyc.Verdict{Reason: ReasonInvalidData}
		}
		return kyc.Verdict{Approved: true}
	})
}

// verify answers the same verdict for the same reference (idempotent, like the vendor).
func (p *Provider) verify(reference string, decide func() kyc.Verdict) (kyc.Verdict, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failNext > 0 {
		p.failNext--
		return kyc.Verdict{}, ErrUnavailable
	}
	if v, ok := p.verdicts[reference]; ok {
		return v, nil
	}
	v := decide()
	sum := sha256.Sum256([]byte(reference))
	v.ProviderRef = "fake-" + hex.EncodeToString(sum[:8])
	p.verdicts[reference] = v
	return v, nil
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

// Object returns a stored object (tests).
func (s *DocumentStore) Object(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.objects[key]
	return b, ok
}
