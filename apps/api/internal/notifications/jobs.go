package notifications

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// DeliverArgs pushes one outbox entry; inserted by Notify in the fact's transaction.
type DeliverArgs struct {
	NotificationID uuid.UUID `json:"notification_id"`
}

// Kind implements river.JobArgs.
func (DeliverArgs) Kind() string { return "notifications.deliver" }

// InsertOpts: one job per entry, retried with River's backoff up to the outbox's attempts.
func (DeliverArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: maxAttempts, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// DeliverWorker runs Deliver.
type DeliverWorker struct {
	river.WorkerDefaults[DeliverArgs]
	Service Service
}

// Timeout bounds one push.
func (*DeliverWorker) Timeout(*river.Job[DeliverArgs]) time.Duration { return 30 * time.Second }

// Work implements river.Worker.
func (w *DeliverWorker) Work(ctx context.Context, job *river.Job[DeliverArgs]) error {
	return w.Service.Deliver(ctx, job.Args.NotificationID)
}

// SweepArgs is the periodic re-queue of entries whose delivery job was lost.
type SweepArgs struct{}

// Kind implements river.JobArgs.
func (SweepArgs) Kind() string { return "notifications.sweep" }

// SweepWorker runs RequeueStale.
type SweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	Service Service
}

// staleAfter leaves a fresh entry to its own job (River retries take a while).
const staleAfter = 15 * time.Minute

// Work implements river.Worker.
func (w *SweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	n, err := w.Service.RequeueStale(ctx, staleAfter)
	if n > 0 {
		slog.InfoContext(ctx, "notifications requeued", "count", n)
	}
	return err
}

// PeriodicJobs is the sweep, run by the elected leader every 5 minutes.
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(5*time.Minute),
			func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil }, nil),
	}
}
