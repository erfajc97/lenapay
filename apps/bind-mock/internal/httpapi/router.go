// Package httpapi is the HTTP surface of the fake PSP (lane C, backlog C1), shaped like BIND:
//
// COBRO (CVU issuing), prefix /bindentidad-comercio-v2/v2/api/v1.201:
//
//	POST   /cajas            create a caja with its CVU (idempotent by "referencia" = Idempotency-Key)
//	GET    /cajas/{id}       read it
//	PATCH  /cajas/{id}       change its alias
//	DELETE /cajas/{id}       disable the CVU
//
// CVUCOLLECT (master account), prefix /cvucollectentidad-financial/v1/v1.201:
//
//	POST /banks/322/accounts/owner/transaction-request-types/TRANSFER/transaction-requests
//	GET  /banks/322/accounts/owner/transaction-request-types/TRANSFER/{id}  (BIND id or origin_id;
//	     unknown → 422 codigo TX019, as the real BIND answers)
//	GET  /banks/322/accounts/transfers?type=&currency=&cvu=&transferenceId=&fromDate=&toDate=&start=&length=
//	GET  /banks/322/accounts/cbu/{number}   and   /banks/322/accounts/alias/{alias}
//	GET  /Balance?currency=ARS
//
// Control (never in the real BIND): /_mock/incoming-transfer, /_mock/auto-payins,
// /_mock/state, /_mock/reset, /_mock/next-transfer, /_mock/transfers/{id}/complete,
// /_mock/webhooks/{transferId}/redeliver. Errors are {"codigo","detalle"} like BIND's.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/erfajc97/lenapay/apps/bind-mock/internal/bank"
	"github.com/erfajc97/lenapay/apps/bind-mock/internal/simulator"
)

// Route prefixes of the two BIND products.
const (
	PrefixCobro   = "/bindentidad-comercio-v2/v2/api/v1.201"
	PrefixCollect = "/cvucollectentidad-financial/v1/v1.201"
	transferBase  = PrefixCollect + "/banks/322/accounts/owner/transaction-request-types/TRANSFER"
	accountsBase  = PrefixCollect + "/banks/322/accounts"
	maxBody       = 64 << 10
)

// Deps are what the routes need.
type Deps struct {
	Bank      *bank.Bank
	Simulator *simulator.Simulator
	// TimeoutDelay is how long a scripted "timeout" holds the answer.
	TimeoutDelay time.Duration
}

type api struct {
	Deps
}

// NewRouter returns the bind-mock routes.
func NewRouter(d Deps) http.Handler {
	a := &api{Deps: d}
	mux := http.NewServeMux()
	mux.HandleFunc(route(http.MethodGet, "/healthz"), func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc(route(http.MethodPost, PrefixCobro+"/cajas"), a.createCaja)
	mux.HandleFunc(route(http.MethodGet, PrefixCobro+"/cajas/{id}"), a.getCaja)
	mux.HandleFunc(route(http.MethodPatch, PrefixCobro+"/cajas/{id}"), a.updateCaja)
	mux.HandleFunc(route(http.MethodDelete, PrefixCobro+"/cajas/{id}"), a.deleteCaja)

	mux.HandleFunc(route(http.MethodPost, transferBase+"/transaction-requests"), a.sendTransfer)
	mux.HandleFunc(route(http.MethodGet, transferBase+"/{id}"), a.getTransfer)
	mux.HandleFunc(route(http.MethodGet, accountsBase+"/transfers"), a.listTransfers)
	mux.HandleFunc(route(http.MethodGet, accountsBase+"/cbu/{number}"), a.resolveAccount)
	mux.HandleFunc(route(http.MethodGet, accountsBase+"/alias/{alias}"), a.resolveAlias)
	mux.HandleFunc(route(http.MethodGet, PrefixCollect+"/Balance"), a.balance)

	mux.HandleFunc(route(http.MethodPost, "/_mock/incoming-transfer"), a.incoming)
	mux.HandleFunc(route(http.MethodGet, "/_mock/auto-payins"), a.autoStatus)
	mux.HandleFunc(route(http.MethodPost, "/_mock/auto-payins"), a.autoPayins)
	mux.HandleFunc(route(http.MethodGet, "/_mock/state"), a.state)
	mux.HandleFunc(route(http.MethodPost, "/_mock/reset"), a.reset)
	mux.HandleFunc(route(http.MethodPost, "/_mock/next-transfer"), a.nextTransfer)
	mux.HandleFunc(route(http.MethodPost, "/_mock/transfers/{id}/complete"), a.completeTransfer)
	mux.HandleFunc(route(http.MethodPost, "/_mock/webhooks/{transferId}/redeliver"), a.redeliver)
	return mux
}

// route builds a ServeMux pattern ("METHOD /path").
func route(method, path string) string { return method + " " + path }

// bindError is BIND's error body.
type bindError struct {
	Code   string `json:"codigo"`
	Detail string `json:"detalle"`
}

// Error codes of the mock (the TX019 of a missing transfer is BIND's real one).
const (
	codeInvalid          = "DATOS_INVALIDOS"
	codeCajaNotFound     = "CAJA_INEXISTENTE"
	codeAliasTaken       = "ALIAS_EN_USO"
	codeCajaDisabled     = "CAJA_DADA_DE_BAJA"
	codeAccountNotFound  = "CUENTA_INEXISTENTE"
	codeTransferNotFound = "TX019"
	codeUnavailable      = "SERVICIO_NO_DISPONIBLE"
	codeInternal         = "ERROR_INTERNO"
)

func writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.WarnContext(r.Context(), "write response", "err", err.Error())
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, detail string) {
	writeJSON(w, r, status, bindError{Code: code, Detail: detail})
}

// writeBankError maps the bank's sentinel errors to BIND-like answers.
func writeBankError(w http.ResponseWriter, r *http.Request, err error, notFoundCode string) {
	switch {
	case errors.Is(err, bank.ErrInvalid):
		writeError(w, r, http.StatusBadRequest, codeInvalid, "Datos inválidos")
	case errors.Is(err, bank.ErrNotFound):
		status := http.StatusNotFound
		if notFoundCode == codeTransferNotFound {
			status = http.StatusUnprocessableEntity
		}
		writeError(w, r, status, notFoundCode, "No encontrado")
	case errors.Is(err, bank.ErrAliasTaken):
		writeError(w, r, http.StatusConflict, codeAliasTaken, "El alias ya está en uso")
	case errors.Is(err, bank.ErrCajaDisabled):
		writeError(w, r, http.StatusConflict, codeCajaDisabled, "La caja está dada de baja")
	default:
		slog.ErrorContext(r.Context(), "unexpected bank error", "err", err.Error())
		writeError(w, r, http.StatusInternalServerError, codeInternal, "Error interno")
	}
}

// decode reads a strict JSON body (unknown fields rejected, size capped).
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil || !errors.Is(dec.Decode(&struct{}{}), io.EOF) {
		writeError(w, r, http.StatusBadRequest, codeInvalid, "Cuerpo inválido")
		return false
	}
	return true
}
