// Package fake is the ERP Directory backed by JSON fixtures (ERP_FIXTURES_DIR, default
// apps/api/fixtures/erp): one file per ERP organization with its users, projects and buyers.
// It has the ERP's semantics for the MVP: the identity index holds every org CUIT and every
// buyer document; credentials are checked per ERP org; reads are snapshots.
package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/erfajc97/lenapay/apps/api/internal/erplink"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
)

// DevPassword is the one password the fake ERP accepts for any of its users. It is a fixture
// value of the local fake (no real ERP account has it), documented for the demo.
const DevPassword = "lebane123" //nolint:gosec // fake ERP fixture credential, never a real secret

// Directory is the fixture-backed ERP.
type Directory struct {
	dir  string
	mu   sync.Mutex
	orgs []fixtureOrg // loaded once, on first use
}

var _ erplink.Directory = (*Directory)(nil)

// NewDirectory returns a directory reading fixtures from dir.
func NewDirectory(dir string) *Directory { return &Directory{dir: dir} }

type fixtureOrg struct {
	ERPOrgID  string            `json:"erpOrgId"`
	LegalName string            `json:"legalName"`
	CUIT      string            `json:"cuit"`
	Users     []fixtureUser     `json:"users"`
	Projects  []erplink.Project `json:"projects"`
	Buyers    []fixtureBuyer    `json:"buyers"`
}

type fixtureUser struct {
	Username string `json:"username"`
}

type fixtureBuyer struct {
	Ref        string  `json:"ref"`
	ProjectRef string  `json:"projectRef"`
	Name       string  `json:"name"`
	DocType    string  `json:"docType"`
	DocNumber  string  `json:"docNumber"`
	Email      *string `json:"email"`
}

func (d *Directory) load() ([]fixtureOrg, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.orgs != nil {
		return d.orgs, nil
	}
	files, err := filepath.Glob(filepath.Join(d.dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("erp fake: list fixtures: %w", err)
	}
	slices.Sort(files)
	orgs := make([]fixtureOrg, 0, len(files))
	for _, f := range files {
		raw, err := os.ReadFile(f) //nolint:gosec // fixture files of the configured directory
		if err != nil {
			return nil, fmt.Errorf("erp fake: read %s: %w", filepath.Base(f), err)
		}
		var o fixtureOrg
		if err := json.Unmarshal(raw, &o); err != nil {
			return nil, fmt.Errorf("erp fake: parse %s: %w", filepath.Base(f), err)
		}
		orgs = append(orgs, o)
	}
	d.orgs = orgs
	return orgs, nil
}

func (d *Directory) org(erpOrgID string) (fixtureOrg, bool, error) {
	orgs, err := d.load()
	if err != nil {
		return fixtureOrg{}, false, err
	}
	i := slices.IndexFunc(orgs, func(o fixtureOrg) bool { return o.ERPOrgID == erpOrgID })
	if i < 0 {
		return fixtureOrg{}, false, nil
	}
	return orgs[i], true, nil
}

// IdentityIndex implements erplink.Directory: every org's CUIT and every buyer's document.
func (d *Directory) IdentityIndex(context.Context) ([]erplink.IdentityEntry, error) {
	orgs, err := d.load()
	if err != nil {
		return nil, err
	}
	var out []erplink.IdentityEntry
	for _, o := range orgs {
		out = append(out, erplink.IdentityEntry{DocType: "cuit", DocNumber: o.CUIT, ERPOrgID: o.ERPOrgID, DisplayName: o.LegalName})
		for _, b := range o.Buyers {
			ref := b.Ref
			out = append(out, erplink.IdentityEntry{DocType: b.DocType, DocNumber: b.DocNumber, ERPOrgID: o.ERPOrgID, ERPClientID: &ref, DisplayName: b.Name})
		}
	}
	return out, nil
}

// VerifyCredentials implements erplink.Directory: the user must belong to that ERP org and
// the password must be DevPassword.
func (d *Directory) VerifyCredentials(_ context.Context, erpOrgID, username string, password config.Secret) error {
	o, ok, err := d.org(erpOrgID)
	if err != nil {
		return err
	}
	known := ok && slices.ContainsFunc(o.Users, func(u fixtureUser) bool { return strings.EqualFold(u.Username, strings.TrimSpace(username)) })
	if !known || password.Reveal() != DevPassword {
		return apperr.New(apperr.CodeERPCredentialsInvalid, nil)
	}
	return nil
}

// Projects implements erplink.Directory.
func (d *Directory) Projects(_ context.Context, erpOrgID string) ([]erplink.Project, error) {
	o, ok, err := d.org(erpOrgID)
	if err != nil || !ok {
		return nil, notFound(erpOrgID, err)
	}
	return slices.Clone(o.Projects), nil
}

// Buyers implements erplink.Directory.
func (d *Directory) Buyers(_ context.Context, erpOrgID string) ([]erplink.Buyer, error) {
	o, ok, err := d.org(erpOrgID)
	if err != nil || !ok {
		return nil, notFound(erpOrgID, err)
	}
	out := make([]erplink.Buyer, 0, len(o.Buyers))
	for _, b := range o.Buyers {
		out = append(out, erplink.Buyer{Ref: b.Ref, ProjectRef: b.ProjectRef, Name: b.Name, DocType: b.DocType, DocNumber: b.DocNumber, Email: b.Email})
	}
	return out, nil
}

func notFound(erpOrgID string, err error) error {
	if err != nil {
		return err
	}
	return apperr.New(apperr.CodeERPLinkNotFound, map[string]any{"erpOrgId": erpOrgID})
}
