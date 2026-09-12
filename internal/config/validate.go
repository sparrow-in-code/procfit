package config

import (
	"fmt"

	"github.com/netikras/procfit/internal/expr"
)

// Validator supplies the registry-backed predicates config validation needs,
// injected so the config package stays decoupled from metrics/query (DIP).
type Validator struct {
	HasMetric    func(string) bool
	HasDimension func(string) bool
	IsRateField  func(string) bool
	EntityFields map[string]bool
	KnownProfile func(string) bool
	ValidColumn  func(string) bool
}

var validLeaf = map[string]bool{"process": true, "thread": true, "none": true}
var validDir = map[string]bool{"asc": true, "desc": true}
var validDrift = map[string]bool{"report": true, "reapply": true, "adopt": true, "ignore": true}
var validDaemonUse = map[string]bool{"auto": true, "require": true, "never": true}

// Validate runs all semantic checks on a normalized Config.
func (v Validator) Validate(c Config) error {
	if c.Version != 1 {
		return fmt.Errorf("version: unsupported version %d (want 1)", c.Version)
	}
	if !validLeaf[c.Leaf] {
		return fmt.Errorf("leaf: invalid value %q", c.Leaf)
	}
	if c.TargetWidth < 0 {
		return fmt.Errorf("target_width: must be >= 0, got %d", c.TargetWidth)
	}
	if err := v.validateGroupBy(c.GroupBy); err != nil {
		return err
	}
	if err := v.validateMetrics(c.Metrics); err != nil {
		return err
	}
	if err := validateSort(c.Sort); err != nil {
		return err
	}
	if !validDaemonUse[c.Daemon.Use] {
		return fmt.Errorf("daemon.use: invalid value %q", c.Daemon.Use)
	}
	if err := v.validatePresets(c.Presets); err != nil {
		return err
	}
	return v.validateManaged(c.Managed)
}

func (v Validator) validateGroupBy(gb []string) error {
	if len(gb) == 0 {
		return nil
	}
	if len(gb) == 1 && gb[0] == "none" {
		return nil
	}
	seen := map[string]bool{}
	for _, d := range gb {
		if d == "none" {
			return fmt.Errorf("group_by: 'none' must be the only value")
		}
		if seen[d] {
			return fmt.Errorf("group_by: duplicate dimension %q", d)
		}
		seen[d] = true
		if v.HasDimension != nil && !v.HasDimension(d) {
			return fmt.Errorf("group_by: unknown dimension %q", d)
		}
	}
	return nil
}

func (v Validator) validateMetrics(m MetricsSel) error {
	if m.Profile != "" && v.KnownProfile != nil && !v.KnownProfile(m.Profile) {
		return fmt.Errorf("metrics.profile: unknown profile %q", m.Profile)
	}
	for _, id := range append(append([]string{}, m.Enable...), m.Disable...) {
		if v.HasMetric != nil && !v.HasMetric(id) {
			return fmt.Errorf("metrics: unknown metric %q", id)
		}
	}
	return nil
}

func validateSort(keys []SortKey) error {
	for _, k := range keys {
		if k.Field == "" {
			return fmt.Errorf("sort: empty field")
		}
		if !validDir[k.Direction] {
			return fmt.Errorf("sort: invalid direction %q for %q", k.Direction, k.Field)
		}
	}
	return nil
}

func (v Validator) validatePresets(presets map[string]Preset) error {
	for name, p := range presets {
		if p.Leaf != "" && !validLeaf[p.Leaf] {
			return fmt.Errorf("presets.%s.leaf: invalid value %q", name, p.Leaf)
		}
		if err := v.validateGroupBy(p.GroupBy); err != nil {
			return fmt.Errorf("presets.%s.%w", name, err)
		}
		if err := v.validateMetrics(p.Metrics); err != nil {
			return fmt.Errorf("presets.%s.%w", name, err)
		}
		if p.Having != "" {
			if _, err := expr.Compile(p.Having); err != nil {
				return fmt.Errorf("presets.%s.having: %w", name, err)
			}
		}
	}
	return detectPresetCycles(presets)
}

func detectPresetCycles(presets map[string]Preset) error {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var visit func(name string) error
	visit = func(name string) error {
		p, ok := presets[name]
		if !ok {
			return fmt.Errorf("preset %q extends missing parent", name)
		}
		color[name] = gray
		if p.Extends != "" {
			switch color[p.Extends] {
			case gray:
				return fmt.Errorf("preset inheritance cycle at %q", p.Extends)
			case white:
				if err := visit(p.Extends); err != nil {
					return err
				}
			}
		}
		color[name] = black
		return nil
	}
	for name := range presets {
		if color[name] == white {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v Validator) validateManaged(rules []Managed) error {
	seen := map[string]bool{}
	for _, m := range rules {
		if m.Name == "" {
			return fmt.Errorf("managed: rule with empty name")
		}
		if seen[m.Name] {
			return fmt.Errorf("managed: duplicate rule name %q", m.Name)
		}
		seen[m.Name] = true
		if m.OnDrift != "" && !validDrift[m.OnDrift] {
			return fmt.Errorf("managed[%s].on_drift: invalid value %q", m.Name, m.OnDrift)
		}
		if err := v.validateSelector(m.Name, m.Selector); err != nil {
			return err
		}
	}
	return nil
}

func (v Validator) validateSelector(name, sel string) error {
	if sel == "" {
		return fmt.Errorf("managed[%s]: empty selector", name)
	}
	prog, err := expr.Compile(sel)
	if err != nil {
		return fmt.Errorf("managed[%s].selector: %w", name, err)
	}
	for _, f := range prog.Fields() {
		// Persistent selectors must use stable identity fields, not rate metrics
		// (RFC §6.5), which can flap.
		if v.IsRateField != nil && v.IsRateField(f) {
			return fmt.Errorf("managed[%s].selector: rate metric %q not allowed in a persistent selector", name, f)
		}
		if v.EntityFields != nil && !v.EntityFields[f] {
			return fmt.Errorf("managed[%s].selector: unknown field %q", name, f)
		}
	}
	return nil
}
