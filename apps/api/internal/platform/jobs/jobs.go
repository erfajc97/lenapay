// Package jobs declares the one seam between services and River (A10, AG13): services enqueue
// with InsertTx inside the transaction of the effect they follow (transactional outbox), so a
// job exists if and only if the effect committed. *river.Client[pgx.Tx] implements it; tests
// pass a recording fake.
package jobs

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// Enqueuer is the part of *river.Client[pgx.Tx] a service uses.
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}
