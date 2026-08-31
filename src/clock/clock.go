// Package clock is a thin, mockable layer over the standard library time
// package.
//
// Call clock.Now() anywhere a module needs the current time instead of
// time.Now(). In production the package returns real (system) time. In tests,
// call clock.SetSource(...) — typically once in setup — to swap the active
// source. Every module that depends on clock.Now() then observes the mocked
// time, so time-dependent logic becomes deterministically testable without
// changing call sites beyond the package name.
//
// Now returns a plain time.Time. Every standard-library method on that value
// (.After, .Before, .Add, .Sub, .In, formatting) works unchanged, because the
// package only replaces where "now" comes from — never the time type itself.
package clock

import (
	"sync"
	"time"
)

// Source produces the current time. Real is the default; tests swap it.
type Source interface {
	Now() time.Time
}

type realSource struct{}

func (realSource) Now() time.Time { return time.Now() }

var (
	mu     sync.RWMutex
	source Source = realSource{}
)

// Now returns the current time from the active source.
func Now() time.Time {
	mu.RLock()
	s := source
	mu.RUnlock()
	return s.Now()
}

// Since returns the duration elapsed since t, measured against the active
// source (clock.Now().Sub(t)).
func Since(t time.Time) time.Duration {
	return Now().Sub(t)
}

// Until returns the duration from now until t, measured against the active
// source (t.Sub(clock.Now())). Negative when t is in the past.
func Until(t time.Time) time.Duration {
	return t.Sub(Now())
}

// SetSource swaps the active time source. Intended for test setup; call Reset
// (or SetSource(clock.Real())) afterwards to restore real time.
func SetSource(s Source) {
	mu.Lock()
	source = s
	mu.Unlock()
}

// Reset restores the real (system) time source.
func Reset() {
	SetSource(realSource{})
}

// Real returns the production time source (the system clock).
func Real() Source { return realSource{} }

// Fixed is a source that always returns the same time. Pin "now" in tests with
// clock.SetSource(clock.Fixed(t0)).
type Fixed time.Time

func (f Fixed) Now() time.Time { return time.Time(f) }

// Func is a source backed by a function, for sources that must advance.
type Func func() time.Time

func (f Func) Now() time.Time { return f() }

// DefaultLocation is the location used when localizing timestamps for client
// responses. It defaults to WIB (UTC+7) because the product is used mainly by
// users in that zone. Override it once at startup from configuration
// (e.g. clock.DefaultLocation = time.UTC) if a different zone is needed.
//
// Storage must stay UTC: keep timestamp columns TIMESTAMPTZ so the instant is
// stored as UTC regardless of this setting. Only serialization should convert
// via InDefaultLocation.
var DefaultLocation = time.FixedZone("WIB", 7*3600)

// InDefaultLocation returns t localized to DefaultLocation for serialization.
func InDefaultLocation(t time.Time) time.Time {
	return t.In(DefaultLocation)
}
