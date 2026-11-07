// Package clock abstracts the current time so services and tests never call time.Now directly
// (FT7: tests pin the time with Fake).
package clock

import (
	"sync"
	"time"
)

// Clock returns the current instant.
type Clock interface {
	Now() time.Time
}

// Real is the wall clock, always in UTC.
type Real struct{}

// Now returns the current UTC time.
func (Real) Now() time.Time { return time.Now().UTC() }

// Fake is a manually driven clock for tests; safe for concurrent use.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake returns a fake clock frozen at start (converted to UTC).
func NewFake(start time.Time) *Fake {
	return &Fake{now: start.UTC()}
}

// Now returns the frozen time.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Set moves the clock to t.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t.UTC()
}

// Advance moves the clock forward by d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}
