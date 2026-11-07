package kyc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// ReviewArgs is the kyc_review job: decide a submitted case automatically (vendor verdict for
// persons, development auto-approval for companies). SubmittedAt makes each submission a
// distinct unique job, so a case resubmitted after a rejection is reviewed again.
type ReviewArgs struct {
	CaseID      uuid.UUID `json:"case_id"`
	SubmittedAt time.Time `json:"submitted_at"`
	RequestID   string    `json:"request_id,omitempty"`
}

// Kind implements river.JobArgs.
func (ReviewArgs) Kind() string { return "kyc_review" }

// InsertOpts: a retried submit transaction never enqueues twice (AG13).
func (ReviewArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// ReviewWorker runs kyc_review in cmd/worker. Review is idempotent: a decided case is a no-op.
type ReviewWorker struct {
	river.WorkerDefaults[ReviewArgs]
	Service Service
}

// Timeout bounds one vendor call plus the decision transaction.
func (w *ReviewWorker) Timeout(*river.Job[ReviewArgs]) time.Duration { return 30 * time.Second }

// Work implements river.Worker.
func (w *ReviewWorker) Work(ctx context.Context, job *river.Job[ReviewArgs]) error {
	return w.Service.Review(ctx, job.Args.CaseID)
}
