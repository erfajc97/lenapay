// Package bank is the in-memory state of the fake BIND: cajas with their CVU and alias
// (COBRO), the master account per currency, incoming and outgoing transfers (CVUCOLLECT) and
// the webhook deliveries. Every mutation is saved to a JSON file when one is configured, so a
// restart keeps the demo data.
//
// Scripted behaviour (documented for testers):
//   - Outgoing transfer outcome by the cents of the amount: .13 FAILED (rejected), .96 503 before
//     anything happens, .97 IN_PROGRESS until PendingDelay passes (or /_mock/transfers/{id}/complete),
//     .98 executed but the answer hangs past the caller's timeout, .99 executed but answers 500.
//     /_mock/next-transfer overrides the next one.
//   - A valid CBU/CVU whose 13-digit account starts with 9999, or an alias starting with
//     "noexiste", is not found; any other valid number or alias resolves to a deterministic holder.
package bank

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Errors of the bank; the HTTP layer maps them to BIND's status codes.
var (
	ErrNotFound     = errors.New("bank: not found")
	ErrInvalid      = errors.New("bank: invalid request")
	ErrAliasTaken   = errors.New("bank: alias taken")
	ErrCajaDisabled = errors.New("bank: caja disabled")
)

// Currencies the master account holds (a function: nothing can change the list at runtime).
func Currencies() []string { return []string{"ARS", "USD"} }

// Caja statuses.
const (
	CajaActive   = "ACTIVA"
	CajaDisabled = "BAJA"
)

// Caja is a collection box with its CVU (BIND COBRO).
type Caja struct {
	ID        string    `json:"id"`
	Reference string    `json:"referencia"`
	Currency  string    `json:"moneda"`
	CVU       string    `json:"cvu"`
	Alias     string    `json:"alias"`
	Holder    Holder    `json:"titular"`
	Status    string    `json:"estado"`
	CreatedAt time.Time `json:"fechaAlta"`
}

// Transfer directions and statuses (BIND's words).
const (
	TypeCredit = "CREDIT"
	TypeDebit  = "DEBIT"

	StatusCompleted  = "COMPLETED"
	StatusInProgress = "IN_PROGRESS"
	StatusFailed     = "FAILED"
)

// Counterparty is the other side of a transfer.
type Counterparty struct {
	Name          string `json:"name"`
	CUIT          string `json:"cuit"`
	AccountNumber string `json:"cbu"`
	Alias         string `json:"alias,omitempty"`
}

// Transfer is a movement of the master account.
type Transfer struct {
	ID           string       `json:"id"`
	OriginID     string       `json:"origin_id,omitempty"` // our reference, outgoing only
	Type         string       `json:"type"`
	Status       string       `json:"status"`
	Currency     string       `json:"currency"`
	Amount       Decimal      `json:"amount"`
	CVU          string       `json:"cvu,omitempty"` // receiving CVU of a credit
	Counterparty Counterparty `json:"counterparty"`
	Concept      string       `json:"concept,omitempty"`
	Description  string       `json:"description,omitempty"`
	FailureCode  string       `json:"failure_code,omitempty"`
	CreatedAt    time.Time    `json:"start_date"`
	CompletedAt  *time.Time   `json:"end_date,omitempty"`
}

// Delivery is one webhook attempt.
type Delivery struct {
	EventID    string    `json:"eventId"`
	TransferID string    `json:"transferId"`
	At         time.Time `json:"at"`
	Status     int       `json:"status"`
	Error      string    `json:"error,omitempty"`
}

// State is everything the bank remembers (and the JSON file holds).
type State struct {
	Seq        int64            `json:"seq"`
	Cajas      map[string]*Caja `json:"cajas"`
	Transfers  []*Transfer      `json:"transfers"`
	Balances   map[string]int64 `json:"balancesMinor"`
	Deliveries []Delivery       `json:"deliveries"`
}

// Options configure a bank.
type Options struct {
	StateFile           string
	Location            *time.Location
	PendingDelay        time.Duration
	InitialBalanceMinor map[string]int64
	Now                 func() time.Time
}

// Bank is safe for concurrent use.
type Bank struct {
	mu          sync.Mutex
	state       State
	opts        Options
	nextOutcome []Outcome
}

// New loads the state file when it exists, or starts with the initial balances.
func New(opts Options) (*Bank, error) {
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if opts.Location == nil {
		opts.Location = time.UTC
	}
	b := &Bank{opts: opts, state: freshState(opts.InitialBalanceMinor)}
	if opts.StateFile == "" {
		return b, nil
	}
	raw, err := os.ReadFile(opts.StateFile)
	if errors.Is(err, os.ErrNotExist) {
		return b, nil
	}
	if err != nil {
		return nil, fmt.Errorf("bank: read state: %w", err)
	}
	if err := json.Unmarshal(raw, &b.state); err != nil {
		return nil, fmt.Errorf("bank: decode state: %w", err)
	}
	if b.state.Cajas == nil {
		b.state.Cajas = map[string]*Caja{}
	}
	return b, nil
}

func freshState(initial map[string]int64) State {
	balances := map[string]int64{}
	for _, c := range Currencies() {
		balances[c] = initial[c]
	}
	return State{Cajas: map[string]*Caja{}, Balances: balances}
}

// Reset wipes the bank back to its initial balances (demo control).
func (b *Bank) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = freshState(b.opts.InitialBalanceMinor)
	b.nextOutcome = nil
	b.save()
}

// Snapshot returns a deep copy of the state (for /_mock/state and tests).
func (b *Bank) Snapshot() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	raw, _ := json.Marshal(b.state) // plain data: cannot fail
	var out State
	_ = json.Unmarshal(raw, &out)
	return out
}

// save writes the state atomically (temp file + rename). Called with b.mu held. A failure is
// logged and the bank keeps serving from memory: the file is a convenience of the demo.
func (b *Bank) save() {
	if b.opts.StateFile == "" {
		return
	}
	raw, err := json.MarshalIndent(b.state, "", "  ")
	if err == nil {
		tmp := b.opts.StateFile + ".tmp"
		if err = os.MkdirAll(filepath.Dir(b.opts.StateFile), 0o750); err == nil {
			if err = os.WriteFile(tmp, raw, 0o600); err == nil {
				err = os.Rename(tmp, b.opts.StateFile)
			}
		}
	}
	if err != nil {
		slog.Warn("bank: state not saved", "err", err.Error())
	}
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf) // crypto/rand never fails on supported platforms
	return prefix + hex.EncodeToString(buf)
}

// --- COBRO: cajas and CVUs ---

// CreateCajaInput asks for a caja with a CVU.
type CreateCajaInput struct {
	Reference string
	Currency  string
	Holder    Holder
	Alias     string
}

// CreateCaja issues a CVU. It is idempotent by Reference: the same reference returns the
// caja created the first time (created = false), whatever else the request says.
func (b *Bank) CreateCaja(in CreateCajaInput) (caja Caja, created bool, err error) {
	if in.Reference == "" || len(in.Reference) > 120 || !slices.Contains(Currencies(), in.Currency) ||
		in.Holder.Name == "" || len(in.Holder.Name) > 200 || (in.Alias != "" && !ValidAlias(in.Alias)) {
		return Caja{}, false, ErrInvalid
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, c := range b.state.Cajas {
		if c.Reference == in.Reference {
			return *c, false, nil
		}
	}
	b.state.Seq++
	alias := in.Alias
	if alias == "" {
		alias = newAlias(b.state.Seq)
	}
	if b.aliasOwner(alias) != nil {
		return Caja{}, false, ErrAliasTaken
	}
	c := &Caja{
		ID: newID("caja-"), Reference: in.Reference, Currency: in.Currency, CVU: newCVU(b.state.Seq),
		Alias: alias, Holder: in.Holder, Status: CajaActive, CreatedAt: b.opts.Now(),
	}
	b.state.Cajas[c.ID] = c
	b.save()
	return *c, true, nil
}

func (b *Bank) aliasOwner(alias string) *Caja {
	for _, c := range b.state.Cajas {
		if strings.EqualFold(c.Alias, alias) {
			return c
		}
	}
	return nil
}

// GetCaja returns a caja by id.
func (b *Bank) GetCaja(id string) (Caja, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c, ok := b.state.Cajas[id]
	if !ok {
		return Caja{}, ErrNotFound
	}
	return *c, nil
}

// UpdateAlias changes the alias of an active caja.
func (b *Bank) UpdateAlias(id, alias string) (Caja, error) {
	if !ValidAlias(alias) {
		return Caja{}, ErrInvalid
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	c, ok := b.state.Cajas[id]
	if !ok {
		return Caja{}, ErrNotFound
	}
	if c.Status != CajaActive {
		return Caja{}, ErrCajaDisabled
	}
	if owner := b.aliasOwner(alias); owner != nil && owner.ID != id {
		return Caja{}, ErrAliasTaken
	}
	c.Alias = alias
	b.save()
	return *c, nil
}

// DisableCaja stops a CVU from receiving money (disabling twice is fine).
func (b *Bank) DisableCaja(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	c, ok := b.state.Cajas[id]
	if !ok {
		return ErrNotFound
	}
	c.Status = CajaDisabled
	b.save()
	return nil
}

// ActiveCVUs lists the CVUs that can receive money (auto payins pick from them).
func (b *Bank) ActiveCVUs() []Caja {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []Caja
	for _, c := range b.state.Cajas {
		if c.Status == CajaActive {
			out = append(out, *c)
		}
	}
	slices.SortFunc(out, func(a, b Caja) int { return strings.Compare(a.ID, b.ID) })
	return out
}

// --- destination lookup ---

// Resolution is the owner of an account number or alias.
type Resolution struct {
	AccountNumber string
	Alias         string
	Type          string // CBU or CVU
	Holder        Holder
	BankName      string
}

// ResolveAccount looks up a 22-digit CBU or CVU.
func (b *Bank) ResolveAccount(number string) (Resolution, error) {
	if !ValidAccountNumber(number) {
		return Resolution{}, ErrInvalid
	}
	if strings.HasPrefix(number[8:], notFoundAccountPrefix) {
		return Resolution{}, ErrNotFound
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, c := range b.state.Cajas {
		if c.CVU == number {
			if c.Status != CajaActive {
				return Resolution{}, ErrNotFound
			}
			return Resolution{AccountNumber: number, Alias: c.Alias, Type: "CVU", Holder: c.Holder, BankName: bankName(number)}, nil
		}
	}
	return externalResolution(number, ""), nil
}

// ResolveAlias looks up an alias.
func (b *Bank) ResolveAlias(alias string) (Resolution, error) {
	if !ValidAlias(alias) {
		return Resolution{}, ErrInvalid
	}
	if strings.HasPrefix(strings.ToLower(alias), notFoundAliasPrefix) {
		return Resolution{}, ErrNotFound
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if c := b.aliasOwner(alias); c != nil {
		if c.Status != CajaActive {
			return Resolution{}, ErrNotFound
		}
		return Resolution{AccountNumber: c.CVU, Alias: c.Alias, Type: "CVU", Holder: c.Holder, BankName: bankName(c.CVU)}, nil
	}
	return externalResolution(externalAccountForAlias(alias), strings.ToLower(alias)), nil
}

func externalResolution(number, alias string) Resolution {
	kind := "CBU"
	if strings.HasPrefix(number, "000") {
		kind = "CVU"
	}
	return Resolution{AccountNumber: number, Alias: alias, Type: kind, Holder: externalHolder(number), BankName: bankName(number)}
}

// --- master account ---

// Balance returns the master account balance in currency.
func (b *Bank) Balance(currency string) (int64, error) {
	if !slices.Contains(Currencies(), currency) {
		return 0, ErrInvalid
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.settleDue()
	return b.state.Balances[currency], nil
}

// RecordDelivery appends a webhook attempt to the state.
func (b *Bank) RecordDelivery(d Delivery) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state.Deliveries = append(b.state.Deliveries, d)
	b.save()
}

// EventFor returns the event id of the first delivery of a transfer (a redelivery reuses it,
// as a real provider does), or a new one.
func (b *Bank) EventFor(transferID string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, d := range b.state.Deliveries {
		if d.TransferID == transferID {
			return d.EventID
		}
	}
	return newID("evt_")
}
