package reconciliation

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// ReconcileArgs is the reconcile_master job of one run, inserted with the run.
type ReconcileArgs struct {
	RunID     uuid.UUID `json:"run_id" river:"unique"`
	RequestID string    `json:"request_id,omitempty"`
}

// Kind implements river.JobArgs.
func (ReconcileArgs) Kind() string { return "reconcile_master" }

// InsertOpts implements river.JobArgsWithInsertOpts.
func (ReconcileArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// ReconcileWorker runs reconcile_master in cmd/worker (idempotent: a finished run is a no-op).
type ReconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	Service Service
}

// Timeout bounds the statement download, the lookups and the write.
func (w *ReconcileWorker) Timeout(*river.Job[ReconcileArgs]) time.Duration { return 2 * time.Minute }

// Work implements river.Worker.
func (w *ReconcileWorker) Work(ctx context.Context, job *river.Job[ReconcileArgs]) error {
	return w.Service.Reconcile(ctx, job.Args.RunID)
}

// ScheduleArgs is the periodic reconcile_daily job: it queues yesterday's runs once.
type ScheduleArgs struct{}

// Kind implements river.JobArgs.
func (ScheduleArgs) Kind() string { return "reconcile_daily" }

// ScheduleWorker runs reconcile_daily.
type ScheduleWorker struct {
	river.WorkerDefaults[ScheduleArgs]
	Service Service
}

// Work implements river.Worker.
func (w *ScheduleWorker) Work(ctx context.Context, _ *river.Job[ScheduleArgs]) error {
	return w.Service.ScheduleDaily(ctx)
}

// PeriodicJobs are reconciliation's periodic jobs. Hourly is enough: ScheduleDaily creates each
// day's runs at most once, so the first tick after midnight does the work and a worker that
// was down catches up on start.
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return ScheduleArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true}),
	}
}
