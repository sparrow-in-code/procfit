// Package render holds presentation helpers shared by the concrete renderers
// (table, json, ...). Renderers are pure presentation: they never collect,
// filter, group, or aggregate (RFC §6.1). Unavailable values are shown by reason
// and never as zero (RFC §6.4).
package render

import (
	"fmt"
	"math"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
)

// FormatMetric renders a metric value for text output using its descriptor's
// unit. Unavailable values render as a reason-appropriate placeholder.
func FormatMetric(desc metrics.Descriptor, v model.MetricValue) string {
	if v.Quality == "mixed" {
		return "mixed"
	}
	if !v.Present() {
		return placeholder(v.Availability)
	}
	return formatUnit(desc.Unit, v.V)
}

func placeholder(a model.Availability) string {
	switch a {
	case model.PermissionDenied, model.ReadError:
		return "?"
	default:
		return "-"
	}
}

func formatUnit(u metrics.Unit, val float64) string {
	switch u {
	case metrics.UnitBytes, metrics.UnitBytesPerSec:
		return humanBytes(val)
	case metrics.UnitPercentOneCPU, metrics.UnitPercentHost:
		return fmt.Sprintf("%.1f", val)
	case metrics.UnitPerSec, metrics.UnitCount:
		return humanCount(val)
	case metrics.UnitDuration:
		return humanDuration(val)
	case metrics.UnitInteger:
		return fmt.Sprintf("%d", int64(math.Round(val)))
	default:
		return fmt.Sprintf("%g", val)
	}
}

// humanBytes formats bytes with binary (1024) scaling, e.g. 3.8G, 14G, 512K.
func humanBytes(b float64) string {
	const unit = 1024.0
	if b < unit {
		return fmt.Sprintf("%d", int64(b))
	}
	suffixes := []string{"K", "M", "G", "T", "P"}
	v := b
	i := -1
	for v >= unit && i < len(suffixes)-1 {
		v /= unit
		i++
	}
	return trimNum(v) + suffixes[i]
}

// humanCount formats counts/rates with decimal (1000) scaling.
func humanCount(c float64) string {
	if c < 1000 {
		return fmt.Sprintf("%d", int64(math.Round(c)))
	}
	suffixes := []string{"K", "M", "G", "T"}
	v := c
	i := -1
	for v >= 1000 && i < len(suffixes)-1 {
		v /= 1000
		i++
	}
	return trimNum(v) + suffixes[i]
}

func humanDuration(sec float64) string {
	s := int64(sec)
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm", s/60)
	case s < 86400:
		return fmt.Sprintf("%dh", s/3600)
	default:
		return fmt.Sprintf("%dd", s/86400)
	}
}

// trimNum renders with one decimal below 10, otherwise integer, for compact
// human output (3.8, 14, 512).
func trimNum(v float64) string {
	if v < 10 {
		return fmt.Sprintf("%.1f", v)
	}
	return fmt.Sprintf("%.0f", v)
}
