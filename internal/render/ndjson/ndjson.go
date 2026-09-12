// Package ndjson renders results as newline-delimited JSON: one metadata record
// followed by timestamped sample records (RFC §18.4). It suits streaming (`stat`)
// where each tick appends records without rewriting prior output.
package ndjson

import (
	"encoding/json"
	"io"
	"time"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

// SchemaVersion is the NDJSON record schema version.
const SchemaVersion = 1

// Streamer writes NDJSON records to w. Call WriteMeta once, then WriteResult per
// sample batch.
type Streamer struct {
	enc  *json.Encoder
	reg  *metrics.Registry
	meta bool
}

// NewStreamer builds a streamer.
func NewStreamer(w io.Writer, reg *metrics.Registry) *Streamer {
	return &Streamer{enc: json.NewEncoder(w), reg: reg}
}

type metaRecord struct {
	Record        string `json:"record"`
	SchemaVersion int    `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`
}

type rowRecord struct {
	Record      string               `json:"record"`
	GeneratedAt string               `json:"generated_at"`
	Generation  int                  `json:"generation"`
	Depth       int                  `json:"depth"`
	Kind        string               `json:"kind"`
	Label       string               `json:"label"`
	PID         *int                 `json:"pid,omitempty"`
	Procs       int                  `json:"procs"`
	Threads     int                  `json:"threads"`
	Metrics     map[string]metricRec `json:"metrics,omitempty"`
}

type metricRec struct {
	Value        *float64 `json:"value"`
	Unit         string   `json:"unit"`
	Availability string   `json:"availability"`
}

// WriteMeta emits the single metadata record (idempotent).
func (s *Streamer) WriteMeta() error {
	if s.meta {
		return nil
	}
	s.meta = true
	return s.enc.Encode(metaRecord{Record: "meta", SchemaVersion: SchemaVersion, GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano)})
}

// WriteResult emits one row record per (flattened) row in the result.
func (s *Streamer) WriteResult(res *query.Result) error {
	if err := s.WriteMeta(); err != nil {
		return err
	}
	ts := res.WallTime.UTC().Format(time.RFC3339Nano)
	return s.writeRows(res.Rows, 0, res.Generation, ts)
}

func (s *Streamer) writeRows(rows []*query.Row, depth, gen int, ts string) error {
	for _, r := range rows {
		rec := rowRecord{
			Record: "row", GeneratedAt: ts, Generation: gen, Depth: depth,
			Kind: string(r.Kind), Label: r.Label, Procs: r.Procs, Threads: r.Threads,
			Metrics: s.metrics(r.Metrics),
		}
		if r.Process != nil {
			pid := r.Process.PID
			rec.PID = &pid
		}
		if err := s.enc.Encode(rec); err != nil {
			return err
		}
		if err := s.writeRows(r.Sub, depth+1, gen, ts); err != nil {
			return err
		}
	}
	return nil
}

func (s *Streamer) metrics(m map[model.MetricID]model.MetricValue) map[string]metricRec {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]metricRec, len(m))
	for id, v := range m {
		unit := ""
		if d, ok := s.reg.Get(string(id)); ok {
			unit = string(d.Unit)
		}
		rec := metricRec{Unit: unit, Availability: string(v.Availability)}
		if val, ok := v.Get(); ok {
			rec.Value = &val
		}
		out[string(id)] = rec
	}
	return out
}
