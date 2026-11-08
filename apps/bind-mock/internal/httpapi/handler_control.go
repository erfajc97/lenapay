package httpapi

import (
	"context"
	"net/http"
	"time"

	"lenapay/apps/bind-mock/internal/bank"
	"lenapay/apps/bind-mock/internal/simulator"
)

type incomingBody struct {
	CVU        string `json:"cvu,omitempty"`
	Alias      string `json:"alias,omitempty"`
	Amount     string `json:"amount"` // decimal text, e.g. "150000.00"
	Currency   string `json:"currency,omitempty"`
	OriginName string `json:"originName,omitempty"`
	OriginCUIT string `json:"originCuit,omitempty"`
	OriginCBU  string `json:"originCbu,omitempty"`
}

type incomingResult struct {
	Transfer transferJSON  `json:"transfer"`
	Webhook  bank.Delivery `json:"webhook"`
}

// incoming simulates a payer transferring to one of our CVUs, then delivers the webhook.
func (a *api) incoming(w http.ResponseWriter, r *http.Request) {
	var body incomingBody
	if !decode(w, r, &body) {
		return
	}
	amount, err := bank.ParseMinor(body.Amount)
	if err != nil || len(body.OriginName) > 200 || (body.OriginCUIT != "" && (len(body.OriginCUIT) != 11 || !isDigits(body.OriginCUIT))) ||
		(body.OriginCBU != "" && !bank.ValidAccountNumber(body.OriginCBU)) {
		writeError(w, r, http.StatusBadRequest, codeInvalid, "Datos inválidos")
		return
	}
	t, d, err := a.Simulator.Receive(r.Context(), bank.IncomingInput{
		CVU: body.CVU, Alias: body.Alias, Currency: body.Currency, Amount: amount,
		Origin: bank.Counterparty{Name: body.OriginName, CUIT: body.OriginCUIT, AccountNumber: body.OriginCBU},
	})
	if err != nil {
		writeBankError(w, r, err, codeCajaNotFound)
		return
	}
	writeJSON(w, r, http.StatusCreated, incomingResult{Transfer: toTransferJSON(t), Webhook: d})
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (a *api) autoStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, a.Simulator.Status())
}

// autoPayins starts ({"enabled":true,"everySeconds":10}) or stops random demo payins. The loop
// outlives the request, so it runs on a context detached from it.
func (a *api) autoPayins(w http.ResponseWriter, r *http.Request) {
	var body simulator.AutoStatus
	if !decode(w, r, &body) {
		return
	}
	if body.Enabled && (body.EverySeconds < 1 || body.EverySeconds > 3600) {
		writeError(w, r, http.StatusBadRequest, codeInvalid, "everySeconds entre 1 y 3600")
		return
	}
	if body.Enabled {
		a.Simulator.StartAuto(context.WithoutCancel(r.Context()), time.Duration(body.EverySeconds)*time.Second)
	} else {
		a.Simulator.StopAuto()
	}
	writeJSON(w, r, http.StatusOK, a.Simulator.Status())
}

func (a *api) state(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, a.Bank.Snapshot())
}

func (a *api) reset(w http.ResponseWriter, r *http.Request) {
	a.Simulator.StopAuto()
	a.Bank.Reset()
	w.WriteHeader(http.StatusNoContent)
}

// nextTransfer scripts the outcome of the next outgoing transfer.
func (a *api) nextTransfer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Outcome bank.Outcome `json:"outcome"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !bank.ValidOutcome(body.Outcome) {
		writeError(w, r, http.StatusBadRequest, codeInvalid, "outcome: ok, rejected, pending, timeout, error o unavailable")
		return
	}
	a.Bank.SetNextOutcome(body.Outcome)
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) completeTransfer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &body) {
		return
	}
	t, err := a.Bank.CompleteTransfer(r.PathValue("id"), body.Status)
	if err != nil {
		writeBankError(w, r, err, codeTransferNotFound)
		return
	}
	writeJSON(w, r, http.StatusOK, toTransferJSON(t))
}

func (a *api) redeliver(w http.ResponseWriter, r *http.Request) {
	d, err := a.Simulator.Redeliver(r.Context(), r.PathValue("transferId"))
	if err != nil {
		writeBankError(w, r, err, codeTransferNotFound)
		return
	}
	writeJSON(w, r, http.StatusOK, d)
}
