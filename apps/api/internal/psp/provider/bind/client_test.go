package bind_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
	"github.com/erfajc97/lenapay/apps/api/internal/psp/provider/bind"
)

const (
	cajas     = "/bindentidad-comercio-v2/v2/api/v1.201/cajas"
	transfer  = "/cvucollectentidad-financial/v1/v1.201/banks/322/accounts/owner/transaction-request-types/TRANSFER"
	transfers = "/cvucollectentidad-financial/v1/v1.201/banks/322/accounts/transfers"
	cvu       = "0000322800000001000012"
)

// server answers with fn and counts the requests (BIND's wire format, as apps/bind-mock serves it).
func server(t *testing.T, fn func(w http.ResponseWriter, r *http.Request, body []byte)) (*bind.Client, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		fn(w, r, body)
	}))
	t.Cleanup(srv.Close)
	return bind.New(bind.Options{BaseURL: srv.URL + "/", Timeout: 2 * time.Second}), &hits
}

func reply(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func TestCreateCVU(t *testing.T) {
	t.Parallel()
	alias := "torre.norte"
	c, hits := server(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		var got map[string]any
		_ = json.Unmarshal(body, &got)
		holder := got["titular"].(map[string]any)
		if r.Method != http.MethodPost || r.URL.Path != cajas || r.Header.Get("Idempotency-Key") != "ref-1" ||
			got["referencia"] != "ref-1" || got["moneda"] != "USD" || got["requiereCvu"] != true || got["alias"] != alias ||
			holder["nombre"] != "Juan Pérez" || holder["cuit"] != "20123456786" {
			reply(w, http.StatusBadRequest, `{"codigo":"DATOS_INVALIDOS"}`)
			return
		}
		reply(w, http.StatusCreated, `{"id":"caja-1","cvu":"`+cvu+`","alias":"torre.norte","estado":"ACTIVA"}`)
	})
	got, err := c.CreateCVU(t.Context(), psp.CreateCVURequest{Reference: "ref-1", Currency: money.USD, HolderName: "Juan Pérez", HolderDoc: "20123456786", Alias: &alias})
	if err != nil || got != (psp.CVUAccount{CVU: cvu, Alias: alias, ProviderRef: "caja-1"}) || hits.Load() != 1 {
		t.Fatalf("got %+v err %v hits %d", got, err, hits.Load())
	}
}

// Each BIND answer maps to exactly one port error; a POST is never retried.
func TestErrorMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		status   int
		body     string
		want     error
		notWant  error
		wantHits int32
	}{
		{"alias taken is also a rejection", http.StatusConflict, `{"codigo":"ALIAS_EN_USO"}`, psp.ErrAliasTaken, psp.ErrUncertain, 1},
		{"other conflict is a plain rejection", http.StatusConflict, `{"codigo":"CAJA_DADA_DE_BAJA"}`, psp.ErrRejected, psp.ErrAliasTaken, 1},
		{"bad request is rejected", http.StatusBadRequest, `{"codigo":"DATOS_INVALIDOS"}`, psp.ErrRejected, psp.ErrUncertain, 1},
		{"5xx is uncertain, not retried on POST", http.StatusBadGateway, `oops`, psp.ErrUncertain, psp.ErrRejected, 1},
		{"429 is uncertain", http.StatusTooManyRequests, ``, psp.ErrUncertain, psp.ErrRejected, 1},
		{"2xx without a cvu is uncertain", http.StatusCreated, `{"id":"caja-1"}`, psp.ErrUncertain, psp.ErrRejected, 1},
		{"2xx not JSON is uncertain", http.StatusCreated, `<html>`, psp.ErrUncertain, psp.ErrRejected, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, hits := server(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { reply(w, tc.status, tc.body) })
			_, err := c.CreateCVU(t.Context(), psp.CreateCVURequest{Reference: "r", Currency: money.ARS, HolderName: "X"})
			if !errors.Is(err, tc.want) || errors.Is(err, tc.notWant) || hits.Load() != tc.wantHits {
				t.Fatalf("err %v hits %d", err, hits.Load())
			}
		})
	}
}

// Reads are retried on 5xx (three attempts in all) and then uncertain; a recovered read succeeds.
func TestReadsAreRetried(t *testing.T) {
	t.Parallel()
	c, hits := server(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { reply(w, http.StatusServiceUnavailable, ``) })
	if _, err := c.TransferByReference(t.Context(), "ref-1"); !errors.Is(err, psp.ErrUncertain) || hits.Load() != 3 {
		t.Fatalf("err %v hits %d", err, hits.Load())
	}
	c, hits = server(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		if hits.Load() == 1 {
			reply(w, http.StatusBadGateway, ``)
			return
		}
		reply(w, http.StatusOK, `{"id":"trf-1","origin_id":"ref-1","status":"COMPLETED","charge":{"value":{"currency":"ARS","amount":10.00}}}`)
	})
	got, err := c.TransferByReference(t.Context(), "ref-1")
	if err != nil || got.Status != psp.TransferSettled || got.ProviderID != "trf-1" || hits.Load() != 2 {
		t.Fatalf("got %+v err %v hits %d", got, err, hits.Load())
	}
}

func TestTransferByReferenceStatuses(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		status int
		body   string
		want   psp.TransferResult
		err    error
	}{
		"settled":        {200, `{"id":"t","status":"COMPLETED"}`, psp.TransferResult{ProviderID: "t", Status: psp.TransferSettled}, nil},
		"pending":        {200, `{"id":"t","status":"IN_PROGRESS"}`, psp.TransferResult{ProviderID: "t", Status: psp.TransferPending}, nil},
		"failed, no id":  {200, `{"status":"FAILED","details":{"codigo":"SALDO_INSUFICIENTE"}}`, psp.TransferResult{Status: psp.TransferRejected, Reason: "SALDO_INSUFICIENTE"}, nil},
		"settled, no id": {200, `{"status":"COMPLETED"}`, psp.TransferResult{}, psp.ErrUncertain},
		"unknown status": {200, `{"id":"t","status":"WHATEVER"}`, psp.TransferResult{}, psp.ErrUncertain},
		"TX019":          {422, `{"codigo":"TX019","detalle":"Transferencia no encontrada"}`, psp.TransferResult{}, psp.ErrNotFound},
		"other 422":      {422, `{"codigo":"OTRO"}`, psp.TransferResult{}, psp.ErrRejected},
		"404":            {404, ``, psp.TransferResult{}, psp.ErrNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, _ := server(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				if r.Method != http.MethodGet || r.URL.Path != transfer+"/ref-1" {
					reply(w, http.StatusBadRequest, `{}`)
					return
				}
				reply(w, tc.status, tc.body)
			})
			got, err := c.TransferByReference(t.Context(), "ref-1")
			if (tc.err == nil && err != nil) || (tc.err != nil && !errors.Is(err, tc.err)) || got != tc.want {
				t.Fatalf("got %+v err %v", got, err)
			}
		})
	}
}

func TestSendTransfer(t *testing.T) {
	t.Parallel()
	var sent string
	c, hits := server(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		sent = string(body)
		if r.URL.Path != transfer+"/transaction-requests" || r.Header.Get("Idempotency-Key") != "attempt-1" {
			reply(w, http.StatusBadRequest, `{}`)
			return
		}
		reply(w, http.StatusOK, `{"id":"trf-9","origin_id":"attempt-1","status":"IN_PROGRESS","charge":{"value":{"currency":"ARS","amount":1500.25}}}`)
	})
	req := psp.SendTransferRequest{Reference: "attempt-1", AccountNumber: "0070999800000000000425", Amount: money.New(150025, money.ARS), Concept: "Pago obra"}
	got, err := c.SendTransfer(t.Context(), req)
	if err != nil || got.Status != psp.TransferPending || got.ProviderID != "trf-9" || hits.Load() != 1 {
		t.Fatalf("got %+v err %v", got, err)
	}
	for _, want := range []string{`"origin_id":"attempt-1"`, `"amount":1500.25`, `"cbu":"0070999800000000000425"`, `"emails":["`} {
		if !strings.Contains(sent, want) {
			t.Fatalf("request %s lacks %s", sent, want)
		}
	}
	// A different echoed amount is not trusted either way.
	c, _ = server(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		reply(w, http.StatusOK, `{"id":"trf-9","status":"COMPLETED","charge":{"value":{"currency":"ARS","amount":1500.24}}}`)
	})
	if _, err := c.SendTransfer(t.Context(), req); !errors.Is(err, psp.ErrUncertain) {
		t.Fatalf("echo mismatch err %v", err)
	}
}

// The caller's deadline bounds the whole call, retries included.
func TestCallerDeadline(t *testing.T) {
	t.Parallel()
	c, _ := server(t, func(_ http.ResponseWriter, r *http.Request, _ []byte) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := c.GetTransfer(ctx, "trf-1"); !errors.Is(err, psp.ErrUncertain) || time.Since(start) > time.Second {
		t.Fatalf("err %v after %s", err, time.Since(start))
	}
}

func TestDisableCVUTreatsGoneAsDone(t *testing.T) {
	t.Parallel()
	c, _ := server(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if r.Method != http.MethodDelete || r.URL.Path != cajas+"/caja-1" {
			reply(w, http.StatusBadRequest, `{}`)
			return
		}
		reply(w, http.StatusNotFound, `{"codigo":"CAJA_INEXISTENTE"}`)
	})
	if err := c.DisableCVU(t.Context(), "caja-1"); err != nil {
		t.Fatalf("err %v", err)
	}
	c, _ = server(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		if r.Method != http.MethodPatch || string(body) != `{"alias":"nuevo.alias"}` {
			reply(w, http.StatusBadRequest, `{}`)
			return
		}
		reply(w, http.StatusNotFound, `{"codigo":"CAJA_INEXISTENTE"}`)
	})
	if err := c.UpdateAlias(t.Context(), "caja-1", "nuevo.alias"); !errors.Is(err, psp.ErrNotFound) {
		t.Fatalf("update alias of unknown caja err %v", err)
	}
}

func TestGetTransfer(t *testing.T) {
	t.Parallel()
	item := func(status, amount string) string {
		return `{"items":[{"id":"trf-1","type":"CREDIT","status":"` + status + `","cvu":"` + cvu +
			`","charge":{"value":{"currency":"ARS","amount":` + amount + `}},"counterparty":{"name":"Ana","cuit":"27111111110","cbu":"0070999800000000000425"},` +
			`"start_date":"2026-11-07T15:00:00Z","end_date":"2026-11-07T15:00:01Z"}],"total":1}`
	}
	cases := map[string]struct {
		body string
		err  error
	}{
		"completed":    {item("COMPLETED", "150000.50"), nil},
		"in progress":  {item("IN_PROGRESS", "1"), psp.ErrUncertain},
		"failed":       {item("FAILED", "1"), psp.ErrNotFound},
		"none":         {`{"items":[],"total":0}`, psp.ErrNotFound},
		"bad amount":   {item("COMPLETED", "1.234"), psp.ErrUncertain},
		"bad currency": {strings.Replace(item("COMPLETED", "1"), `"ARS"`, `"EUR"`, 1), psp.ErrUncertain},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, _ := server(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
				q := r.URL.Query()
				if r.URL.Path != transfers || q.Get("type") != "CREDIT" || q.Get("transferenceId") != "trf-1" {
					reply(w, http.StatusBadRequest, `{}`)
					return
				}
				reply(w, http.StatusOK, tc.body)
			})
			got, err := c.GetTransfer(t.Context(), "trf-1")
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("err %v, want %v", err, tc.err)
				}
				return
			}
			want := psp.IncomingTransfer{
				ID: "trf-1", CVU: cvu, Amount: money.New(15_000_050, money.ARS), OriginName: "Ana", OriginCUIT: "27111111110",
				OriginCBU: "0070999800000000000425", OccurredAt: time.Date(2026, 11, 7, 15, 0, 1, 0, time.UTC),
			}
			if err != nil || got != want {
				t.Fatalf("got %+v err %v", got, err)
			}
		})
	}
}

func TestResolveDestination(t *testing.T) {
	t.Parallel()
	c, _ := server(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		switch r.URL.Path {
		case "/cvucollectentidad-financial/v1/v1.201/banks/322/accounts/cbu/" + cvu:
			reply(w, http.StatusOK, `{"owner":{"display_name":"Juan","id":"20123456786"},"type":"CVU","account_routing":{"scheme":"CBU","address":"`+cvu+`"},"bank":{"name":"BIND"}}`)
		case "/cvucollectentidad-financial/v1/v1.201/banks/322/accounts/alias/proveedor.sur":
			reply(w, http.StatusOK, `{"owner":{"display_name":"Sur SA","id":"30123456781"},"type":"CBU","account_routing":{"scheme":"CBU","address":"0070999800000000000425"},"bank":{"name":"Galicia"}}`)
		default:
			reply(w, http.StatusNotFound, `{"codigo":"CUENTA_INEXISTENTE"}`)
		}
	})
	d, err := c.ResolveDestination(t.Context(), cvu)
	if err != nil || d.Type != psp.DestinationCVU || d.HolderName != "Juan" || d.AccountNumber != cvu || d.Value != cvu {
		t.Fatalf("cvu %+v %v", d, err)
	}
	d, err = c.ResolveDestination(t.Context(), "proveedor.sur")
	if err != nil || d.Type != psp.DestinationAlias || d.AccountNumber != "0070999800000000000425" || d.HolderCUIT != "30123456781" || d.BankName != "Galicia" {
		t.Fatalf("alias %+v %v", d, err)
	}
	if _, err := c.ResolveDestination(t.Context(), "noexiste.x"); !errors.Is(err, psp.ErrNotFound) {
		t.Fatalf("unknown err %v", err)
	}
}

// The statement walks every page (start is 1-based) of the day.
func TestStatementPaginates(t *testing.T) {
	t.Parallel()
	c, hits := server(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		q := r.URL.Query()
		if q.Get("fromDate") != "2026-11-07" || q.Get("toDate") != "2026-11-07" || q.Get("currency") != "USD" || q.Get("length") != "500" {
			reply(w, http.StatusBadRequest, `{}`)
			return
		}
		if q.Get("start") == "1" {
			items := make([]string, 500)
			for i := range items {
				items[i] = `{"id":"c","type":"CREDIT","status":"COMPLETED","charge":{"value":{"currency":"USD","amount":1}}}`
			}
			reply(w, http.StatusOK, `{"items":[`+strings.Join(items, ",")+`],"total":501}`)
			return
		}
		reply(w, http.StatusOK, `{"items":[{"id":"d","type":"DEBIT","status":"COMPLETED","charge":{"value":{"currency":"USD","amount":2.5}}}],"total":501}`)
	})
	lines, err := c.Statement(t.Context(), money.USD, time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC))
	if err != nil || len(lines) != 501 || hits.Load() != 2 {
		t.Fatalf("lines %d err %v hits %d", len(lines), err, hits.Load())
	}
	last := lines[500]
	if last.Direction != psp.DirectionOut || last.Amount != money.New(250, money.USD) || lines[0].Direction != psp.DirectionIn {
		t.Fatalf("last %+v first %+v", last, lines[0])
	}
}
