package main

import (
	"context"
	"errors"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/erplink"
	erpfake "github.com/erfajc97/lenapay/apps/api/internal/erplink/provider/fake"
	"github.com/erfajc97/lenapay/apps/api/internal/movements"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

// andinaCompany is the second company of "Mis pagos": approved, linked to its ERP org (the
// worker imports the projects and buyers, the persona's CUIT among them), with an ARS CVU for
// her and two of her payins, so the persona sees a company with history AND installments.
func (sd *seeder) andinaCompany(ctx context.Context) error {
	owner, err := sd.user(ctx, andinaEmail)
	if err != nil {
		return err
	}
	orgID, err := sd.approvedCompany(ctx, owner, andinaName, andinaCUIT)
	if err != nil {
		return err
	}
	sd.andina = auth.Principal{UserID: owner, OrgID: orgID, Role: auth.RoleAdmin, KYCStatus: auth.KYCApproved, StepUp: true}
	if err := sd.linkAndina(ctx); err != nil {
		return err
	}
	cvu, err := sd.andinaPersonaCVU(ctx)
	if err != nil {
		return err
	}
	have, err := sd.countOf(ctx, sd.andina, movements.TypePayin, false)
	if err != nil {
		return err
	}
	for i := have; i < andinaPayins; i++ {
		body := map[string]string{"cvu": cvu, "amount": "850000.00", "currency": "ARS", "originName": personaName, "originCuit": personaCUIT}
		if err := sd.mock(ctx, "/_mock/incoming-transfer", body); err != nil {
			return err
		}
	}
	return sd.waitFor(ctx, "Andina payins credited", func() (bool, error) {
		n, err := sd.countOf(ctx, sd.andina, movements.TypePayin, true)
		return n >= andinaPayins, err
	})
}

// linkAndina links the company to its ERP org once and waits for the worker's import.
func (sd *seeder) linkAndina(ctx context.Context) error {
	_, err := sd.s.ERPLink.Get(ctx, sd.andina)
	if apperr.IsCode(err, apperr.CodeERPLinkNotFound) {
		if err := sd.s.ERPLink.LoadIdentityIndex(ctx); err != nil {
			return err
		}
		creds := erplink.Credentials{Username: andinaERPUser, Password: config.Secret(erpfake.DevPassword)}
		_, err = sd.s.ERPLink.Link(ctx, sd.andina, creds)
	}
	if err != nil {
		return err
	}
	return sd.waitFor(ctx, "ERP import", func() (bool, error) {
		l, err := sd.s.ERPLink.Get(ctx, sd.andina)
		return l.ImportStatus == erplink.ImportCompleted, err
	})
}

// andinaPersonaCVU returns (issuing it once) the ARS CVU of the payer the ERP import created
// for the persona's CUIT.
func (sd *seeder) andinaPersonaCVU(ctx context.Context) (string, error) {
	q := personaCUIT
	found, err := sd.s.Payers.List(ctx, sd.andina, payers.ListInput{Query: &q, Page: page.Request{Limit: 5}})
	if err != nil || len(found.Items) == 0 {
		return "", errors.Join(err, errors.New("the ERP import did not create the persona's payer"))
	}
	payer, err := sd.s.Payers.Get(ctx, sd.andina, found.Items[0].ID)
	if err != nil {
		return "", err
	}
	if cvu := cvuOf(payer, money.ARS); cvu != "" {
		return cvu, nil
	}
	c, err := sd.s.Payers.CreateCVU(ctx, sd.andina, payer.ID, payers.CreateCVUInput{Currency: money.ARS, IdempotencyKey: "seed-cvu-andina-" + personaCUIT + "-ARS"})
	return c.Number, err
}

// fundPersona gives the personal wallet an ARS balance to pay with: one transfer to her
// personal CVU (issued by the worker after the KYC approval), read from her accounts.
func (sd *seeder) fundPersona(ctx context.Context) error {
	if !sd.cfg.PersonalModeEnabled {
		return nil
	}
	userID, err := sd.s.Identity.UserIDByEmail(ctx, personaEmail)
	if err != nil {
		return err
	}
	o, err := sd.personalOrg(ctx, userID)
	if err != nil {
		return err
	}
	p := auth.Principal{UserID: userID, OrgID: o.ID, Role: auth.RoleAdmin, KYCStatus: auth.KYCApproved}
	if n, err := sd.countOf(ctx, p, movements.TypePayin, false); err != nil || n > 0 {
		return err
	}
	var cvu accounts.ReceivingCVU
	if err := sd.waitFor(ctx, "personal CVU", func() (bool, error) {
		list, err := sd.s.Accounts.List(ctx, p, false) // the person's own view: Account.cvus
		for _, a := range list {
			for _, c := range a.CVUs {
				if c.Currency == money.ARS && c.Active {
					cvu = c
				}
			}
		}
		return cvu.Number != "", err
	}); err != nil {
		return err
	}
	body := map[string]string{"cvu": cvu.Number, "amount": "3000000.00", "currency": "ARS", "originName": personaName, "originCuit": personaCUIT}
	if err := sd.mock(ctx, "/_mock/incoming-transfer", body); err != nil {
		return err
	}
	return sd.waitFor(ctx, "personal wallet funded", func() (bool, error) {
		n, err := sd.countOf(ctx, p, movements.TypePayin, true)
		return n >= 1, err
	})
}
