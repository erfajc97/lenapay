package payers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type personalStub struct {
	Service
	org uuid.UUID
}

func (s *personalStub) CreatePersonalCVUs(_ context.Context, orgID uuid.UUID) error {
	s.org = orgID
	return nil
}

func TestPersonalCVUsJob(t *testing.T) {
	t.Parallel()
	args := PersonalCVUsArgs{OrgID: uuid.New()}
	if opts := args.InsertOpts(); !opts.UniqueOpts.ByArgs || args.Kind() != "payers.create_personal_cvus" {
		t.Fatalf("opts %+v", opts)
	}
	stub := &personalStub{}
	w := &PersonalCVUsWorker{Service: stub}
	if err := w.Work(t.Context(), &river.Job[PersonalCVUsArgs]{Args: args}); err != nil || stub.org != args.OrgID || w.Timeout(nil) <= 0 {
		t.Fatalf("work: %v %+v", err, stub)
	}
}
