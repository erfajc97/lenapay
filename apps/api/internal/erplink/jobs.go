package erplink

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// ImportArgs is the erp_import job, inserted with the link.
type ImportArgs struct {
	OrgID     uuid.UUID `json:"org_id" river:"unique"`
	RequestID string    `json:"request_id,omitempty"`
}

// Kind implements river.JobArgs.
func (ImportArgs) Kind() string { return "erp_import" }

// InsertOpts implements river.JobArgsWithInsertOpts.
func (ImportArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// ImportWorker runs erp_import in cmd/worker (idempotent: the importers dedupe by ERP ref).
type ImportWorker struct {
	river.WorkerDefaults[ImportArgs]
	Service Service
}

// Timeout bounds one snapshot copy.
func (w *ImportWorker) Timeout(*river.Job[ImportArgs]) time.Duration { return 2 * time.Minute }

// Work implements river.Worker.
func (w *ImportWorker) Work(ctx context.Context, job *river.Job[ImportArgs]) error {
	return w.Service.Import(ctx, job.Args.OrgID)
}

// IndexRefreshArgs is the periodic erp_index_refresh job: reload erp_identity_index from the
// Directory (the fixtures now; the ERP dump or CDC read model behind the same port later).
type IndexRefreshArgs struct{}

// Kind implements river.JobArgs.
func (IndexRefreshArgs) Kind() string { return "erp_index_refresh" }

// IndexRefreshWorker runs erp_index_refresh.
type IndexRefreshWorker struct {
	river.WorkerDefaults[IndexRefreshArgs]
	Service Service
}

// Work implements river.Worker.
func (w *IndexRefreshWorker) Work(ctx context.Context, _ *river.Job[IndexRefreshArgs]) error {
	return w.Service.LoadIdentityIndex(ctx)
}

// PeriodicJobs are the ERP link's periodic jobs: the index loads when the worker starts and
// refreshes every 6 hours (upserts, so a run is idempotent).
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(6*time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return IndexRefreshArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true}),
	}
}
