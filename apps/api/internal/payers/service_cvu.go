package payers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/payers/store"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/idempotency"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// statusCreated is the createCvu answer a replay repeats (201; services do not import net/http).
const statusCreated = 201

// statusActive is the receiving status of a CVU (DB CHECK cvus.status: active | disabled).
const statusActive = "active"

// cvuReferenceSpace namespaces the deterministic PSP references of CVUs: the same client
// action always sends the same reference, so the PSP returns the first CVU on a retry.
var (
	cvuReferenceSpace = uuid.MustParse("7f1d2c4e-9a51-4c47-8f0e-3c5b6a2d1e90")
	aliasPattern      = regexp.MustCompile(`^[a-zA-Z0-9.-]{6,20}$`)
	errPeek           = errors.New("payers: idempotency peek")
)

// cvuReplay is the stored response of a completed createCvu key (the CVU is re-read on replay).
type cvuReplay struct {
	CVUID uuid.UUID `json:"cvuId"`
}

// CreateCVU issues the payer's CVU (R-CVU-1, MO2). The PSP is called OUTSIDE any transaction
// with a reference derived from the idempotency key, so a retry after a timeout gets the same
// CVU back from the PSP; the CVU row, the audit entry and the key's response are written in one
// transaction afterwards.
func (s *service) CreateCVU(ctx context.Context, p auth.Principal, payerID uuid.UUID, in CreateCVUInput) (CVU, error) {
	if in.Currency != money.ARS && in.Currency != money.USD {
		return CVU{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "currency"})
	}
	if in.Alias != nil && !aliasPattern.MatchString(*in.Alias) {
		return CVU{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "alias"})
	}
	payer, err := s.Get(ctx, p, payerID)
	if err != nil {
		return CVU{}, err
	}
	hash, err := idempotency.HashRequest(struct {
		PayerID  uuid.UUID
		Currency money.Currency
		Alias    *string
	}{payerID, in.Currency, in.Alias})
	if err != nil {
		return CVU{}, err
	}
	key := idempotency.Key{Scope: idempotency.Scope(p.OrgID, "payers.create_cvu"), Value: in.IdempotencyKey, RequestHash: hash}
	if replay, err := s.peek(ctx, key); err != nil || replay != nil {
		return s.replayCVU(ctx, p.OrgID, replay, err)
	}
	if err := s.noActiveCVU(ctx, p.OrgID, payerID, in.Currency); err != nil {
		return CVU{}, err
	}
	holderName, holderDoc, err := s.holderOf(ctx, p)
	if err != nil {
		return CVU{}, err
	}
	reference := uuid.NewSHA1(cvuReferenceSpace, []byte(key.Scope+"|"+key.Value+"|"+fmt.Sprintf("%x", hash)))
	issued, err := s.deps.PSP.CreateCVU(ctx, psp.CreateCVURequest{
		Reference: reference.String(), Currency: in.Currency, HolderName: holderName, HolderDoc: holderDoc, Alias: in.Alias,
	})
	if err != nil {
		return CVU{}, pspError(err)
	}
	row := store.InsertCVUParams{
		ID: reference, OrgID: p.OrgID, AccountID: payer.AccountID, PayerID: &payerID, Currency: string(in.Currency),
		Cvu: issued.CVU, Alias: nonEmpty(issued.Alias), PspRef: issued.ProviderRef,
	}
	var replay *idempotency.Stored
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if replay, err = s.deps.Idempotency.Claim(ctx, tx, key); err != nil || replay != nil {
			return err
		}
		if err := insertCVU(ctx, store.New(tx), row); err != nil {
			return err
		}
		if err := s.audit(ctx, tx, p, "cvu.create", "cvu", row.ID, key.Value); err != nil {
			return err
		}
		body, _ := json.Marshal(cvuReplay{CVUID: row.ID}) // a uuid: cannot fail
		return s.deps.Idempotency.Complete(ctx, tx, key, statusCreated, body)
	})
	if replay != nil || err != nil {
		return s.replayCVU(ctx, p.OrgID, replay, err)
	}
	return s.cvu(ctx, p.OrgID, row.ID)
}

// peek claims the key in a transaction that is always rolled back: it answers "replay",
// "reused with another request" or "new" WITHOUT keeping the claim, so the PSP is never called
// for a key that cannot succeed. The authoritative claim happens with the insert.
func (s *service) peek(ctx context.Context, key idempotency.Key) (*idempotency.Stored, error) {
	var replay *idempotency.Stored
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		r, err := s.deps.Idempotency.Claim(ctx, tx, key)
		if err != nil {
			return err
		}
		replay = r
		return errPeek
	})
	if errors.Is(err, errPeek) {
		return replay, nil
	}
	return nil, err
}

func (s *service) replayCVU(ctx context.Context, orgID uuid.UUID, replay *idempotency.Stored, err error) (CVU, error) {
	if err != nil {
		return CVU{}, err
	}
	var stored cvuReplay
	if err := json.Unmarshal(replay.Body, &stored); err != nil {
		return CVU{}, fmt.Errorf("decode stored cvu response: %w", err)
	}
	return s.cvu(ctx, orgID, stored.CVUID)
}

func (s *service) noActiveCVU(ctx context.Context, orgID, payerID uuid.UUID, currency money.Currency) error {
	_, err := store.New(s.deps.Pool).GetActiveCVUForPayer(ctx, store.GetActiveCVUForPayerParams{
		OrgID: orgID, PayerID: &payerID, Currency: string(currency),
	})
	switch {
	case err == nil:
		return apperr.New(apperr.CodeCVUAlreadyExists, nil)
	case db.IsNoRows(err):
		return nil
	default:
		return fmt.Errorf("active cvu: %w", err)
	}
}

// insertCVU maps the unique indexes of cvus to their catalog codes.
func insertCVU(ctx context.Context, q *store.Queries, row store.InsertCVUParams) error {
	err := q.InsertCVU(ctx, row)
	switch {
	case err == nil:
		return nil
	case db.IsUniqueViolation(err, "cvus_payer_currency_key"), db.IsUniqueViolation(err, "cvus_account_currency_key"):
		return apperr.Wrap(apperr.CodeCVUAlreadyExists, err, nil)
	case db.IsUniqueViolation(err, "cvus_alias_key"):
		return apperr.Wrap(apperr.CodeCVUAliasTaken, err, nil)
	default:
		return fmt.Errorf("insert cvu: %w", err)
	}
}

// UpdateCVUAlias changes the alias at the PSP first (the PSP owns alias uniqueness across the
// whole network), then records it (R-CVU-2).
func (s *service) UpdateCVUAlias(ctx context.Context, p auth.Principal, cvuID uuid.UUID, alias string) (CVU, error) {
	if !aliasPattern.MatchString(alias) {
		return CVU{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "alias"})
	}
	current, row, err := s.cvuRow(ctx, p.OrgID, cvuID)
	if err != nil {
		return CVU{}, err
	}
	if !current.Active {
		return CVU{}, apperr.New(apperr.CodeCVUDisabled, nil)
	}
	if current.Alias != nil && strings.EqualFold(*current.Alias, alias) {
		return current, nil
	}
	if err := s.deps.PSP.UpdateAlias(ctx, row.PspRef, alias); err != nil {
		return CVU{}, pspError(err)
	}
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		n, err := store.New(tx).UpdateCVUAlias(ctx, store.UpdateCVUAliasParams{ID: cvuID, OrgID: p.OrgID, Alias: &alias, UpdatedAt: s.deps.Clock.Now()})
		switch {
		case db.IsUniqueViolation(err, "cvus_alias_key"):
			return apperr.Wrap(apperr.CodeCVUAliasTaken, err, nil)
		case err != nil:
			return fmt.Errorf("update alias: %w", err)
		case n != 1:
			return apperr.New(apperr.CodeCVUDisabled, nil) // disabled meanwhile
		}
		return s.audit(ctx, tx, p, "cvu.update_alias", "cvu", cvuID, "")
	})
	if err != nil {
		return CVU{}, err
	}
	return s.cvu(ctx, p.OrgID, cvuID)
}

// DisableCVU disables the CVU at the PSP, then records it. Disabling twice is a no-op.
func (s *service) DisableCVU(ctx context.Context, p auth.Principal, cvuID uuid.UUID) error {
	current, row, err := s.cvuRow(ctx, p.OrgID, cvuID)
	if err != nil || !current.Active {
		return err
	}
	if err := s.deps.PSP.DisableCVU(ctx, row.PspRef); err != nil {
		return pspError(err)
	}
	return db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		now := s.deps.Clock.Now()
		n, err := store.New(tx).DisableCVU(ctx, store.DisableCVUParams{ID: cvuID, OrgID: p.OrgID, DisabledAt: &now})
		if err != nil {
			return fmt.Errorf("disable cvu: %w", err)
		}
		if n == 0 {
			return nil // a concurrent request disabled it first
		}
		return s.audit(ctx, tx, p, "cvu.disable", "cvu", cvuID, "")
	})
}

// CVUByNumber is the system lookup of a receiving CVU (payins): the CVU decides the org.
func (s *service) CVUByNumber(ctx context.Context, number string) (CVU, error) {
	row, err := store.New(s.deps.Pool).GetCVUByNumber(ctx, number)
	if db.IsNoRows(err) {
		return CVU{}, apperr.New(apperr.CodeCVUNotFound, nil)
	}
	if err != nil {
		return CVU{}, fmt.Errorf("cvu by number: %w", err)
	}
	return s.toCVU(row), nil
}

func (s *service) cvu(ctx context.Context, orgID, cvuID uuid.UUID) (CVU, error) {
	c, _, err := s.cvuRow(ctx, orgID, cvuID)
	return c, err
}

func (s *service) cvuRow(ctx context.Context, orgID, cvuID uuid.UUID) (CVU, store.Cvu, error) {
	row, err := store.New(s.deps.Pool).GetCVU(ctx, store.GetCVUParams{ID: cvuID, OrgID: orgID})
	if db.IsNoRows(err) {
		return CVU{}, store.Cvu{}, apperr.New(apperr.CodeCVUNotFound, nil)
	}
	if err != nil {
		return CVU{}, store.Cvu{}, fmt.Errorf("get cvu: %w", err)
	}
	return s.toCVU(row), row, nil
}

// OnKYCApproved queues the personal CVUs inside the approval transaction (never a PSP call
// there): the job exists if and only if the approval commits.
func (s *service) OnKYCApproved(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error {
	if orgKind != "personal" {
		return nil
	}
	if _, err := s.deps.Jobs.InsertTx(ctx, tx, PersonalCVUsArgs{OrgID: orgID}, nil); err != nil {
		return fmt.Errorf("queue personal cvus: %w", err)
	}
	return nil
}

// CreatePersonalCVUs issues one CVU per currency for the personal account (R-CVU-3). It is
// safe to run again: each currency's PSP reference is fixed and existing CVUs are skipped.
func (s *service) CreatePersonalCVUs(ctx context.Context, orgID uuid.UUID) error {
	system := auth.Principal{OrgID: orgID}
	account, err := s.personalAccount(ctx, system)
	if err != nil {
		return err
	}
	existing, err := store.New(s.deps.Pool).ListCVUsForAccount(ctx, store.ListCVUsForAccountParams{OrgID: orgID, AccountID: account.ID})
	if err != nil {
		return fmt.Errorf("personal cvus: %w", err)
	}
	have := map[string]bool{}
	for _, c := range existing {
		have[c.Currency] = have[c.Currency] || (c.PayerID == nil && c.Status == statusActive)
	}
	var org struct{ name, doc string }
	for _, currency := range []money.Currency{money.ARS, money.USD} {
		if have[string(currency)] {
			continue
		}
		if org.name == "" {
			if org.name, org.doc, err = s.holderOf(ctx, system); err != nil {
				return err
			}
		}
		reference := uuid.NewSHA1(cvuReferenceSpace, []byte("personal|"+orgID.String()+"|"+string(currency)))
		issued, err := s.deps.PSP.CreateCVU(ctx, psp.CreateCVURequest{
			Reference: reference.String(), Currency: currency, HolderName: org.name, HolderDoc: org.doc,
		})
		if err != nil {
			return fmt.Errorf("personal cvu %s: %w", currency, err) // the job retries
		}
		err = insertCVU(ctx, store.New(s.deps.Pool), store.InsertCVUParams{
			ID: reference, OrgID: orgID, AccountID: account.ID, Currency: string(currency),
			Cvu: issued.CVU, Alias: nonEmpty(issued.Alias), PspRef: issued.ProviderRef,
		})
		if err != nil && !apperr.IsCode(err, apperr.CodeCVUAlreadyExists) && !db.IsUniqueViolation(err, "") {
			return err
		}
	}
	return nil
}

func (s *service) PersonalCVU(ctx context.Context, orgID uuid.UUID, currency money.Currency) (CVU, error) {
	account, err := s.personalAccount(ctx, auth.Principal{OrgID: orgID})
	if err != nil {
		return CVU{}, err
	}
	rows, err := store.New(s.deps.Pool).ListCVUsForAccount(ctx, store.ListCVUsForAccountParams{OrgID: orgID, AccountID: account.ID})
	if err != nil {
		return CVU{}, fmt.Errorf("personal cvu: %w", err)
	}
	for _, r := range rows {
		if r.PayerID == nil && r.Status == statusActive && r.Currency == string(currency) {
			return s.toCVU(r), nil
		}
	}
	return CVU{}, apperr.New(apperr.CodeCVUNotFound, nil)
}

func (s *service) personalAccount(ctx context.Context, system auth.Principal) (accounts.Account, error) {
	list, err := s.deps.Accounts.List(ctx, system, false)
	if err != nil {
		return accounts.Account{}, err
	}
	for _, a := range list {
		if a.Kind == accounts.KindPersonal {
			return a, nil
		}
	}
	return accounts.Account{}, apperr.New(apperr.CodeAccountNotFound, nil)
}

// holderOf returns the org's legal name and CUIT. A system caller reads it as the org's first
// member (orgs.Get is a member's view).
func (s *service) holderOf(ctx context.Context, p auth.Principal) (name, doc string, err error) {
	if p.UserID == uuid.Nil {
		members, err := s.deps.Orgs.ListMembers(ctx, p)
		if err != nil {
			return "", "", err
		}
		if len(members) == 0 {
			return "", "", apperr.New(apperr.CodeOrgNotFound, nil)
		}
		p.UserID = members[0].UserID
	}
	org, err := s.deps.Orgs.Get(ctx, p)
	if err != nil {
		return "", "", err
	}
	return org.LegalName, deref(org.CUIT), nil
}

// pspError maps the PSP port's errors to catalog codes. Uncertain is never "failed": the
// client retries with the same idempotency key and gets the same CVU (MO4).
func pspError(err error) error {
	switch {
	case errors.Is(err, psp.ErrAliasTaken):
		return apperr.Wrap(apperr.CodeCVUAliasTaken, err, nil)
	case errors.Is(err, psp.ErrNotFound):
		return apperr.Wrap(apperr.CodeCVUNotFound, err, nil)
	case errors.Is(err, psp.ErrRejected):
		return apperr.Wrap(apperr.CodeValidation, err, map[string]any{"reason": "provider_rejected"})
	case errors.Is(err, psp.ErrUncertain):
		slog.Warn("psp uncertain", "err", err.Error())
		return apperr.Wrap(apperr.CodeProviderUnavailable, err, nil)
	default:
		return fmt.Errorf("psp: %w", err)
	}
}

func (s *service) toCVU(r store.Cvu) CVU {
	link := strings.TrimRight(s.deps.ShareBaseURL, "/") + "/" + r.Cvu
	return CVU{
		ID: r.ID, OrgID: r.OrgID, PayerID: r.PayerID, AccountID: r.AccountID, Currency: money.Currency(r.Currency),
		Number: r.Cvu, Alias: r.Alias, Active: r.Status == statusActive, ShareLink: link,
		QRPayload: link + "?" + url.Values{"currency": {r.Currency}}.Encode(), CreatedAt: r.CreatedAt,
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
