package config

import (
	"fmt"
	"time"
)

// normalize converts a strictly-decoded rawConfig into the canonical Config,
// parsing durations and applying built-in defaults for absent scalars.
func normalize(raw rawConfig) (Config, error) {
	c := Config{
		Version: 1,
		Leaf:    "process",
	}
	if raw.Version != nil {
		c.Version = *raw.Version
	}
	if err := setDuration(&c.Interval, raw.Interval, "interval"); err != nil {
		return c, err
	}
	c.GroupBy = raw.GroupBy
	if raw.Leaf != nil {
		c.Leaf = *raw.Leaf
	}
	c.Columns = raw.Columns

	intervals, err := normalizeIntervals(raw.Intervals)
	if err != nil {
		return c, err
	}
	c.Intervals = intervals
	c.Metrics = normalizeMetrics(raw.Metrics)
	c.Sort = normalizeSort(raw.Sort)
	c.State = normalizeState(raw.State)
	c.Daemon = normalizeDaemon(raw.Daemon)

	presets, err := normalizePresets(raw.Presets)
	if err != nil {
		return c, err
	}
	c.Presets = presets

	managed, err := normalizeManaged(raw.Managed)
	if err != nil {
		return c, err
	}
	c.Managed = managed

	if raw.AllowBuiltinPresetOverride != nil {
		c.AllowBuiltinPresetOverride = *raw.AllowBuiltinPresetOverride
	}
	return c, nil
}

func setDuration(dst *time.Duration, s *string, field string) error {
	if s == nil {
		return nil
	}
	d, err := time.ParseDuration(*s)
	if err != nil {
		return fmt.Errorf("%s: invalid duration %q", field, *s)
	}
	if d < 0 {
		return fmt.Errorf("%s: negative duration %q", field, *s)
	}
	*dst = d
	return nil
}

func normalizeIntervals(in map[string]string) (map[string]time.Duration, error) {
	if in == nil {
		return nil, nil
	}
	out := make(map[string]time.Duration, len(in))
	for k, v := range in {
		var d time.Duration
		if err := setDuration(&d, &v, "intervals."+k); err != nil {
			return nil, err
		}
		out[k] = d
	}
	return out, nil
}

func normalizeMetrics(m *rawMetrics) MetricsSel {
	if m == nil {
		return MetricsSel{}
	}
	sel := MetricsSel{Enable: m.Enable, Disable: m.Disable}
	if m.Profile != nil {
		sel.Profile = *m.Profile
	}
	return sel
}

func normalizeSort(in []rawSort) []SortKey {
	if len(in) == 0 {
		return nil
	}
	out := make([]SortKey, len(in))
	for i, s := range in {
		dir := s.Direction
		if dir == "" {
			dir = "asc"
		}
		out[i] = SortKey{Field: s.Field, Direction: dir}
	}
	return out
}

func normalizeState(s *rawState) State {
	var st State
	if s == nil {
		return st
	}
	if s.RuntimeDir != nil {
		st.RuntimeDir = *s.RuntimeDir
	}
	if s.History != nil {
		st.History = *s.History
	}
	return st
}

func normalizeDaemon(d *rawDaemon) Daemon {
	dm := Daemon{Use: "auto"}
	if d == nil {
		return dm
	}
	if d.Use != nil {
		dm.Use = *d.Use
	}
	if d.Socket != nil {
		dm.Socket = *d.Socket
	}
	return dm
}

func normalizePresets(in map[string]rawPreset) (map[string]Preset, error) {
	if in == nil {
		return nil, nil
	}
	out := make(map[string]Preset, len(in))
	for name, rp := range in {
		p := Preset{Leaf: "", GroupBy: rp.GroupBy, Columns: rp.Columns, Metrics: normalizeMetrics(rp.Metrics), Sort: normalizeSort(rp.Sort)}
		if rp.Extends != nil {
			p.Extends = *rp.Extends
		}
		if rp.Leaf != nil {
			p.Leaf = *rp.Leaf
		}
		if rp.Having != nil {
			p.Having = *rp.Having
		}
		if err := setDuration(&p.Interval, rp.Interval, "presets."+name+".interval"); err != nil {
			return nil, err
		}
		out[name] = p
	}
	return out, nil
}

func normalizeManaged(in []rawManaged) ([]Managed, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]Managed, 0, len(in))
	for _, rm := range in {
		m := Managed{Name: rm.Name, Selector: rm.Selector, OnDrift: "report"}
		if rm.OnDrift != nil {
			m.OnDrift = *rm.OnDrift
		}
		if rm.View != nil {
			m.View = View{GroupBy: rm.View.GroupBy, Columns: rm.View.Columns}
			if rm.View.Leaf != nil {
				m.View.Leaf = *rm.View.Leaf
			}
		}
		if rm.Control != nil {
			m.HasControl = true
			m.Control = Control{Nice: rm.Control.Nice, Freeze: rm.Control.Freeze, Stop: rm.Control.Stop}
		}
		out = append(out, m)
	}
	return out, nil
}
