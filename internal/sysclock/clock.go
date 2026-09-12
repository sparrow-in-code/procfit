// Package sysclock provides the production ports.Clock backed by the system wall
// and monotonic clocks. Business logic depends on ports.Clock and receives this
// (or a fake) by injection, never calling time.Now directly.
package sysclock

import "time"

// Clock is the real system clock.
type Clock struct {
	base time.Time
}

// New returns a real clock. NowMono is measured relative to construction using
// the monotonic component captured in base.
func New() *Clock { return &Clock{base: time.Now()} }

// Now returns the current wall-clock time.
func (c *Clock) Now() time.Time { return time.Now() }

// NowMono returns a monotonic elapsed reading since construction.
func (c *Clock) NowMono() time.Duration { return time.Since(c.base) }
