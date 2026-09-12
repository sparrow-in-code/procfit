// Package collect turns raw ProcessSource readings into immutable snapshot
// generations of normalized entities with metric values. It owns rate/delta
// math and the "unknown is not zero" discipline (RFC §13.6, §6.4). Collectors
// are the policy layer over the ProcessSource port; adding a new metric source
// is a new collector, not an edit here (Open/Closed).
package collect

import (
	"time"

	"github.com/netikras/procfit/internal/model"
)

// Snapshot is one immutable generation of sampled processes. Downstream stages
// (query, render) consume it read-only.
type Snapshot struct {
	Generation int
	BootID     string
	WallTime   time.Time
	// Elapsed is the monotonic duration since the previous sample; zero on the
	// first sample. Rates use this actual elapsed time, not the nominal
	// interval (RFC §13.6).
	Elapsed   time.Duration
	Processes []model.Process
}
