package procfs

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/model"
)

// LoadAvgCollector reads the host load average (1/5/15 min) from /proc/loadavg.
// Host-scoped: one read per sample, no per-process work.
type LoadAvgCollector struct{ root string }

// NewLoadAvgCollector builds the collector rooted at the given procfs path.
func NewLoadAvgCollector(root string) *LoadAvgCollector {
	if root == "" {
		root = "/proc"
	}
	return &LoadAvgCollector{root: root}
}

// ID names the source.
func (c *LoadAvgCollector) ID() string { return "loadavg" }

var loadIDs = []model.MetricID{"load-1", "load-5", "load-15"}

// Metrics lists the produced ids.
func (c *LoadAvgCollector) Metrics() []model.MetricID { return loadIDs }

// CollectHost returns the three load averages, or all unavailable if the file
// cannot be read/parsed.
func (c *LoadAvgCollector) CollectHost(_ context.Context) map[model.MetricID]model.MetricValue {
	out := make(map[model.MetricID]model.MetricValue, len(loadIDs))
	data, err := os.ReadFile(filepath.Join(c.root, "loadavg"))
	vals, ok := parseLoadAvg(string(data))
	for i, id := range loadIDs {
		switch {
		case err != nil:
			out[id] = model.Unavailable[float64](model.Unsupported, "loadavg")
		case !ok:
			out[id] = model.Unavailable[float64](model.ReadError, "loadavg")
		default:
			out[id] = model.NewValue(vals[i], model.Exact, "loadavg")
		}
	}
	return out
}

// parseLoadAvg extracts the first three fields of /proc/loadavg
// ("0.52 0.58 0.59 2/1234 5678").
func parseLoadAvg(s string) ([3]float64, bool) {
	var out [3]float64
	f := strings.Fields(s)
	if len(f) < 3 {
		return out, false
	}
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseFloat(f[i], 64)
		if err != nil {
			return out, false
		}
		out[i] = v
	}
	return out, true
}
