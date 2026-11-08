package bank

import (
	"errors"
	"slices"
	"strings"
	"time"
)

// Outcome scripts what the next outgoing transfer does.
type Outcome string

// Outcomes (see the package doc for the magic amounts that pick them).
const (
	OutcomeOK          Outcome = "ok"
	OutcomeRejected    Outcome = "rejected"
	OutcomePending     Outcome = "pending"
	OutcomeTimeout     Outcome = "timeout"
	OutcomeError       Outcome = "error"
	OutcomeUnavailable Outcome = "unavailable"
)

// ValidOutcome reports whether o is a known outcome.
func ValidOutcome(o Outcome) bool {
	return slices.Contains([]Outcome{OutcomeOK, OutcomeRejected, OutcomePending, OutcomeTimeout, OutcomeError, OutcomeUnavailable}, o)
}

// Failure codes of a FAILED transfer (BIND "codigo").
const (
	FailureRejected     = "RECHAZADA"
	FailureNoFunds      = "SALDO_INSUFICIENTE"
	FailureNoSuchTarget = "CUENTA_INEXISTENTE"
)

// SetNextOutcome forces the outcome of the next outgoing transfers, in order.
func (b *Bank) SetNextOutcome(o Outcome) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextOutcome = append(b.nextOutcome, o)
}

func outcomeForAmount(minor int64) Outcome {
	switch minor % 100 {
	case 13:
		return OutcomeRejected
	case 96:
		return OutcomeUnavailable
	case 97:
		return OutcomePending
	case 98:
		return OutcomeTimeout
	case 99:
		return OutcomeError
	}
	return OutcomeOK
}

// SendInput is an outgoing transfer request (BIND TRANSFER transaction-request).
type SendInput struct {
	OriginID  string
	ToAccount string // CBU/CVU, or
	ToAlias   string // alias
	// OriginCVU is BIND's origin_debit: the client CVU of ours the money leaves from. Optional;
	// it names the origin of an on-us credit.
	OriginCVU   string
	Currency    string
	Amount      int64
	Concept     string
	Description string
}

// SendResult is the stored transfer and how the HTTP answer must behave.
type SendResult struct {
	Transfer Transfer
	// Outcome tells the HTTP layer to answer normally (ok, rejected, pending), hang
	// (timeout), answer 500 (error) or 503 (unavailable: nothing was stored).
	Outcome Outcome
}

// SendTransfer debits the master account toward a destination. BIND dedupes by origin_id: a
// second request with the same origin_id returns the first transfer and moves no money.
func (b *Bank) SendTransfer(in SendInput) (SendResult, error) {
	if in.OriginID == "" || len(in.OriginID) > 100 || in.Amount <= 0 || !slices.Contains(Currencies(), in.Currency) ||
		(in.ToAccount == "") == (in.ToAlias == "") || len(in.Concept) > 100 || len(in.Description) > 200 ||
		(in.OriginCVU != "" && !ValidAccountNumber(in.OriginCVU)) {
		return SendResult{}, ErrInvalid
	}
	var target Resolution
	var resolveErr error
	if in.ToAccount != "" {
		target, resolveErr = b.ResolveAccount(in.ToAccount)
	} else {
		target, resolveErr = b.ResolveAlias(in.ToAlias)
	}
	if resolveErr != nil && !errors.Is(resolveErr, ErrNotFound) {
		return SendResult{}, resolveErr // malformed destination: a 400, nothing stored
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if existing := b.findTransfer(in.OriginID); existing != nil && existing.Type == TypeDebit {
		return SendResult{Transfer: *existing, Outcome: OutcomeOK}, nil
	}
	outcome := outcomeForAmount(in.Amount)
	if len(b.nextOutcome) > 0 {
		outcome, b.nextOutcome = b.nextOutcome[0], b.nextOutcome[1:]
	}
	if outcome == OutcomeUnavailable {
		return SendResult{Outcome: outcome}, nil // the bank never saw it: a retry executes it
	}
	now := b.opts.Now()
	t := &Transfer{
		ID: newID("trf-"), OriginID: in.OriginID, Type: TypeDebit, Status: StatusInProgress, Currency: in.Currency,
		Amount: Decimal(in.Amount), Concept: in.Concept, Description: in.Description, CreatedAt: now,
		Counterparty: Counterparty{Name: target.Holder.Name, CUIT: target.Holder.CUIT, AccountNumber: target.AccountNumber, Alias: in.ToAlias},
	}
	onUs := resolveErr == nil && b.cajaByCVU(target.AccountNumber) != nil
	if onUs {
		t.OnUsCVU, t.Origin = target.AccountNumber, b.originOf(in.OriginCVU)
	}
	b.state.Transfers = append(b.state.Transfers, t)
	switch {
	case resolveErr != nil:
		b.fail(t, FailureNoSuchTarget)
	case onUs && b.cajaByCVU(target.AccountNumber).Currency != in.Currency, outcome == OutcomeRejected:
		b.fail(t, FailureRejected)
	case outcome != OutcomePending:
		b.complete(t)
	}
	b.save()
	return SendResult{Transfer: *t, Outcome: outcome}, nil
}

// complete settles an IN_PROGRESS debit (FAILED when the master account cannot cover it, or
// when its on-us CVU stopped receiving meanwhile). An on-us debit also credits its CVU.
func (b *Bank) complete(t *Transfer) {
	var target *Caja
	if t.OnUsCVU != "" {
		if target = b.cajaByCVU(t.OnUsCVU); target == nil || target.Status != CajaActive {
			b.fail(t, FailureNoSuchTarget)
			return
		}
	}
	if b.state.Balances[t.Currency] < int64(t.Amount) {
		b.fail(t, FailureNoFunds)
		return
	}
	now := b.opts.Now()
	b.state.Balances[t.Currency] -= int64(t.Amount)
	t.Status, t.CompletedAt = StatusCompleted, &now
	if target != nil {
		b.creditOnUs(t, target, now)
	}
}

// creditOnUs books the incoming side of a settled on-us debit and tells the credit listener.
// Called with b.mu held.
func (b *Bank) creditOnUs(debit *Transfer, target *Caja, now time.Time) {
	credit := &Transfer{
		ID: newID("trf-"), Type: TypeCredit, Status: StatusCompleted, Currency: debit.Currency, Amount: debit.Amount,
		CVU: target.CVU, Counterparty: *debit.Origin, Concept: withDefault(debit.Description, debit.Concept), RelatedID: debit.ID,
		CreatedAt: now, CompletedAt: &now,
	}
	debit.RelatedID = credit.ID
	b.state.Balances[credit.Currency] += int64(credit.Amount)
	b.state.Transfers = append(b.state.Transfers, credit)
	if b.onCredit != nil {
		b.onCredit(*credit)
	}
}

// originOf is the counterparty an on-us credit shows: the holder of the debited CVU when it is
// one of ours, else the master account holder. Called with b.mu held.
func (b *Bank) originOf(originCVU string) *Counterparty {
	if c := b.cajaByCVU(originCVU); originCVU != "" && c != nil {
		return &Counterparty{Name: c.Holder.Name, CUIT: c.Holder.CUIT, AccountNumber: c.CVU, Alias: c.Alias}
	}
	h := MasterHolder()
	return &Counterparty{Name: h.Name, CUIT: h.CUIT, AccountNumber: MasterAccountNumber()}
}

func (b *Bank) fail(t *Transfer, code string) {
	now := b.opts.Now()
	t.Status, t.FailureCode, t.CompletedAt = StatusFailed, code, &now
}

// settleDue completes the IN_PROGRESS debits older than PendingDelay. Called with b.mu held.
func (b *Bank) settleDue() {
	changed := false
	for _, t := range b.state.Transfers {
		if t.Status == StatusInProgress && !b.opts.Now().Before(t.CreatedAt.Add(b.opts.PendingDelay)) {
			b.complete(t)
			changed = true
		}
	}
	if changed {
		b.save()
	}
}

func (b *Bank) findTransfer(idOrOrigin string) *Transfer {
	for _, t := range b.state.Transfers {
		if t.ID == idOrOrigin || (t.OriginID != "" && t.OriginID == idOrOrigin) {
			return t
		}
	}
	return nil
}

// GetTransfer returns a transfer by BIND id or by our origin_id (as BIND's read accepts both).
func (b *Bank) GetTransfer(idOrOrigin string) (Transfer, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.settleDue()
	t := b.findTransfer(idOrOrigin)
	if t == nil {
		return Transfer{}, ErrNotFound
	}
	return *t, nil
}

// CompleteTransfer forces the final status of an IN_PROGRESS debit (demo control).
func (b *Bank) CompleteTransfer(id, status string) (Transfer, error) {
	if status != StatusCompleted && status != StatusFailed {
		return Transfer{}, ErrInvalid
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.findTransfer(id)
	if t == nil || t.Type != TypeDebit {
		return Transfer{}, ErrNotFound
	}
	if t.Status != StatusInProgress {
		return Transfer{}, ErrInvalid
	}
	if status == StatusCompleted {
		b.complete(t)
	} else {
		b.fail(t, FailureRejected)
	}
	b.save()
	return *t, nil
}

// TransferFilter selects transfers of the listing endpoint.
type TransferFilter struct {
	Type       string // CREDIT, DEBIT or "" for both
	Currency   string
	CVU        string
	TransferID string
	// From and To are local days (inclusive) in the bank's zone, by completion date; zero = open.
	From, To time.Time
	// Start is 1-based; Length caps the page.
	Start, Length int
}

// ListTransfers returns one page of matching transfers and the total count. A day filter
// lists only COMPLETED movements: that is the statement of the master account.
func (b *Bank) ListTransfers(f TransferFilter) ([]Transfer, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.settleDue()
	var all []Transfer
	for _, t := range b.state.Transfers {
		if b.matches(t, f) {
			all = append(all, *t)
		}
	}
	start := max(f.Start, 1) - 1
	if start >= len(all) {
		return []Transfer{}, len(all)
	}
	end := min(start+max(f.Length, 1), len(all))
	return all[start:end], len(all)
}

func (b *Bank) matches(t *Transfer, f TransferFilter) bool {
	if (f.Type != "" && t.Type != f.Type) || (f.Currency != "" && t.Currency != f.Currency) ||
		(f.CVU != "" && t.CVU != f.CVU) || (f.TransferID != "" && t.ID != f.TransferID) {
		return false
	}
	if f.From.IsZero() && f.To.IsZero() {
		return true
	}
	if t.Status != StatusCompleted || t.CompletedAt == nil {
		return false
	}
	day := t.CompletedAt.In(b.opts.Location)
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	return (f.From.IsZero() || !day.Before(f.From)) && (f.To.IsZero() || !day.After(f.To))
}

// IncomingInput simulates a payer transferring to one of our CVUs (or aliases).
type IncomingInput struct {
	CVU      string
	Alias    string
	Currency string // "" = the caja's currency
	Amount   int64
	Origin   Counterparty
}

// Incoming credits the master account for a transfer to an active CVU. A disabled CVU bounces
// the money (ErrCajaDisabled), as the network does.
func (b *Bank) Incoming(in IncomingInput) (Transfer, error) {
	if in.Amount <= 0 || (in.CVU == "") == (in.Alias == "") {
		return Transfer{}, ErrInvalid
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var caja *Caja
	for _, c := range b.state.Cajas {
		if (in.CVU != "" && c.CVU == in.CVU) || (in.Alias != "" && strings.EqualFold(c.Alias, in.Alias)) {
			caja = c
		}
	}
	switch {
	case caja == nil:
		return Transfer{}, ErrNotFound
	case caja.Status != CajaActive:
		return Transfer{}, ErrCajaDisabled
	case in.Currency != "" && in.Currency != caja.Currency:
		return Transfer{}, ErrInvalid
	}
	origin := in.Origin
	if origin.AccountNumber == "" {
		origin.AccountNumber = AccountNumberFor(origin.CUIT)
	}
	if origin.Name == "" || origin.CUIT == "" {
		h := externalHolder(origin.AccountNumber)
		origin.Name, origin.CUIT = withDefault(origin.Name, h.Name), withDefault(origin.CUIT, h.CUIT)
	}
	now := b.opts.Now()
	t := &Transfer{
		ID: newID("trf-"), Type: TypeCredit, Status: StatusCompleted, Currency: caja.Currency, Amount: Decimal(in.Amount),
		CVU: caja.CVU, Counterparty: origin, CreatedAt: now, CompletedAt: &now,
	}
	b.state.Balances[t.Currency] += in.Amount
	b.state.Transfers = append(b.state.Transfers, t)
	b.save()
	return *t, nil
}

func withDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
