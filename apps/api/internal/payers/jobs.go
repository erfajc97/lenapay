package payers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// PersonalCVUsArgs asks for the personal CVUs of an approved personal org (R-CVU-3). It is
// inserted in the KYC approval transaction; the tenant travels in the payload (T5).
type PersonalCVUsArgs struct {
	OrgID uuid.UUID `json:"org_id"`
}

// Kind implements river.JobArgs.
func (PersonalCVUsArgs) Kind() string { return "payers.create_personal_cvus" }

// InsertOpts: one job per org even if the approval transaction is retried.
func (PersonalCVUsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 25, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// PersonalCVUsWorker runs CreatePersonalCVUs (idempotent: River delivers at least once).
type PersonalCVUsWorker struct {
	river.WorkerDefaults[PersonalCVUsArgs]
	Service Service
}

// Timeout bounds two PSP calls and their writes.
func (*PersonalCVUsWorker) Timeout(*river.Job[PersonalCVUsArgs]) time.Duration { return time.Minute }

// Work implements river.Worker.
func (w *PersonalCVUsWorker) Work(ctx context.Context, job *river.Job[PersonalCVUsArgs]) error {
	return w.Service.CreatePersonalCVUs(ctx, job.Args.OrgID)
}
