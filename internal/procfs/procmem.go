package procfs

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/model"
)

// maxProcMemScans bounds concurrent per-process memory-detail reads.
const maxProcMemScans = 32

// ProcMemCollector adds cost-1 per-process memory detail that the light path does
// not read: proportional/unique set size from /proc/PID/smaps_rollup (a fair
// accounting of shared memory) and OOM kill-likelihood from /proc/PID/oom_score*.
// Absent/unreadable sources degrade to unavailable, never zero.
type ProcMemCollector struct {
	root string
	sem  chan struct{}
}

// NewProcMemCollector builds the collector rooted at the given procfs path.
func NewProcMemCollector(root string) *ProcMemCollector {
	if root == "" {
		root = "/proc"
	}
	return &ProcMemCollector{root: root, sem: make(chan struct{}, maxProcMemScans)}
}

// ID matches Descriptor.Collector for the procmem metrics.
func (c *ProcMemCollector) ID() string { return "procmem" }

// Metrics lists the produced ids.
func (c *ProcMemCollector) Metrics() []model.MetricID {
	return []model.MetricID{"pss", "uss", "swap-pss", "oom-score", "oom-score-adj"}
}

// Collect enriches each process with smaps_rollup + oom_score detail, bounded.
func (c *ProcMemCollector) Collect(ctx context.Context, procs []model.Process) {
	done := make(chan struct{})
	var launched int
	for i := range procs {
		if ctx.Err() != nil {
			break
		}
		launched++
		go func(p *model.Process) {
			c.sem <- struct{}{}
			defer func() { <-c.sem; done <- struct{}{} }()
			c.collectOne(p)
		}(&procs[i])
	}
	for i := 0; i < launched; i++ {
		<-done
	}
}

func (c *ProcMemCollector) collectOne(p *model.Process) {
	dir := filepath.Join(c.root, itoa(p.PID))
	c.applyRollup(p, dir)
	c.applyOOM(p, dir)
}

func (c *ProcMemCollector) applyRollup(p *model.Process, dir string) {
	data, err := os.ReadFile(filepath.Join(dir, "smaps_rollup"))
	if err != nil {
		avail := classifyReadErr(err)
		for _, id := range []model.MetricID{"pss", "uss", "swap-pss"} {
			p.SetMetric(id, model.Unavailable[float64](avail, "procmem"))
		}
		return
	}
	r := parseSmapsRollup(string(data))
	p.SetMetric("pss", model.NewValue(float64(r.pss), model.Sampled, "procmem"))
	p.SetMetric("uss", model.NewValue(float64(r.uss()), model.Sampled, "procmem"))
	p.SetMetric("swap-pss", model.NewValue(float64(r.swapPss), model.Sampled, "procmem"))
}

func (c *ProcMemCollector) applyOOM(p *model.Process, dir string) {
	if v, ok := readIntFile(filepath.Join(dir, "oom_score")); ok {
		p.SetMetric("oom-score", model.NewValue(float64(v), model.Sampled, "procmem"))
	} else {
		p.SetMetric("oom-score", model.Unavailable[float64](model.ReadError, "procmem"))
	}
	if v, ok := readIntFile(filepath.Join(dir, "oom_score_adj")); ok {
		p.SetMetric("oom-score-adj", model.NewValue(float64(v), model.Sampled, "procmem"))
	} else {
		p.SetMetric("oom-score-adj", model.Unavailable[float64](model.ReadError, "procmem"))
	}
}

// smapsRollup holds the byte figures we take from /proc/PID/smaps_rollup.
type smapsRollup struct {
	pss          uint64
	privateClean uint64
	privateDirty uint64
	swapPss      uint64
}

// uss is the unique (private-only) resident memory.
func (r smapsRollup) uss() uint64 { return r.privateClean + r.privateDirty }

// parseSmapsRollup reads the kB fields we need into bytes. Pure/testable.
func parseSmapsRollup(s string) smapsRollup {
	var r smapsRollup
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "Pss:"):
			r.pss = kbToBytes(line[len("Pss:"):])
		case strings.HasPrefix(line, "Private_Clean:"):
			r.privateClean = kbToBytes(line[len("Private_Clean:"):])
		case strings.HasPrefix(line, "Private_Dirty:"):
			r.privateDirty = kbToBytes(line[len("Private_Dirty:"):])
		case strings.HasPrefix(line, "SwapPss:"):
			r.swapPss = kbToBytes(line[len("SwapPss:"):])
		}
	}
	return r
}

// readIntFile reads a small file holding a single (possibly negative) integer.
func readIntFile(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	return n, true
}
