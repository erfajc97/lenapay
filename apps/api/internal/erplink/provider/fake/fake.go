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
	"time"

	"lenapay/apps/api/internal/erplink"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/config"
	"lenapay/apps/api/internal/platform/money"
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
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	FullName    string   `json:"fullName"`
	Admin       bool     `json:"admin"`
	Permissions []string `json:"permissions"`
}

type fixtureBuyer struct {
	Ref          string               `json:"ref"`
	ProjectRef   string               `json:"projectRef"`
	Name         string               `json:"name"`
	DocType      string               `json:"docType"`
	DocNumber    string               `json:"docNumber"`
	Email        *string              `json:"email"`
	Installments []fixtureInstallment `json:"installments"`
}

type fixtureInstallment struct {
	Number          int     `json:"number"`
	DueDate         string  `json:"dueDate"` // YYYY-MM-DD
	AmountMinor     int64   `json:"amountMinor"`
	Currency        string  `json:"currency"`
	Status          string  `json:"status"`
	AdjustmentIndex *string `json:"adjustmentIndex"`
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

// Users implements erplink.Directory: the ERP org's users with their ERP permissions (Lebane's
// PermisoUsuario codes). A user without an email falls back to the username when it is one.
func (d *Directory) Users(_ context.Context, erpOrgID string) ([]erplink.User, error) {
	o, ok, err := d.org(erpOrgID)
	if err != nil || !ok {
		return nil, notFound(erpOrgID, err)
	}
	out := make([]erplink.User, 0, len(o.Users))
	for _, u := range o.Users {
		email := u.Email
		if email == "" && strings.Contains(u.Username, "@") {
			email = u.Username
		}
		out = append(out, erplink.User{
			Username: u.Username, Email: email, FullName: u.FullName, Admin: u.Admin, Permissions: slices.Clone(u.Permissions),
		})
	}
	return out, nil
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

// Installments implements erplink.Directory: the buyer's plan from the fixture, by number (an
// unknown buyer has none). A malformed installment is a fixture error, never skipped.
func (d *Directory) Installments(_ context.Context, erpOrgID, buyerRef string) ([]erplink.Installment, error) {
	o, ok, err := d.org(erpOrgID)
	if err != nil || !ok {
		return nil, notFound(erpOrgID, err)
	}
	i := slices.IndexFunc(o.Buyers, func(b fixtureBuyer) bool { return b.Ref == buyerRef })
	if i < 0 {
		return []erplink.Installment{}, nil
	}
	out := make([]erplink.Installment, 0, len(o.Buyers[i].Installments))
	for _, in := range o.Buyers[i].Installments {
		inst, err := in.toInstallment()
		if err != nil {
			return nil, fmt.Errorf("erp fake: %s installment %d: %w", buyerRef, in.Number, err)
		}
		out = append(out, inst)
	}
	slices.SortFunc(out, func(a, b erplink.Installment) int { return a.Number - b.Number })
	return out, nil
}

func (in fixtureInstallment) toInstallment() (erplink.Installment, error) {
	due, err := time.Parse(time.DateOnly, in.DueDate)
	if err != nil {
		return erplink.Installment{}, fmt.Errorf("due date: %w", err)
	}
	currency, err := money.ParseCurrency(in.Currency)
	if err != nil {
		return erplink.Installment{}, err
	}
	status := erplink.InstallmentStatus(in.Status)
	if !slices.Contains([]erplink.InstallmentStatus{erplink.InstallmentPending, erplink.InstallmentPaid, erplink.InstallmentOverdue}, status) {
		return erplink.Installment{}, fmt.Errorf("status %q", in.Status)
	}
	if in.Number < 1 || in.AmountMinor <= 0 {
		return erplink.Installment{}, fmt.Errorf("number or amount out of range")
	}
	return erplink.Installment{
		Number: in.Number, DueDate: due, Amount: money.New(in.AmountMinor, currency), Status: status,
		AdjustmentIndex: in.AdjustmentIndex,
	}, nil
}

func notFound(erpOrgID string, err error) error {
	if err != nil {
		return err
	}
	return apperr.New(apperr.CodeERPLinkNotFound, map[string]any{"erpOrgId": erpOrgID})
}
