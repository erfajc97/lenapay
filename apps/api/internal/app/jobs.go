package app

import (
	"github.com/riverqueue/river"

	"github.com/erfajc97/lenapay/apps/api/internal/erplink"
	"github.com/erfajc97/lenapay/apps/api/internal/kyc"
	"github.com/erfajc97/lenapay/apps/api/internal/notifications"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/payins"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts"
	"github.com/erfajc97/lenapay/apps/api/internal/reconciliation"
)

// RegisterWorkers adds every feature's River workers to the worker process (AG13) and returns
// how many it added (the worker idles until there is at least one). Lanes append one entry per
// worker defined in their feature's jobs.go, e.g.
//
//	func() { river.AddWorker(workers, &payouts.DispatchWorker{Service: s.Payouts}) },
func RegisterWorkers(workers *river.Workers, s *Services) int {
	_ = s
	registrations := []func(){
		func() { river.AddWorker(workers, &kyc.ReviewWorker{Service: s.KYC}) },
		func() { river.AddWorker(workers, &payouts.DispatchWorker{Service: s.Payouts}) },
		func() { river.AddWorker(workers, &payouts.SweepWorker{Service: s.Payouts}) },
		func() { river.AddWorker(workers, &reconciliation.ReconcileWorker{Service: s.Reconciliation}) },
		func() { river.AddWorker(workers, &reconciliation.ScheduleWorker{Service: s.Reconciliation}) },
		func() { river.AddWorker(workers, &erplink.ImportWorker{Service: s.ERPLink}) },
		func() { river.AddWorker(workers, &erplink.IndexRefreshWorker{Service: s.ERPLink}) },
		func() { river.AddWorker(workers, &payins.ProcessPayinWorker{Service: s.Payins}) },
		func() { river.AddWorker(workers, &payers.PersonalCVUsWorker{Service: s.Payers}) },
		func() { river.AddWorker(workers, &notifications.DeliverWorker{Service: s.Notifications}) },
		func() { river.AddWorker(workers, &notifications.SweepWorker{Service: s.Notifications}) },
	}
	for _, register := range registrations {
		register()
	}
	_ = workers
	return len(registrations)
}

// PeriodicJobs are run by the elected leader of the worker processes (the payouts sweep, the
// daily reconciliation). Lanes append theirs.
func PeriodicJobs() []*river.PeriodicJob {
	var periodic []*river.PeriodicJob
	periodic = append(periodic, payouts.PeriodicJobs()...)
	periodic = append(periodic, reconciliation.PeriodicJobs()...)
	periodic = append(periodic, erplink.PeriodicJobs()...)
	periodic = append(periodic, notifications.PeriodicJobs()...)
	return periodic
}
