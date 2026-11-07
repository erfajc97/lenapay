// Package bind is the HTTP adapter of the psp.Provider port for BIND (COBRO for CVUs,
// CVUCOLLECT for the master account and transfers). It is the only code that knows BIND's
// paths and JSON; locally it talks to apps/bind-mock, which serves the same wire format.
//
// Failure semantics (AG8, MO4): one http.Client with every timeout set; only idempotent calls
// (GET, DELETE) are retried, on network errors, 429 and 5xx, with jittered backoff inside the
// caller's deadline. A POST or PATCH is sent once: a timeout, network error or 5xx is
// psp.ErrUncertain and the caller re-reads by our reference (BIND dedupes by origin_id and by
// the caja "referencia"), never "failed".
package bind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// BIND paths (bank 322 = Banco Industrial).
const (
	prefixCobro    = "/bindentidad-comercio-v2/v2/api/v1.201"
	prefixCollect  = "/cvucollectentidad-financial/v1/v1.201"
	pathCajas      = prefixCobro + "/cajas"
	pathTransfer   = prefixCollect + "/banks/322/accounts/owner/transaction-request-types/TRANSFER"
	pathTransfers  = prefixCollect + "/banks/322/accounts/transfers"
	pathResolveCBU = prefixCollect + "/banks/322/accounts/cbu/"
	pathResolveAli = prefixCollect + "/banks/322/accounts/alias/"
)

const (
	maxAttempts   = 3
	maxBodyBytes  = 1 << 20
	statementPage = 500
	// defaultReceiptEmail receives BIND's transfer receipts (BIND rejects an empty list).
	defaultReceiptEmail = "comprobantes@lenapay.app"
)

// Options configure the adapter.
type Options struct {
	BaseURL string        // e.g. http://bind-mock:8090 (validated by config)
	Timeout time.Duration // per attempt, from PSP_TIMEOUT
}

// Client implements psp.Provider over BIND's HTTP API.
type Client struct {
	base string
	http *http.Client
}

var _ psp.Provider = (*Client)(nil)

// New builds the adapter with its own timed client (never http.DefaultClient).
func New(o Options) *Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: o.Timeout,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   16,
	}
	return &Client{base: strings.TrimRight(o.BaseURL, "/"), http: &http.Client{Timeout: o.Timeout, Transport: transport}}
}

// request is one logical call.
type request struct {
	method         string
	path           string
	query          url.Values
	body           any
	idempotencyKey string
}

func (r request) retryable() bool { return r.method == http.MethodGet || r.method == http.MethodDelete }

// send runs the call (retrying idempotent ones) and returns the final status and body. err is
// set only when no answer was obtained: it already wraps psp.ErrUncertain.
func (c *Client) send(ctx context.Context, req request) (int, []byte, error) {
	var payload []byte
	if req.body != nil {
		var err error
		if payload, err = json.Marshal(req.body); err != nil {
			return 0, nil, fmt.Errorf("bind: encode: %w", err)
		}
	}
	attempts := 1
	if req.retryable() {
		attempts = maxAttempts
	}
	var (
		status int
		body   []byte
		err    error
	)
	for attempt := 1; attempt <= attempts; attempt++ {
		status, body, err = c.once(ctx, req, payload)
		if err == nil && status != http.StatusTooManyRequests && status < http.StatusInternalServerError {
			return status, body, nil
		}
		if attempt < attempts {
			wait := time.Duration(attempt)*200*time.Millisecond + rand.N(100*time.Millisecond) //nolint:gosec // jitter, not a secret
			select {
			case <-ctx.Done():
				return 0, nil, fmt.Errorf("%w: bind %s %s: %w", psp.ErrUncertain, req.method, req.path, ctx.Err())
			case <-time.After(wait):
			}
		}
	}
	if err != nil {
		return 0, nil, fmt.Errorf("%w: bind %s %s: %w", psp.ErrUncertain, req.method, req.path, err)
	}
	return status, body, nil
}

func (c *Client) once(ctx context.Context, req request, payload []byte) (int, []byte, error) {
	target := c.base + req.path
	if len(req.query) > 0 {
		target += "?" + req.query.Encode()
	}
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.method, target, body)
	if err != nil {
		return 0, nil, fmt.Errorf("request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	if payload != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if req.idempotencyKey != "" {
		httpReq.Header.Set("Idempotency-Key", req.idempotencyKey)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return 0, nil, err //nolint:wrapcheck // wrapped once by send with the operation
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("read: %w", err)
	}
	return resp.StatusCode, raw, nil
}

// classify maps a non-2xx answer to the port's errors.
func classify(op string, status int, body []byte) error {
	var e errorBody
	_ = json.Unmarshal(body, &e) // a non-JSON error body only loses the code
	switch {
	case status == http.StatusNotFound, status == http.StatusUnprocessableEntity && e.Code == codeTransferNotFound:
		return fmt.Errorf("%w: bind %s", psp.ErrNotFound, op)
	case status == http.StatusConflict && e.Code == codeAliasTaken:
		return fmt.Errorf("%w: bind %s", psp.ErrAliasTaken, op)
	case status == http.StatusTooManyRequests || status >= http.StatusInternalServerError:
		return fmt.Errorf("%w: bind %s answered %d", psp.ErrUncertain, op, status)
	default:
		return fmt.Errorf("%w: bind %s answered %d %s", psp.ErrRejected, op, status, e.Code)
	}
}

func (c *Client) json(ctx context.Context, op string, req request, out any) error {
	status, body, err := c.send(ctx, req)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return classify(op, status, body)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		// 2xx we cannot read: the effect may have happened.
		return fmt.Errorf("%w: bind %s: decode: %w", psp.ErrUncertain, op, err)
	}
	return nil
}

// CreateCVU issues a caja with its CVU; BIND dedupes by "referencia" (= Idempotency-Key).
func (c *Client) CreateCVU(ctx context.Context, req psp.CreateCVURequest) (psp.CVUAccount, error) {
	body := createCajaRequest{
		Reference: req.Reference, Currency: string(req.Currency),
		Holder: holderJSON{Name: req.HolderName, CUIT: req.HolderDoc}, RequiresCVU: true,
	}
	if req.Alias != nil {
		body.Alias = *req.Alias
	}
	var out cajaResponse
	if err := c.json(ctx, "create caja", request{method: http.MethodPost, path: pathCajas, body: body, idempotencyKey: req.Reference}, &out); err != nil {
		return psp.CVUAccount{}, err
	}
	if out.ID == "" || len(out.CVU) != 22 {
		return psp.CVUAccount{}, fmt.Errorf("%w: bind create caja: answer without id or cvu", psp.ErrUncertain)
	}
	return psp.CVUAccount{CVU: out.CVU, Alias: out.Alias, ProviderRef: out.ID}, nil
}

// UpdateAlias changes the alias of a caja (PATCH: not retried; BIND throttles alias changes).
func (c *Client) UpdateAlias(ctx context.Context, providerRef, alias string) error {
	return c.json(ctx, "update alias", request{
		method: http.MethodPatch, path: pathCajas + "/" + url.PathEscape(providerRef), body: map[string]string{"alias": alias},
	}, nil)
}

// DisableCVU disables a caja. A 404 is success: a retried DELETE finds it already gone.
func (c *Client) DisableCVU(ctx context.Context, providerRef string) error {
	err := c.json(ctx, "disable caja", request{method: http.MethodDelete, path: pathCajas + "/" + url.PathEscape(providerRef)}, nil)
	if errors.Is(err, psp.ErrNotFound) {
		return nil
	}
	return err
}

// GetTransfer re-reads an incoming transfer in the master account listing (MO4). Only a
// COMPLETED credit is returned; a credit still in progress is uncertain (ask again later).
func (c *Client) GetTransfer(ctx context.Context, transferID string) (psp.IncomingTransfer, error) {
	var out transferList
	q := url.Values{"type": {typeCredit}, "transferenceId": {transferID}, "start": {"1"}, "length": {"1"}}
	if err := c.json(ctx, "get transfer", request{method: http.MethodGet, path: pathTransfers, query: q}, &out); err != nil {
		return psp.IncomingTransfer{}, err
	}
	if len(out.Items) == 0 || out.Items[0].ID != transferID {
		return psp.IncomingTransfer{}, fmt.Errorf("%w: bind transfer %s", psp.ErrNotFound, transferID)
	}
	t := out.Items[0]
	switch t.Status {
	case statusCompleted:
	case statusFailed:
		return psp.IncomingTransfer{}, fmt.Errorf("%w: bind transfer %s failed", psp.ErrNotFound, transferID)
	default:
		return psp.IncomingTransfer{}, fmt.Errorf("%w: bind transfer %s is %s", psp.ErrUncertain, transferID, t.Status)
	}
	currency, err := money.ParseCurrency(t.Charge.Value.Currency)
	if err != nil {
		return psp.IncomingTransfer{}, fmt.Errorf("%w: bind transfer %s: %w", psp.ErrUncertain, transferID, err)
	}
	return psp.IncomingTransfer{
		ID: t.ID, CVU: t.CVU, Amount: money.New(int64(t.Charge.Value.Amount), currency),
		OriginName: t.Counterparty.Name, OriginCUIT: t.Counterparty.CUIT, OriginCBU: t.Counterparty.CBU,
		OccurredAt: occurredAt(t),
	}, nil
}

func occurredAt(t transferResponse) time.Time {
	if t.EndDate != nil {
		return t.EndDate.UTC()
	}
	return t.StartDate.UTC()
}

// ResolveDestination looks up a 22-digit CBU/CVU or an alias and its holder.
func (c *Client) ResolveDestination(ctx context.Context, to string) (psp.Destination, error) {
	byNumber := len(to) == 22 && allDigits(to)
	path := pathResolveAli + url.PathEscape(to)
	if byNumber {
		path = pathResolveCBU + to
	}
	var out resolutionResponse
	if err := c.json(ctx, "resolve destination", request{method: http.MethodGet, path: path}, &out); err != nil {
		return psp.Destination{}, err
	}
	typed := psp.DestinationAlias
	switch {
	case byNumber && out.Type == "CVU":
		typed = psp.DestinationCVU
	case byNumber:
		typed = psp.DestinationCBU
	}
	return psp.Destination{
		Type: typed, Value: to, AccountNumber: out.Routing.Address,
		HolderName: out.Owner.DisplayName, HolderCUIT: out.Owner.ID, BankName: out.Bank.Name,
	}, nil
}

// SendTransfer starts an outgoing transfer with origin_id = our per-attempt reference.
func (c *Client) SendTransfer(ctx context.Context, req psp.SendTransferRequest) (psp.TransferResult, error) {
	body := transferRequest{
		OriginID: req.Reference, To: map[string]string{"cbu": req.AccountNumber},
		Value:       valueJSON{Currency: string(req.Amount.Currency), Amount: decimal(req.Amount.Minor)},
		Description: req.Concept, Concept: "VAR", Emails: []string{defaultReceiptEmail},
	}
	var out transferResponse
	if err := c.json(ctx, "send transfer", request{
		method: http.MethodPost, path: pathTransfer + "/transaction-requests", body: body, idempotencyKey: req.Reference,
	}, &out); err != nil {
		return psp.TransferResult{}, err
	}
	if out.Status != statusFailed && int64(out.Charge.Value.Amount) != req.Amount.Minor {
		// BIND echoed another amount: do not trust either side, re-read later.
		return psp.TransferResult{}, fmt.Errorf("%w: bind send transfer: echoed amount differs", psp.ErrUncertain)
	}
	return toResult(out)
}

// TransferByReference re-reads an outgoing transfer by our origin_id (BIND's read accepts it).
func (c *Client) TransferByReference(ctx context.Context, reference string) (psp.TransferResult, error) {
	var out transferResponse
	if err := c.json(ctx, "transfer by reference", request{method: http.MethodGet, path: pathTransfer + "/" + url.PathEscape(reference)}, &out); err != nil {
		return psp.TransferResult{}, err
	}
	return toResult(out)
}

func toResult(t transferResponse) (psp.TransferResult, error) {
	res := psp.TransferResult{ProviderID: t.ID}
	switch t.Status {
	case statusCompleted:
		res.Status = psp.TransferSettled
	case statusInProgress:
		res.Status = psp.TransferPending
	case statusFailed:
		res.Status = psp.TransferRejected
		if t.Details != nil {
			res.Reason = t.Details.Code
		}
	default:
		return psp.TransferResult{}, fmt.Errorf("%w: bind transfer status %q", psp.ErrUncertain, t.Status)
	}
	if res.Status != psp.TransferRejected && t.ID == "" {
		return psp.TransferResult{}, fmt.Errorf("%w: bind transfer without id", psp.ErrUncertain)
	}
	return res, nil
}

// Statement lists the COMPLETED movements of the master account on day (BIND's local date).
func (c *Client) Statement(ctx context.Context, currency money.Currency, day time.Time) ([]psp.StatementLine, error) {
	date := day.Format(time.DateOnly)
	var lines []psp.StatementLine
	for start := 1; ; start += statementPage {
		q := url.Values{
			"currency": {string(currency)}, "fromDate": {date}, "toDate": {date},
			"start": {fmt.Sprint(start)}, "length": {fmt.Sprint(statementPage)},
		}
		var out transferList
		if err := c.json(ctx, "statement", request{method: http.MethodGet, path: pathTransfers, query: q}, &out); err != nil {
			return nil, err
		}
		for _, t := range out.Items {
			dir := psp.DirectionOut
			if t.Type == typeCredit {
				dir = psp.DirectionIn
			}
			lines = append(lines, psp.StatementLine{
				TransferID: t.ID, Direction: dir, Amount: money.New(int64(t.Charge.Value.Amount), currency), OccurredAt: occurredAt(t),
			})
		}
		if len(out.Items) == 0 || start+len(out.Items) > out.Total {
			return lines, nil
		}
	}
}
