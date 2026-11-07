package payins

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// ProcessPayinArgs credits the transfer a stored webhook event announced. Inserted in the
// webhook's transaction; the transfer is re-read at the PSP when the job runs (MO4).
type ProcessPayinArgs struct {
	EventID    uuid.UUID `json:"event_id"`
	TransferID string    `json:"transfer_id"`
}

// Kind implements river.JobArgs.
func (ProcessPayinArgs) Kind() string { return "payins.process" }

// InsertOpts: unique by args; enough attempts to outlast a PSP outage with River's backoff.
func (ProcessPayinArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 25, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// ProcessPayinWorker runs ProcessTransfer (idempotent: a credited transfer is credited once).
type ProcessPayinWorker struct {
	river.WorkerDefaults[ProcessPayinArgs]
	Service Service
}

// Timeout bounds the PSP read plus the credit transaction.
func (*ProcessPayinWorker) Timeout(*river.Job[ProcessPayinArgs]) time.Duration { return time.Minute }

// Work implements river.Worker.
func (w *ProcessPayinWorker) Work(ctx context.Context, job *river.Job[ProcessPayinArgs]) error {
	return w.Service.ProcessTransfer(ctx, job.Args.EventID, job.Args.TransferID)
}
