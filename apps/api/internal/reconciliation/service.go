package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/ledger"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
	applog "github.com/erfajc97/lenapay/apps/api/internal/platform/log"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
	"github.com/erfajc97/lenapay/apps/api/internal/reconciliation/store"
)

// Service is the reconciliation API used by the handler and the worker.
type Service interface {
	// ListRuns lists p.OrgID's runs (newest first); ops runs are never listed to an org.
	ListRuns(ctx context.Context, p auth.Principal, currency *money.Currency, req page.Request) (page.Page[Run], error)
	// RequestRun queues an on-demand run (the internal endpoint: OrgID nil = whole master account).
	RequestRun(ctx context.Context, in RunInput) (Run, error)
	// ScheduleDaily queues yesterday's runs (ops run + one per org with activity), at most once
	// per currency, day and org: the periodic job may fire any number of times.
	ScheduleDaily(ctx context.Context) error
	// Reconcile is the reconcile_master job.
	Reconcile(ctx context.Context, runID uuid.UUID) error
}

// Deps are the collaborators of the reconciliation service.
type Deps struct {
	Pool    *pgxpool.Pool
	Clock   clock.Clock
	Jobs    jobs.Enqueuer
	PSP     psp.Provider
	Payins  Payins
	Payouts Payouts
	CVUs    CVUs
	// Location is the business time zone that defines a day (UTC when nil).
	Location *time.Location
}

type service struct {
	deps Deps
}

// NewService returns the reconciliation service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// Who asked for a run (DB CHECK reconciliation_runs.requested_by).
const (
	requestedBySchedule = "schedule"
	requestedByInternal = "internal"
)

func (s *service) loc() *time.Location {
	if s.deps.Location == nil {
		return time.UTC
	}
	return s.deps.Location
}

// dayDate normalizes a business day to the date stored in reconciliation_runs.day.
func dayDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// window is [start, end) of a business day in the business time zone.
func (s *service) window(day time.Time) (time.Time, time.Time) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, s.loc())
	return start, start.AddDate(0, 0, 1)
}

func (s *service) ListRuns(ctx context.Context, p auth.Principal, currency *money.Currency, req page.Request) (page.Page[Run], error) {
	if !p.HasOrg() {
		return page.Page[Run]{}, apperr.New(apperr.CodeOrgNotFound, nil)
	}
	if !p.Role.AtLeast(auth.RoleAdmin) {
		return page.Page[Run]{}, apperr.New(apperr.CodeForbidden, nil)
	}
	params := store.ListOrgRunsParams{OrgID: &p.OrgID, PageLimit: req.FetchLimit()}
	if currency != nil {
		c := string(*currency)
		params.Currency = &c
	}
	if req.After != nil {
		params.CursorCreatedAt, params.CursorID = &req.After.CreatedAt, &req.After.ID
	}
	rows, err := store.New(s.deps.Pool).ListOrgRuns(ctx, params)
	if err != nil {
		return page.Page[Run]{}, fmt.Errorf("reconciliation: list runs: %w", err)
	}
	runs := make([]Run, 0, len(rows))
	for _, r := range rows {
		runs = append(runs, fromRow(r))
	}
	return page.Build(runs, req.Limit, func(r Run) page.Position {
		return page.Position{CreatedAt: r.CreatedAt, ID: r.ID}
	}), nil
}

func (s *service) RequestRun(ctx context.Context, in RunInput) (Run, error) {
	if _, err := money.ParseCurrency(string(in.Currency)); err != nil {
		return Run{}, apperr.Wrap(apperr.CodeValidation, err, map[string]any{"field": "currency"})
	}
	if dayDate(in.Day).After(dayDate(s.deps.Clock.Now().In(s.loc()))) {
		return Run{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "day"})
	}
	var out Run
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		id := ids.New()
		if err := q.InsertRun(ctx, store.InsertRunParams{
			ID: id, OrgID: in.OrgID, Currency: string(in.Currency), Day: dayDate(in.Day),
			RequestedBy: requestedByInternal, CreatedAt: s.deps.Clock.Now(),
		}); err != nil {
			return fmt.Errorf("reconciliation: insert run: %w", err)
		}
		if err := s.enqueue(ctx, tx, id); err != nil {
			return err
		}
		row, err := q.GetRun(ctx, id)
		if err != nil {
			return fmt.Errorf("reconciliation: read run: %w", err)
		}
		out = fromRow(row)
		return nil
	})
	return out, err
}

func (s *service) ScheduleDaily(ctx context.Context) error {
	yesterday := s.deps.Clock.Now().In(s.loc()).AddDate(0, 0, -1)
	for _, c := range ledger.Currencies() {
		orgs, err := s.activeOrgs(ctx, c, yesterday)
		if err != nil {
			return err
		}
		if err := s.scheduleRuns(ctx, c, yesterday, orgs); err != nil {
			return err
		}
	}
	return nil
}

// activeOrgs are the orgs with ledger movements on day (an org with only PSP lines and no
// ledger record shows up in the ops run, where its lines are missing_in_ledger).
func (s *service) activeOrgs(ctx context.Context, c money.Currency, day time.Time) ([]uuid.UUID, error) {
	movements, err := s.ledgerMovements(ctx, c, day)
	if err != nil {
		return nil, err
	}
	seen := map[uuid.UUID]bool{}
	var out []uuid.UUID
	for _, m := range movements {
		if !seen[m.OrgID] {
			seen[m.OrgID] = true
			out = append(out, m.OrgID)
		}
	}
	return out, nil
}

func (s *service) scheduleRuns(ctx context.Context, c money.Currency, day time.Time, orgs []uuid.UUID) error {
	scopes := make([]*uuid.UUID, 0, len(orgs)+1)
	scopes = append(scopes, nil) // the ops run over the whole master account
	for i := range orgs {
		scopes = append(scopes, &orgs[i])
	}
	return db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		for _, org := range scopes {
			id := ids.New()
			rows, err := q.InsertScheduledRun(ctx, store.InsertScheduledRunParams{
				ID: id, OrgID: org, Currency: string(c), Day: dayDate(day), CreatedAt: s.deps.Clock.Now(),
			})
			if err != nil {
				return fmt.Errorf("reconciliation: schedule run: %w", err)
			}
			if rows == 0 {
				continue // already scheduled for this currency, day and org
			}
			if err := s.enqueue(ctx, tx, id); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *service) enqueue(ctx context.Context, tx pgx.Tx, runID uuid.UUID) error {
	if _, err := s.deps.Jobs.InsertTx(ctx, tx, ReconcileArgs{RunID: runID, RequestID: applog.RequestID(ctx)}, nil); err != nil {
		return fmt.Errorf("reconciliation: enqueue: %w", err)
	}
	return nil
}

func (s *service) Reconcile(ctx context.Context, runID uuid.UUID) error {
	run, err := store.New(s.deps.Pool).GetRun(ctx, runID)
	if db.IsNoRows(err) {
		return apperr.New(apperr.CodeReconciliationRunNotFound, map[string]any{"runId": runID.String()})
	}
	if err != nil {
		return fmt.Errorf("reconciliation: get run: %w", err)
	}
	if Status(run.Status) != StatusQueued {
		return nil // already reconciled (River delivers at least once)
	}
	currency := money.Currency(run.Currency)
	items, err := s.compute(ctx, currency, run.Day)
	if err != nil {
		return err
	}
	items = forOrg(items, run.OrgID)
	totals, err := summarize(currency, items)
	if err != nil {
		return err
	}
	return db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return s.store(ctx, store.New(tx), runID, items, totals)
	})
}

// store writes the outcome of a run in ONE transaction; a concurrent copy of the job finds the
// run no longer queued and writes nothing.
func (s *service) store(ctx context.Context, q *store.Queries, runID uuid.UUID, items []Item, t Totals) error {
	now := s.deps.Clock.Now()
	started, err := q.StartRun(ctx, store.StartRunParams{StartedAt: &now, ID: runID})
	if err != nil {
		return fmt.Errorf("reconciliation: start run: %w", err)
	}
	if started != 1 {
		return nil
	}
	for _, it := range items {
		params := store.InsertItemParams{
			ID: ids.New(), RunID: runID, OrgID: it.OrgID, ProviderTransferID: it.ProviderTransferID,
			Direction: string(it.Direction), Currency: string(it.Currency), Status: string(it.Status), CreatedAt: now,
		}
		if it.PSPAmount != nil {
			params.PspAmountMinor = &it.PSPAmount.Minor
		}
		if it.LedgerAmount != nil {
			params.LedgerAmountMinor = &it.LedgerAmount.Minor
		}
		if err := q.InsertItem(ctx, params); err != nil {
			return fmt.Errorf("reconciliation: insert item: %w", err)
		}
	}
	return q.FinishRun(ctx, store.FinishRunParams{
		MatchedCount: t.Matched, MissingInLedgerCount: t.MissingInLedger, MissingAtPspCount: t.MissingAtPSP,
		PspTotalMinor: t.PSP.Minor, LedgerTotalMinor: t.Ledger.Minor, DifferenceMinor: t.Difference.Minor,
		FinishedAt: &now, ID: runID,
	})
}

// compute matches the master statement of the day against every ledger movement, then
// attributes the incoming lines we have no record of to the org that owns the receiving CVU.
func (s *service) compute(ctx context.Context, c money.Currency, day time.Time) ([]Item, error) {
	start, _ := s.window(day)
	lines, err := s.deps.PSP.Statement(ctx, c, start)
	if err != nil {
		return nil, fmt.Errorf("reconciliation: statement: %w", err)
	}
	movements, err := s.ledgerMovements(ctx, c, day)
	if err != nil {
		return nil, err
	}
	items := match(c, lines, movements)
	for i := range items {
		if items[i].Status == ItemMissingInLedger && items[i].Direction == psp.DirectionIn {
			org, err := s.ownerOf(ctx, items[i].ProviderTransferID)
			if err != nil {
				return nil, err
			}
			items[i].OrgID = org
		}
	}
	return items, nil
}

// ownerOf finds the org of an incoming transfer by its receiving CVU (nil when the PSP or the
// CVU registry does not know it: the line stays in the ops run only).
func (s *service) ownerOf(ctx context.Context, transferID string) (*uuid.UUID, error) {
	tr, err := s.deps.PSP.GetTransfer(ctx, transferID)
	if errors.Is(err, psp.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reconciliation: transfer %s: %w", transferID, err)
	}
	cvu, err := s.deps.CVUs.CVUByNumber(ctx, tr.CVU)
	if apperr.IsCode(err, apperr.CodeCVUNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reconciliation: cvu owner: %w", err)
	}
	return &cvu.OrgID, nil
}

// ledgerMovements are the credited payins (gross: what crossed the master account) and the
// settled payouts (amount: the fee never leaves Lenapay) of the business day.
func (s *service) ledgerMovements(ctx context.Context, c money.Currency, day time.Time) ([]LedgerMovement, error) {
	from, to := s.window(day)
	ins, err := s.deps.Payins.CreditedBetween(ctx, c, from, to)
	if err != nil {
		return nil, fmt.Errorf("reconciliation: payins: %w", err)
	}
	outs, err := s.deps.Payouts.SettledBetween(ctx, c, from, to)
	if err != nil {
		return nil, fmt.Errorf("reconciliation: payouts: %w", err)
	}
	movements := make([]LedgerMovement, 0, len(ins)+len(outs))
	for _, p := range ins {
		movements = append(movements, LedgerMovement{Keys: []string{p.ProviderTransferID}, Direction: psp.DirectionIn, Amount: p.Gross, OrgID: p.OrgID})
	}
	for _, p := range outs {
		movements = append(movements, LedgerMovement{Keys: []string{deref(p.ProviderTransferID), deref(p.ProviderAttemptID)}, Direction: psp.DirectionOut, Amount: p.Amount, OrgID: p.OrgID})
	}
	return movements, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func fromRow(r store.ReconciliationRun) Run {
	c := money.Currency(r.Currency)
	return Run{
		ID: r.ID, OrgID: r.OrgID, Currency: c, Day: r.Day, Status: Status(r.Status),
		MatchedCount: r.MatchedCount, MissingInLedgerCount: r.MissingInLedgerCount, MissingAtPSPCount: r.MissingAtPspCount,
		PSPTotal: money.New(r.PspTotalMinor, c), LedgerTotal: money.New(r.LedgerTotalMinor, c),
		Difference: money.New(r.DifferenceMinor, c), CreatedAt: r.CreatedAt, FinishedAt: r.FinishedAt,
	}
}
