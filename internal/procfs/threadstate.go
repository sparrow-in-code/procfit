package procfs

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/netikras/procfit/internal/model"
)

// ThreadStateCollector counts each process's threads in R (runnable) and D
// (uninterruptible) states from /proc/PID/task/*/stat — the process's own
// contribution to the load average (PM-0514). Cost-1: a per-process task-dir
// scan. A vanished/hidden task dir → unavailable, never a fabricated zero.
type ThreadStateCollector struct{ root string }

// NewThreadStateCollector builds the collector rooted at the given procfs path.
func NewThreadStateCollector(root string) *ThreadStateCollector {
	if root == "" {
		root = "/proc"
	}
	return &ThreadStateCollector{root: root}
}

// ID names the source.
func (c *ThreadStateCollector) ID() string { return "threadstate" }

// Metrics lists the produced ids.
func (c *ThreadStateCollector) Metrics() []model.MetricID {
	return []model.MetricID{"threads-running", "threads-uninterruptible"}
}

// Collect fills the R/D thread counts for each process.
func (c *ThreadStateCollector) Collect(_ context.Context, procs []model.Process) {
	for i := range procs {
		c.count(&procs[i])
	}
}

func (c *ThreadStateCollector) count(p *model.Process) {
	dir := filepath.Join(c.root, itoa(p.PID), "task")
	entries, err := os.ReadDir(dir)
	if err != nil {
		av := model.Vanished
		if os.IsPermission(err) {
			av = model.PermissionDenied
		}
		p.SetMetric("threads-running", model.Unavailable[float64](av, "threadstate"))
		p.SetMetric("threads-uninterruptible", model.Unavailable[float64](av, "threadstate"))
		return
	}
	var running, uninterruptible float64
	for _, e := range entries {
		switch readTaskState(filepath.Join(dir, e.Name(), "stat")) {
		case 'R':
			running++
		case 'D':
			uninterruptible++
		}
	}
	p.SetMetric("threads-running", model.NewValue(running, model.Sampled, "threadstate"))
	p.SetMetric("threads-uninterruptible", model.NewValue(uninterruptible, model.Sampled, "threadstate"))
}

// readTaskState returns the state char (field 3) of a task stat line, taking the
// char after the last ')' so a comm containing spaces/parens is handled. Returns
// 0 on any error.
func readTaskState(path string) byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	s := string(data)
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return 0
	}
	rest := strings.TrimLeft(s[i+1:], " ")
	if rest == "" {
		return 0
	}
	return rest[0]
}
