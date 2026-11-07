package payouts

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// DispatchArgs is the dispatch_payout job, inserted in the approval transaction (personal
// payouts: in the creation transaction). Unique by payout, so a retried transaction or a
// replayed approval never queues it twice.
type DispatchArgs struct {
	PayoutID  uuid.UUID `json:"payout_id" river:"unique"`
	RequestID string    `json:"request_id,omitempty"`
}

// Kind implements river.JobArgs.
func (DispatchArgs) Kind() string { return "dispatch_payout" }

// InsertOpts implements river.JobArgsWithInsertOpts.
func (DispatchArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// DispatchWorker runs dispatch_payout in cmd/worker. Dispatch is idempotent: a payout that is
// no longer approved is only re-read at the PSP by its attempt reference, never re-sent.
type DispatchWorker struct {
	river.WorkerDefaults[DispatchArgs]
	Service Service
}

// Timeout bounds one PSP call plus the settlement transaction.
func (w *DispatchWorker) Timeout(*river.Job[DispatchArgs]) time.Duration { return 30 * time.Second }

// Work implements river.Worker.
func (w *DispatchWorker) Work(ctx context.Context, job *river.Job[DispatchArgs]) error {
	return w.Service.Dispatch(ctx, job.Args.PayoutID)
}

// SweepArgs is the periodic sweep_payouts job (R-OUT-5).
type SweepArgs struct{}

// Kind implements river.JobArgs.
func (SweepArgs) Kind() string { return "sweep_payouts" }

// SweepWorker re-reads the `sending` payouts at the PSP.
type SweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	Service Service
}

// Timeout bounds one batch of re-reads.
func (w *SweepWorker) Timeout(*river.Job[SweepArgs]) time.Duration { return 2 * time.Minute }

// Work implements river.Worker.
func (w *SweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	return w.Service.Sweep(ctx)
}

// PeriodicJobs are the payouts' periodic jobs (run on the elected leader only).
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(time.Minute),
			func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true}),
	}
}
