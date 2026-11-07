package payins

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type processStub struct {
	Service
	event    uuid.UUID
	transfer string
}

func (s *processStub) ProcessTransfer(_ context.Context, eventID uuid.UUID, transferID string) error {
	s.event, s.transfer = eventID, transferID
	return nil
}

// The job is unique by its args (a retried webhook transaction cannot queue it twice) and hands
// the event and the transfer to the service.
func TestProcessPayinJob(t *testing.T) {
	t.Parallel()
	args := ProcessPayinArgs{EventID: uuid.New(), TransferID: "trf-1"}
	if opts := args.InsertOpts(); !opts.UniqueOpts.ByArgs || opts.MaxAttempts < 2 || args.Kind() != "payins.process" {
		t.Fatalf("opts %+v", opts)
	}
	stub := &processStub{}
	w := &ProcessPayinWorker{Service: stub}
	if err := w.Work(t.Context(), &river.Job[ProcessPayinArgs]{Args: args}); err != nil || stub.event != args.EventID || stub.transfer != "trf-1" {
		t.Fatalf("work: %v %+v", err, stub)
	}
	if w.Timeout(nil) <= 0 {
		t.Fatal("the worker needs a timeout")
	}
}
