package app

import "github.com/riverqueue/river"

// RegisterWorkers adds every feature's River workers to the worker process (AG13) and returns
// how many it added (the worker idles until there is at least one). Lanes append one entry per
// worker defined in their feature's jobs.go, e.g.
//
//	func() { river.AddWorker(workers, &payouts.DispatchWorker{Service: s.Payouts}) },
func RegisterWorkers(workers *river.Workers, s *Services) int {
	_ = s
	registrations := []func(){}
	for _, register := range registrations {
		register()
	}
	_ = workers
	return len(registrations)
}

// PeriodicJobs are run by the elected leader of the worker processes (the payouts sweep, the
// daily reconciliation). Lanes append theirs.
func PeriodicJobs() []*river.PeriodicJob {
	return nil
}
