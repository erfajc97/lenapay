package notifications

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type jobsStub struct {
	Service
	delivered uuid.UUID
	age       time.Duration
}

func (s *jobsStub) Deliver(_ context.Context, id uuid.UUID) error {
	s.delivered = id
	return nil
}

func (s *jobsStub) RequeueStale(_ context.Context, age time.Duration) (int, error) {
	s.age = age
	return 1, nil
}

func TestNotificationJobs(t *testing.T) {
	t.Parallel()
	args := DeliverArgs{NotificationID: uuid.New()}
	if opts := args.InsertOpts(); !opts.UniqueOpts.ByArgs || opts.MaxAttempts != maxAttempts || args.Kind() != "notifications.deliver" {
		t.Fatalf("opts %+v", opts)
	}
	stub := &jobsStub{}
	deliver := &DeliverWorker{Service: stub}
	if err := deliver.Work(t.Context(), &river.Job[DeliverArgs]{Args: args}); err != nil || stub.delivered != args.NotificationID || deliver.Timeout(nil) <= 0 {
		t.Fatalf("deliver: %v", err)
	}
	if err := (&SweepWorker{Service: stub}).Work(t.Context(), &river.Job[SweepArgs]{}); err != nil || stub.age != staleAfter {
		t.Fatalf("sweep: %v %s", err, stub.age)
	}
	if len(PeriodicJobs()) != 1 || (SweepArgs{}).Kind() != "notifications.sweep" {
		t.Fatal("the sweep is periodic")
	}
}
