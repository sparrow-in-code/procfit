package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	yaml "gopkg.in/yaml.v3"
)

// outConfig is the serialization view of a Config: durations become strings and
// empty values are omitted, so all three encoders emit clean, equivalent output.
type outConfig struct {
	Version   int                  `toml:"version" yaml:"version" json:"version"`
	Interval  string               `toml:"interval,omitempty" yaml:"interval,omitempty" json:"interval,omitempty"`
	GroupBy   []string             `toml:"group_by,omitempty" yaml:"group_by,omitempty" json:"group_by,omitempty"`
	Leaf      string               `toml:"leaf,omitempty" yaml:"leaf,omitempty" json:"leaf,omitempty"`
	Columns   []string             `toml:"columns,omitempty" yaml:"columns,omitempty" json:"columns,omitempty"`
	Intervals map[string]string    `toml:"intervals,omitempty" yaml:"intervals,omitempty" json:"intervals,omitempty"`
	Metrics   *outMetrics          `toml:"metrics,omitempty" yaml:"metrics,omitempty" json:"metrics,omitempty"`
	Sort      []outSort            `toml:"sort,omitempty" yaml:"sort,omitempty" json:"sort,omitempty"`
	State     *outState            `toml:"state,omitempty" yaml:"state,omitempty" json:"state,omitempty"`
	Daemon    *outDaemon           `toml:"daemon,omitempty" yaml:"daemon,omitempty" json:"daemon,omitempty"`
	Presets   map[string]outPreset `toml:"presets,omitempty" yaml:"presets,omitempty" json:"presets,omitempty"`
	Managed   []outManaged         `toml:"managed,omitempty" yaml:"managed,omitempty" json:"managed,omitempty"`
}

type outMetrics struct {
	Profile string   `toml:"profile,omitempty" yaml:"profile,omitempty" json:"profile,omitempty"`
	Enable  []string `toml:"enable,omitempty" yaml:"enable,omitempty" json:"enable,omitempty"`
	Disable []string `toml:"disable,omitempty" yaml:"disable,omitempty" json:"disable,omitempty"`
}

type outSort struct {
	Field     string `toml:"field" yaml:"field" json:"field"`
	Direction string `toml:"direction" yaml:"direction" json:"direction"`
}

type outState struct {
	RuntimeDir string `toml:"runtime_dir,omitempty" yaml:"runtime_dir,omitempty" json:"runtime_dir,omitempty"`
	History    bool   `toml:"history" yaml:"history" json:"history"`
}

type outDaemon struct {
	Use    string `toml:"use,omitempty" yaml:"use,omitempty" json:"use,omitempty"`
	Socket string `toml:"socket,omitempty" yaml:"socket,omitempty" json:"socket,omitempty"`
}

type outPreset struct {
	Extends  string      `toml:"extends,omitempty" yaml:"extends,omitempty" json:"extends,omitempty"`
	Interval string      `toml:"interval,omitempty" yaml:"interval,omitempty" json:"interval,omitempty"`
	GroupBy  []string    `toml:"group_by,omitempty" yaml:"group_by,omitempty" json:"group_by,omitempty"`
	Leaf     string      `toml:"leaf,omitempty" yaml:"leaf,omitempty" json:"leaf,omitempty"`
	Columns  []string    `toml:"columns,omitempty" yaml:"columns,omitempty" json:"columns,omitempty"`
	Having   string      `toml:"having,omitempty" yaml:"having,omitempty" json:"having,omitempty"`
	Metrics  *outMetrics `toml:"metrics,omitempty" yaml:"metrics,omitempty" json:"metrics,omitempty"`
	Sort     []outSort   `toml:"sort,omitempty" yaml:"sort,omitempty" json:"sort,omitempty"`
}

type outManaged struct {
	Name     string      `toml:"name" yaml:"name" json:"name"`
	Selector string      `toml:"selector" yaml:"selector" json:"selector"`
	OnDrift  string      `toml:"on_drift,omitempty" yaml:"on_drift,omitempty" json:"on_drift,omitempty"`
	Control  *outControl `toml:"control,omitempty" yaml:"control,omitempty" json:"control,omitempty"`
}

type outControl struct {
	Nice   *int  `toml:"nice,omitempty" yaml:"nice,omitempty" json:"nice,omitempty"`
	Freeze *bool `toml:"freeze,omitempty" yaml:"freeze,omitempty" json:"freeze,omitempty"`
	Stop   *bool `toml:"stop,omitempty" yaml:"stop,omitempty" json:"stop,omitempty"`
}

func dur(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.String()
}

func toOut(c Config) outConfig {
	o := outConfig{
		Version: c.Version, Interval: dur(c.Interval), GroupBy: c.GroupBy,
		Leaf: c.Leaf, Columns: c.Columns,
	}
	if len(c.Intervals) > 0 {
		o.Intervals = make(map[string]string, len(c.Intervals))
		for k, v := range c.Intervals {
			o.Intervals[k] = v.String()
		}
	}
	if c.Metrics.Profile != "" || len(c.Metrics.Enable) > 0 || len(c.Metrics.Disable) > 0 {
		o.Metrics = &outMetrics{Profile: c.Metrics.Profile, Enable: c.Metrics.Enable, Disable: c.Metrics.Disable}
	}
	for _, s := range c.Sort {
		o.Sort = append(o.Sort, outSort(s))
	}
	o.State = &outState{RuntimeDir: c.State.RuntimeDir, History: c.State.History}
	o.Daemon = &outDaemon{Use: c.Daemon.Use, Socket: c.Daemon.Socket}
	o.Presets = presetsToOut(c.Presets)
	o.Managed = managedToOut(c.Managed)
	return o
}

func presetsToOut(in map[string]Preset) map[string]outPreset {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]outPreset, len(in))
	for name, p := range in {
		op := outPreset{Extends: p.Extends, Interval: dur(p.Interval), GroupBy: p.GroupBy, Leaf: p.Leaf, Columns: p.Columns, Having: p.Having}
		if p.Metrics.Profile != "" || len(p.Metrics.Enable) > 0 || len(p.Metrics.Disable) > 0 {
			op.Metrics = &outMetrics{Profile: p.Metrics.Profile, Enable: p.Metrics.Enable, Disable: p.Metrics.Disable}
		}
		for _, s := range p.Sort {
			op.Sort = append(op.Sort, outSort(s))
		}
		out[name] = op
	}
	return out
}

func managedToOut(in []Managed) []outManaged {
	if len(in) == 0 {
		return nil
	}
	out := make([]outManaged, 0, len(in))
	for _, m := range in {
		om := outManaged{Name: m.Name, Selector: m.Selector, OnDrift: m.OnDrift}
		if m.HasControl {
			om.Control = &outControl{Nice: m.Control.Nice, Freeze: m.Control.Freeze, Stop: m.Control.Stop}
		}
		out = append(out, om)
	}
	return out
}

// Encode serializes a Config to the requested format.
func Encode(c Config, format Format) ([]byte, error) {
	out := toOut(c)
	switch format {
	case FormatTOML:
		return toml.Marshal(out)
	case FormatYAML:
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(out); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	case FormatJSON:
		return json.MarshalIndent(out, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}
