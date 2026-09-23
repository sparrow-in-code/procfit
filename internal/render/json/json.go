// Package json renders a query.Result as versioned machine output (RFC §18.4).
// The JSON schema is a public, versioned interface; the table format is not.
package json

import (
	"encoding/json"
	"io"
	"time"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

// SchemaVersion is the version of the JSON output envelope.
const SchemaVersion = 1

type envelope struct {
	SchemaVersion      int       `json:"schema_version"`
	GeneratedAt        time.Time `json:"generated_at"`
	MonotonicElapsedNS int64     `json:"monotonic_elapsed_ns"`
	Generation         int       `json:"generation"`
	// Host carries whole-machine (ScopeHost) metrics — power, C-state, pressure —
	// which are not per-process. Additive, optional (omitted when none).
	Host map[string]jsonValue `json:"host,omitempty"`
	Rows []jsonRow            `json:"rows"`
}

type jsonRow struct {
	Kind    string               `json:"kind"`
	Label   string               `json:"label"`
	Key     string               `json:"key,omitempty"`
	Procs   int                  `json:"procs"`
	Threads int                  `json:"threads"`
	Leaves  int                  `json:"leaves"`
	PID     *int                 `json:"pid,omitempty"`
	Metrics map[string]jsonValue `json:"metrics,omitempty"`
	Sub     []jsonRow            `json:"children,omitempty"`
}

type jsonValue struct {
	Value        *float64 `json:"value"`
	Unit         string   `json:"unit"`
	Availability string   `json:"availability"`
	Quality      string   `json:"quality,omitempty"`
	Source       string   `json:"source,omitempty"`
}

// Render writes the result as JSON.
func Render(w io.Writer, res *query.Result, reg *metrics.Registry) error {
	env := envelope{
		SchemaVersion:      SchemaVersion,
		GeneratedAt:        res.WallTime,
		MonotonicElapsedNS: res.Elapsed.Nanoseconds(),
		Generation:         res.Generation,
		Host:               convertMetrics(res.HostMetrics, reg),
		Rows:               convertRows(res.Rows, reg),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

func convertRows(rows []*query.Row, reg *metrics.Registry) []jsonRow {
	out := make([]jsonRow, 0, len(rows))
	for _, r := range rows {
		jr := jsonRow{
			Kind: string(r.Kind), Label: r.Label, Key: r.Key,
			Procs: r.Procs, Threads: r.Threads, Leaves: r.Leaves,
			Metrics: convertMetrics(r.Metrics, reg),
			Sub:     convertRows(r.Sub, reg),
		}
		if r.Process != nil {
			pid := r.Process.PID
			jr.PID = &pid
		}
		out = append(out, jr)
	}
	return out
}

func convertMetrics(m map[model.MetricID]model.MetricValue, reg *metrics.Registry) map[string]jsonValue {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]jsonValue, len(m))
	for id, v := range m {
		unit := ""
		if d, ok := reg.Get(string(id)); ok {
			unit = string(d.Unit)
		}
		jv := jsonValue{
			Unit:         unit,
			Availability: string(v.Availability),
			Quality:      string(v.Quality),
			Source:       v.Source,
		}
		if v.Present() {
			val := v.V
			jv.Value = &val
		}
		out[string(id)] = jv
	}
	return out
}
