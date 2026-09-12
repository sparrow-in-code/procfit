// Package testutil provides first-class test doubles (fakes) that satisfy the
// same port contracts as production adapters (Liskov, DEVELOPMENT.md §2.2). They
// let the whole codebase be tested without root or a real /proc (RFC §31.3).
package testutil

import "time"

// FakeClock is a controllable ports.Clock. Both wall and monotonic readings
// advance together under Advance, giving deterministic rate math in tests.
type FakeClock struct {
	wall time.Time
	mono time.Duration
}

// NewFakeClock returns a clock starting at the given wall time and mono=0.
func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{wall: start}
}

// Now returns the current fake wall time.
func (c *FakeClock) Now() time.Time { return c.wall }

// NowMono returns the current fake monotonic reading.
func (c *FakeClock) NowMono() time.Duration { return c.mono }

// Advance moves both clocks forward by d.
func (c *FakeClock) Advance(d time.Duration) {
	c.wall = c.wall.Add(d)
	c.mono += d
}
