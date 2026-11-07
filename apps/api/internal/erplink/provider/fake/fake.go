// Package fake is the ERP Directory backed by JSON fixtures (ERP_FIXTURES_DIR, default
// apps/api/fixtures/erp). Lane D adds the fixture files and gives it the ERP's semantics.
package fake

import (
	"context"
	"sync"

	"github.com/erfajc97/lenapay/apps/api/internal/erplink"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
)

// Directory is the fixture-backed ERP.
type Directory struct {
	mu  sync.Mutex
	dir string
}

var _ erplink.Directory = (*Directory)(nil)

// NewDirectory returns a directory reading fixtures from dir.
func NewDirectory(dir string) *Directory { return &Directory{dir: dir} }

func (d *Directory) stub() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_ = d.dir
	return apperr.NotImplemented()
}

// IdentityIndex implements erplink.Directory.
func (d *Directory) IdentityIndex(context.Context) ([]erplink.IdentityEntry, error) {
	return nil, d.stub()
}

// VerifyCredentials implements erplink.Directory.
func (d *Directory) VerifyCredentials(context.Context, string, string, config.Secret) error {
	return d.stub()
}

// Projects implements erplink.Directory.
func (d *Directory) Projects(context.Context, string) ([]erplink.Project, error) {
	return nil, d.stub()
}

// Buyers implements erplink.Directory.
func (d *Directory) Buyers(context.Context, string) ([]erplink.Buyer, error) {
	return nil, d.stub()
}
