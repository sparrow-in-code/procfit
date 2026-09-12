// Package ports defines the interfaces (ports) that the OS-agnostic core owns
// and that platform/tool-specific adapters implement (DEVELOPMENT.md §2.4, RFC
// §24/§25). The core depends only on these interfaces; concrete syscalls, /proc
// access, transports, and the wall clock live in the outer ring and are injected
// in. This keeps the core portable (Linux today; Windows/macOS/BSD are additive
// adapters) and fully testable via fakes in internal/testutil.
package ports

import "time"

// Clock abstracts time so business logic never calls time.Now() directly. This
// enables deterministic tests and correct rate math based on a monotonic source
// (RFC §13.6: rates use actual elapsed monotonic duration, not the configured
// interval).
type Clock interface {
	// Now returns the current wall-clock time, for display and timestamps.
	Now() time.Time
	// NowMono returns a monotonic reading suitable for measuring elapsed
	// durations between samples. It is not related to wall time.
	NowMono() time.Duration
}
