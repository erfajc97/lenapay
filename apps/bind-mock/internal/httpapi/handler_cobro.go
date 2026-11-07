package httpapi

import (
	"net/http"
	"time"

	"github.com/erfajc97/lenapay/apps/bind-mock/internal/bank"
)

type titularJSON struct {
	Name string `json:"nombre"`
	CUIT string `json:"cuit"`
}

type createCajaBody struct {
	Reference   string      `json:"referencia"`
	Currency    string      `json:"moneda"`
	Holder      titularJSON `json:"titular"`
	Alias       string      `json:"alias,omitempty"`
	RequiresCVU bool        `json:"requiereCvu"`
}

type cajaJSON struct {
	ID        string      `json:"id"`
	Reference string      `json:"referencia"`
	Currency  string      `json:"moneda"`
	CVU       string      `json:"cvu"`
	Alias     string      `json:"alias"`
	Holder    titularJSON `json:"titular"`
	Status    string      `json:"estado"`
	CreatedAt time.Time   `json:"fechaAlta"`
}

func toCajaJSON(c bank.Caja) cajaJSON {
	return cajaJSON{
		ID: c.ID, Reference: c.Reference, Currency: c.Currency, CVU: c.CVU, Alias: c.Alias,
		Holder: titularJSON{Name: c.Holder.Name, CUIT: c.Holder.CUIT}, Status: c.Status, CreatedAt: c.CreatedAt,
	}
}

// createCaja issues a CVU. The Idempotency-Key, when sent, must equal "referencia": the bank
// dedupes by it, so a retried POST returns the first caja (200) instead of a second one (201).
func (a *api) createCaja(w http.ResponseWriter, r *http.Request) {
	var body createCajaBody
	if !decode(w, r, &body) {
		return
	}
	if key := r.Header.Get("Idempotency-Key"); (key != "" && key != body.Reference) || !body.RequiresCVU {
		writeError(w, r, http.StatusBadRequest, codeInvalid, "Idempotency-Key distinto de referencia o requiereCvu falso")
		return
	}
	caja, created, err := a.Bank.CreateCaja(bank.CreateCajaInput{
		Reference: body.Reference, Currency: body.Currency, Alias: body.Alias,
		Holder: bank.Holder{Name: body.Holder.Name, CUIT: body.Holder.CUIT},
	})
	if err != nil {
		writeBankError(w, r, err, codeCajaNotFound)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, r, status, toCajaJSON(caja))
}

func (a *api) getCaja(w http.ResponseWriter, r *http.Request) {
	caja, err := a.Bank.GetCaja(r.PathValue("id"))
	if err != nil {
		writeBankError(w, r, err, codeCajaNotFound)
		return
	}
	writeJSON(w, r, http.StatusOK, toCajaJSON(caja))
}

func (a *api) updateCaja(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Alias string `json:"alias"`
	}
	if !decode(w, r, &body) {
		return
	}
	caja, err := a.Bank.UpdateAlias(r.PathValue("id"), body.Alias)
	if err != nil {
		writeBankError(w, r, err, codeCajaNotFound)
		return
	}
	writeJSON(w, r, http.StatusOK, toCajaJSON(caja))
}

func (a *api) deleteCaja(w http.ResponseWriter, r *http.Request) {
	if err := a.Bank.DisableCaja(r.PathValue("id")); err != nil {
		writeBankError(w, r, err, codeCajaNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
