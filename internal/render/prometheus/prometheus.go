// Package prometheus renders a query.Result in the Prometheus text exposition
// format (a public, scrapable interface). Renderers are pure presentation: the
// engine has already sampled, grouped, and aggregated.
package prometheus

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

// namePrefix keeps procfit's series in their own namespace.
const namePrefix = "procfit_"

// Render writes the top-level rows and host metrics of a result as Prometheus
// exposition text. Only top-level rows are emitted (the query's group-by/leaf
// defines the series, so there is no group/leaf double counting — use
// `--leaf none` for pure per-group aggregates). Unavailable values are omitted,
// never written as a fabricated zero.
func Render(w io.Writer, res *query.Result, reg *metrics.Registry) error {
	if res == nil {
		return nil
	}
	if err := renderProcessFamilies(w, res.Rows, reg); err != nil {
		return err
	}
	return renderHostFamilies(w, res.HostMetrics, reg)
}

// renderProcessFamilies emits one family per metric id present on the rows, each
// series labelled by the row's target (and pid for process rows).
func renderProcessFamilies(w io.Writer, rows []*query.Row, reg *metrics.Registry) error {
	name := func(id string) string { return metricName(id) }
	for _, id := range metricIDs(rows) {
		var body strings.Builder
		for _, r := range rows {
			v, ok := r.Metrics[model.MetricID(id)]
			if !ok || !v.Present() {
				continue
			}
			fmt.Fprintf(&body, "%s{%s} %s\n", name(id), rowLabels(r), formatFloat(v.V))
		}
		if body.Len() == 0 {
			continue // no available samples → omit the family entirely
		}
		if err := writeHeader(w, id, reg); err != nil {
			return err
		}
		if _, err := io.WriteString(w, body.String()); err != nil {
			return err
		}
	}
	return nil
}

// renderHostFamilies emits whole-machine metrics as unlabelled series.
func renderHostFamilies(w io.Writer, host map[model.MetricID]model.MetricValue, reg *metrics.Registry) error {
	ids := make([]string, 0, len(host))
	for id := range host {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		v := host[model.MetricID(id)]
		if !v.Present() {
			continue
		}
		if err := writeHeader(w, id, reg); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%s %s\n", metricName(id), formatFloat(v.V)); err != nil {
			return err
		}
	}
	return nil
}

// metricIDs returns the sorted union of metric ids present across the rows.
func metricIDs(rows []*query.Row) []string {
	seen := map[string]bool{}
	for _, r := range rows {
		for id := range r.Metrics {
			seen[string(id)] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func writeHeader(w io.Writer, id string, reg *metrics.Registry) error {
	name := metricName(id)
	help := id
	if d, ok := reg.Get(id); ok && d.Description != "" {
		help = d.Description
	}
	_, err := fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n", name, escapeHelp(help), name)
	return err
}

// metricName maps a metric id to a Prometheus series name: procfit_ + id with
// any non [a-zA-Z0-9_] byte replaced by '_'.
func metricName(id string) string {
	var b strings.Builder
	b.WriteString(namePrefix)
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// rowLabels renders the label set for a row: target always, pid for process rows.
func rowLabels(r *query.Row) string {
	labels := []string{fmt.Sprintf(`target="%s"`, escapeLabel(r.Label))}
	if r.Process != nil {
		labels = append(labels, fmt.Sprintf(`pid="%d"`, r.Process.PID))
	}
	return strings.Join(labels, ",")
}

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return strings.ReplaceAll(s, "\n", `\n`)
}

func escapeHelp(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, "\n", `\n`)
}

// formatFloat prints a value without a trailing exponent for typical ranges.
func formatFloat(v float64) string {
	return fmt.Sprintf("%g", v)
}
