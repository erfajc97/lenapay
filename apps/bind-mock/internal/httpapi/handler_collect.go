package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/erfajc97/lenapay/apps/bind-mock/internal/bank"
)

type valueJSON struct {
	Currency string       `json:"currency"`
	Amount   bank.Decimal `json:"amount"`
}

type toJSON struct {
	CBU   string `json:"cbu,omitempty"`
	Alias string `json:"alias,omitempty"`
}

type sendTransferBody struct {
	OriginID    string    `json:"origin_id"`
	To          toJSON    `json:"to"`
	Value       valueJSON `json:"value"`
	Description string    `json:"description"`
	Concept     string    `json:"concept"`
	Emails      []string  `json:"emails"`
}

type detailsJSON struct {
	Code   string `json:"codigo"`
	Detail string `json:"detalle"`
}

type counterpartyJSON struct {
	Name  string `json:"name"`
	CUIT  string `json:"cuit"`
	CBU   string `json:"cbu"`
	Alias string `json:"alias,omitempty"`
}

// transferJSON is one transfer, as the TRANSFER read and the transfers listing return it.
type transferJSON struct {
	ID           string           `json:"id"`
	OriginID     string           `json:"origin_id,omitempty"`
	Type         string           `json:"type"`
	Status       string           `json:"status"`
	Charge       chargeJSON       `json:"charge"`
	CVU          string           `json:"cvu,omitempty"`
	Counterparty counterpartyJSON `json:"counterparty"`
	Concept      string           `json:"concept,omitempty"`
	StartDate    time.Time        `json:"start_date"`
	EndDate      *time.Time       `json:"end_date,omitempty"`
	Details      *detailsJSON     `json:"details,omitempty"`
}

type chargeJSON struct {
	Value valueJSON `json:"value"`
}

func toTransferJSON(t bank.Transfer) transferJSON {
	out := transferJSON{
		ID: t.ID, OriginID: t.OriginID, Type: t.Type, Status: t.Status,
		Charge: chargeJSON{Value: valueJSON{Currency: t.Currency, Amount: t.Amount}}, CVU: t.CVU,
		Counterparty: counterpartyJSON{Name: t.Counterparty.Name, CUIT: t.Counterparty.CUIT, CBU: t.Counterparty.AccountNumber, Alias: t.Counterparty.Alias},
		Concept:      t.Concept, StartDate: t.CreatedAt, EndDate: t.CompletedAt,
	}
	if t.FailureCode != "" {
		out.Details = &detailsJSON{Code: t.FailureCode, Detail: "Transferencia rechazada"}
	}
	return out
}

// sendTransfer debits the master account. Scripted outcomes (package bank doc) decide whether
// the answer is normal, a 503 before anything happened, a 500 after the money moved, or a
// response that never comes in time.
func (a *api) sendTransfer(w http.ResponseWriter, r *http.Request) {
	var body sendTransferBody
	if !decode(w, r, &body) {
		return
	}
	if len(body.Emails) == 0 || len(body.Emails) > 10 { // BIND rejects an empty receipt list
		writeError(w, r, http.StatusBadRequest, codeInvalid, "emails es obligatorio")
		return
	}
	res, err := a.Bank.SendTransfer(bank.SendInput{
		OriginID: body.OriginID, ToAccount: body.To.CBU, ToAlias: body.To.Alias, Currency: body.Value.Currency,
		Amount: int64(body.Value.Amount), Concept: body.Concept, Description: body.Description,
	})
	if err != nil {
		writeBankError(w, r, err, codeAccountNotFound)
		return
	}
	switch res.Outcome {
	case bank.OutcomeUnavailable:
		writeError(w, r, http.StatusServiceUnavailable, codeUnavailable, "Servicio no disponible")
	case bank.OutcomeError:
		writeError(w, r, http.StatusInternalServerError, codeInternal, "Error interno")
	case bank.OutcomeTimeout:
		a.hang(w, r)
	default:
		writeJSON(w, r, http.StatusOK, toTransferJSON(res.Transfer))
	}
}

// hang holds the answer past the caller's timeout (the transfer already happened).
func (a *api) hang(w http.ResponseWriter, r *http.Request) {
	timer := time.NewTimer(a.TimeoutDelay)
	defer timer.Stop()
	select {
	case <-r.Context().Done():
	case <-timer.C:
		writeError(w, r, http.StatusGatewayTimeout, codeUnavailable, "Tiempo de espera agotado")
	}
}

func (a *api) getTransfer(w http.ResponseWriter, r *http.Request) {
	t, err := a.Bank.GetTransfer(r.PathValue("id"))
	if err != nil {
		writeBankError(w, r, err, codeTransferNotFound)
		return
	}
	writeJSON(w, r, http.StatusOK, toTransferJSON(t))
}

type listJSON struct {
	Items []transferJSON `json:"items"`
	Total int            `json:"total"`
}

// listTransfers is BIND's paginated "consultar transferencias" (start is 1-based).
func (a *api) listTransfers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := bank.TransferFilter{
		Type: q.Get("type"), Currency: q.Get("currency"), CVU: q.Get("cvu"), TransferID: q.Get("transferenceId"),
		Start: 1, Length: 50,
	}
	var bad bool
	parseDay := func(key string) time.Time {
		if q.Get(key) == "" {
			return time.Time{}
		}
		d, err := time.Parse(time.DateOnly, q.Get(key))
		bad = bad || err != nil
		return d
	}
	parseInt := func(key string, def, maxValue int) int {
		if q.Get(key) == "" {
			return def
		}
		n, err := strconv.Atoi(q.Get(key))
		bad = bad || err != nil || n < 1 || n > maxValue
		return n
	}
	f.From, f.To = parseDay("fromDate"), parseDay("toDate")
	f.Start, f.Length = parseInt("start", 1, 1_000_000), parseInt("length", 50, 500)
	if bad || (f.Type != "" && f.Type != bank.TypeCredit && f.Type != bank.TypeDebit) {
		writeError(w, r, http.StatusBadRequest, codeInvalid, "Parámetros inválidos")
		return
	}
	items, total := a.Bank.ListTransfers(f)
	out := listJSON{Items: make([]transferJSON, 0, len(items)), Total: total}
	for _, t := range items {
		out.Items = append(out.Items, toTransferJSON(t))
	}
	writeJSON(w, r, http.StatusOK, out)
}

type ownerJSON struct {
	DisplayName string `json:"display_name"`
	ID          string `json:"id"`
}

type routingJSON struct {
	Scheme  string `json:"scheme"`
	Address string `json:"address"`
}

type bankJSON struct {
	Name string `json:"name"`
}

type resolutionJSON struct {
	Owner   ownerJSON   `json:"owner"`
	Type    string      `json:"type"`
	Routing routingJSON `json:"account_routing"`
	Alias   string      `json:"alias,omitempty"`
	Bank    bankJSON    `json:"bank"`
}

func (a *api) resolveAccount(w http.ResponseWriter, r *http.Request) {
	res, err := a.Bank.ResolveAccount(r.PathValue("number"))
	a.writeResolution(w, r, res, err)
}

func (a *api) resolveAlias(w http.ResponseWriter, r *http.Request) {
	res, err := a.Bank.ResolveAlias(r.PathValue("alias"))
	a.writeResolution(w, r, res, err)
}

func (a *api) writeResolution(w http.ResponseWriter, r *http.Request, res bank.Resolution, err error) {
	if err != nil {
		writeBankError(w, r, err, codeAccountNotFound)
		return
	}
	writeJSON(w, r, http.StatusOK, resolutionJSON{
		Owner: ownerJSON{DisplayName: res.Holder.Name, ID: res.Holder.CUIT}, Type: res.Type,
		Routing: routingJSON{Scheme: "CBU", Address: res.AccountNumber}, Alias: res.Alias, Bank: bankJSON{Name: res.BankName},
	})
}

type balanceJSON struct {
	Currency string       `json:"moneda"`
	Balance  bank.Decimal `json:"saldo"`
	Date     time.Time    `json:"fecha"`
}

func (a *api) balance(w http.ResponseWriter, r *http.Request) {
	currency := r.URL.Query().Get("currency")
	if currency == "" {
		currency = "ARS"
	}
	minor, err := a.Bank.Balance(currency)
	if err != nil {
		writeBankError(w, r, err, codeAccountNotFound)
		return
	}
	writeJSON(w, r, http.StatusOK, balanceJSON{Currency: currency, Balance: bank.Decimal(minor), Date: time.Now().UTC()})
}
