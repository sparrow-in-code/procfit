package procfs

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/model"
)

// PSICollector reads host pressure-stall information from
// /proc/pressure/{cpu,io,memory} — the "some avg10" percentage, the single best
// "why is load high" readout (PM-0514, RFC §13). It is host-scoped and broadcast
// onto every process (AggMax). Absent (CONFIG_PSI off) → unavailable, never zero.
type PSICollector struct{ root string }

// NewPSICollector builds the collector rooted at the given procfs path.
func NewPSICollector(root string) *PSICollector {
	if root == "" {
		root = "/proc"
	}
	return &PSICollector{root: root}
}

// ID names the source.
func (c *PSICollector) ID() string { return "psi" }

// psiFiles maps each metric id to its /proc/pressure file.
var psiFiles = []struct {
	id   model.MetricID
	file string
}{
	{"psi-cpu", "cpu"},
	{"psi-io", "io"},
	{"psi-mem", "memory"},
}

// Metrics lists the produced ids.
func (c *PSICollector) Metrics() []model.MetricID {
	return []model.MetricID{"psi-cpu", "psi-io", "psi-mem"}
}

// CollectHost reads each PSI file into a host metric map.
func (c *PSICollector) CollectHost(_ context.Context) map[model.MetricID]model.MetricValue {
	out := make(map[model.MetricID]model.MetricValue, len(psiFiles))
	for _, m := range psiFiles {
		out[m.id] = c.read(m.file)
	}
	return out
}

func (c *PSICollector) read(file string) model.MetricValue {
	data, err := os.ReadFile(filepath.Join(c.root, "pressure", file))
	if err != nil {
		if os.IsNotExist(err) {
			return model.Unavailable[float64](model.Unsupported, "psi") // CONFIG_PSI off
		}
		return model.Unavailable[float64](model.ReadError, "psi")
	}
	pct, ok := parsePSISomeAvg10(string(data))
	if !ok {
		return model.Unavailable[float64](model.ReadError, "psi")
	}
	return model.NewValue(pct, model.Sampled, "psi")
}

// parsePSISomeAvg10 extracts the "some avg10=" percentage from a PSI file body.
func parsePSISomeAvg10(data string) (float64, bool) {
	for _, line := range strings.Split(data, "\n") {
		if !strings.HasPrefix(line, "some ") {
			continue
		}
		for _, f := range strings.Fields(line) {
			if v, ok := strings.CutPrefix(f, "avg10="); ok {
				n, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return 0, false
				}
				return n, true
			}
		}
	}
	return 0, false
}
