package app

import "time"

// realClock is the production ports.Clock backed by the wall and monotonic
// system clocks. Business logic never uses it directly; it is injected.
type realClock struct {
	base time.Time
}

func newRealClock() *realClock { return &realClock{base: time.Now()} }

func (c *realClock) Now() time.Time { return time.Now() }

// NowMono returns a monotonic elapsed reading since construction. time.Since
// uses the monotonic clock component captured in base.
func (c *realClock) NowMono() time.Duration { return time.Since(c.base) }
