// Package config decodes TOML/YAML/JSON into one canonical configuration model
// with identical semantics across formats (RFC §9). The pipeline is: strict
// decode into a raw DTO → normalize → semantic validation → resolved Config.
// Strictness (unknown/duplicate keys, bad enums/durations) is enforced so
// configuration — which can stop or reprioritize processes — is trustworthy.
package config

import "time"

// raw* types mirror the on-disk schema using presence-preserving types (pointers
// and slices) and string durations. They are decoded strictly, then normalized.
type rawConfig struct {
	Version                    *int                 `toml:"version" yaml:"version" json:"version"`
	Interval                   *string              `toml:"interval" yaml:"interval" json:"interval"`
	GroupBy                    []string             `toml:"group_by" yaml:"group_by" json:"group_by"`
	Leaf                       *string              `toml:"leaf" yaml:"leaf" json:"leaf"`
	Columns                    []string             `toml:"columns" yaml:"columns" json:"columns"`
	Intervals                  map[string]string    `toml:"intervals" yaml:"intervals" json:"intervals"`
	Metrics                    *rawMetrics          `toml:"metrics" yaml:"metrics" json:"metrics"`
	Sort                       []rawSort            `toml:"sort" yaml:"sort" json:"sort"`
	State                      *rawState            `toml:"state" yaml:"state" json:"state"`
	Daemon                     *rawDaemon           `toml:"daemon" yaml:"daemon" json:"daemon"`
	Presets                    map[string]rawPreset `toml:"presets" yaml:"presets" json:"presets"`
	Managed                    []rawManaged         `toml:"managed" yaml:"managed" json:"managed"`
	TargetWidth                *int                 `toml:"target_width" yaml:"target_width" json:"target_width"`
	AllowBuiltinPresetOverride *bool                `toml:"allow_builtin_preset_override" yaml:"allow_builtin_preset_override" json:"allow_builtin_preset_override"`
}

type rawMetrics struct {
	Profile *string  `toml:"profile" yaml:"profile" json:"profile"`
	Enable  []string `toml:"enable" yaml:"enable" json:"enable"`
	Disable []string `toml:"disable" yaml:"disable" json:"disable"`
}

type rawSort struct {
	Field     string `toml:"field" yaml:"field" json:"field"`
	Direction string `toml:"direction" yaml:"direction" json:"direction"`
}

type rawState struct {
	RuntimeDir *string `toml:"runtime_dir" yaml:"runtime_dir" json:"runtime_dir"`
	History    *bool   `toml:"history" yaml:"history" json:"history"`
}

type rawDaemon struct {
	Use    *string `toml:"use" yaml:"use" json:"use"`
	Socket *string `toml:"socket" yaml:"socket" json:"socket"`
}

type rawPreset struct {
	Extends  *string     `toml:"extends" yaml:"extends" json:"extends"`
	Interval *string     `toml:"interval" yaml:"interval" json:"interval"`
	GroupBy  []string    `toml:"group_by" yaml:"group_by" json:"group_by"`
	Leaf     *string     `toml:"leaf" yaml:"leaf" json:"leaf"`
	Columns  []string    `toml:"columns" yaml:"columns" json:"columns"`
	Having   *string     `toml:"having" yaml:"having" json:"having"`
	Metrics  *rawMetrics `toml:"metrics" yaml:"metrics" json:"metrics"`
	Sort     []rawSort   `toml:"sort" yaml:"sort" json:"sort"`
}

type rawManaged struct {
	Name     string      `toml:"name" yaml:"name" json:"name"`
	Selector string      `toml:"selector" yaml:"selector" json:"selector"`
	OnDrift  *string     `toml:"on_drift" yaml:"on_drift" json:"on_drift"`
	View     *rawView    `toml:"view" yaml:"view" json:"view"`
	Control  *rawControl `toml:"control" yaml:"control" json:"control"`
}

type rawView struct {
	GroupBy []string `toml:"group_by" yaml:"group_by" json:"group_by"`
	Leaf    *string  `toml:"leaf" yaml:"leaf" json:"leaf"`
	Columns []string `toml:"columns" yaml:"columns" json:"columns"`
}

type rawControl struct {
	Nice   *int  `toml:"nice" yaml:"nice" json:"nice"`
	Freeze *bool `toml:"freeze" yaml:"freeze" json:"freeze"`
	Stop   *bool `toml:"stop" yaml:"stop" json:"stop"`
}

// Config is the canonical, normalized configuration.
type Config struct {
	Version                    int
	Interval                   time.Duration
	GroupBy                    []string
	Leaf                       string
	Columns                    []string
	Intervals                  map[string]time.Duration
	Metrics                    MetricsSel
	Sort                       []SortKey
	State                      State
	Daemon                     Daemon
	Presets                    map[string]Preset
	Managed                    []Managed
	TargetWidth                int
	AllowBuiltinPresetOverride bool
}

// MetricsSel is a normalized metric selection.
type MetricsSel struct {
	Profile string
	Enable  []string
	Disable []string
}

// SortKey is a normalized sort key.
type SortKey struct {
	Field     string
	Direction string
}

// State is the normalized [state] block.
type State struct {
	RuntimeDir string
	History    bool
}

// Daemon is the normalized [daemon] block.
type Daemon struct {
	Use    string
	Socket string
}

// Preset is a normalized preset.
type Preset struct {
	Extends  string
	Interval time.Duration
	GroupBy  []string
	Leaf     string
	Columns  []string
	Having   string
	Metrics  MetricsSel
	Sort     []SortKey
}

// Managed is a normalized managed rule.
type Managed struct {
	Name       string
	Selector   string
	OnDrift    string
	View       View
	Control    Control
	HasControl bool
}

// View is a normalized managed view.
type View struct {
	GroupBy []string
	Leaf    string
	Columns []string
}

// Control is a normalized managed control intent.
type Control struct {
	Nice   *int
	Freeze *bool
	Stop   *bool
}
