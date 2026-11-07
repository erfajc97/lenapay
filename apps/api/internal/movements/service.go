package movements

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/movements/store"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

// Service is the movements read API.
type Service interface {
	// List returns p.OrgID's movements, newest first.
	List(ctx context.Context, p auth.Principal, f Filter) (page.Page[Movement], error)
	// Get returns one of p.OrgID's movements (movement.not_found when missing or foreign).
	Get(ctx context.Context, p auth.Principal, id uuid.UUID) (Movement, error)
}

// Deps are the collaborators of the movements service.
type Deps struct {
	Pool *pgxpool.Pool
	// Location is the business time zone of Filter.From/To (A6).
	Location *time.Location
}

type service struct {
	deps Deps
}

// NewService returns the movements service.
func NewService(deps Deps) Service { return &service{deps: deps} }

func (s *service) List(ctx context.Context, p auth.Principal, f Filter) (page.Page[Movement], error) {
	from, to, err := s.window(f.From, f.To)
	if err != nil {
		return page.Page[Movement]{}, err
	}
	params := store.ListMovementsParams{
		OrgID: &p.OrgID, AccountID: f.AccountID, FromTime: from, ToTime: to,
		Type: (*string)(f.Type), Status: (*string)(f.Status), Currency: (*string)(f.Currency),
		PageLimit: f.Page.FetchLimit(),
	}
	if f.Page.After != nil {
		params.CursorCreatedAt, params.CursorID = &f.Page.After.CreatedAt, &f.Page.After.ID
	}
	rows, err := store.New(s.deps.Pool).ListMovements(ctx, params)
	if err != nil {
		return page.Page[Movement]{}, fmt.Errorf("movements: list: %w", err)
	}
	items := make([]Movement, 0, len(rows))
	for _, r := range rows {
		items = append(items, toMovement(r))
	}
	return page.Build(items, f.Page.Limit, func(m Movement) page.Position {
		return page.Position{CreatedAt: m.CreatedAt, ID: m.ID}
	}), nil
}

func (s *service) Get(ctx context.Context, p auth.Principal, id uuid.UUID) (Movement, error) {
	row, err := store.New(s.deps.Pool).GetMovement(ctx, store.GetMovementParams{ID: id, OrgID: &p.OrgID})
	if db.IsNoRows(err) {
		return Movement{}, apperr.New(apperr.CodeMovementNotFound, map[string]any{"movementId": id.String()})
	}
	if err != nil {
		return Movement{}, fmt.Errorf("movements: get: %w", err)
	}
	return toMovement(store.ListMovementsRow(row)), nil
}

// window turns inclusive business dates into the half-open instant range
// [start of from, start of the day after to) in the business time zone.
func (s *service) window(from, to *time.Time) (*time.Time, *time.Time, error) {
	loc := s.deps.Location
	if loc == nil {
		loc = time.UTC
	}
	startOf := func(d time.Time) time.Time {
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	}
	var start, end *time.Time
	if from != nil {
		v := startOf(*from)
		start = &v
	}
	if to != nil {
		v := startOf(*to).AddDate(0, 0, 1)
		end = &v
	}
	if start != nil && end != nil && !start.Before(*end) {
		return nil, nil, apperr.New(apperr.CodeValidation, map[string]any{"param": "from"})
	}
	return start, end, nil
}

func toMovement(r store.ListMovementsRow) Movement {
	currency := money.Currency(r.Currency)
	m := Movement{
		ID: r.ID, Type: Type(r.Type), Status: Status(r.Status), Direction: Direction(r.Direction),
		AccountID: r.AccountID, Amount: money.New(r.AmountMinor, currency), Concept: r.Concept,
		RefID: r.RefID, CreatedAt: r.CreatedAt,
		Counterparty: Counterparty{CUIT: r.CounterpartyCuit, CBU: r.CounterpartyCbu},
	}
	if r.FeeMinor != nil {
		fee := money.New(*r.FeeMinor, currency)
		m.Fee = &fee
	}
	if r.CounterpartyName != "" {
		m.Counterparty.Name = &r.CounterpartyName
	}
	return m
}
