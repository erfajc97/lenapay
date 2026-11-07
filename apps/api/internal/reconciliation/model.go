package reconciliation

import (
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// Status of a run (DB CHECK reconciliation_runs.status).
type Status string

// Run statuses.
const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// Run is one reconciliation of a currency and day.
type Run struct {
	ID                   uuid.UUID
	Currency             money.Currency
	Day                  time.Time
	Status               Status
	MatchedCount         int32
	MissingInLedgerCount int32
	MissingAtPSPCount    int32
	PSPTotal             money.Amount
	LedgerTotal          money.Amount
	Difference           money.Amount
	CreatedAt            time.Time
	FinishedAt           *time.Time
}

// RunInput requests a run.
type RunInput struct {
	Currency money.Currency
	Day      time.Time
}
