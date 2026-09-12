package daemon

import (
	"github.com/netikras/procfit/internal/config"
	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
)

// loadConfig discovers and loads config. ok is false when no config file exists
// (not an error).
func loadConfig(reg *metrics.Registry, dims *query.Dimensions) (config.Config, bool, error) {
	d := config.Discovery{ConfigDir: config.DefaultConfigDir()}
	path, err := d.Discover()
	if err != nil || path == "" {
		return config.Config{}, false, err
	}
	cfg, err := config.LoadFile(path, daemonValidator(reg, dims))
	if err != nil {
		return config.Config{}, false, err
	}
	return cfg, true, nil
}

// loadPolicies converts a config's [[managed]] rules into control policy specs.
func loadPolicies(reg *metrics.Registry, dims *query.Dimensions) ([]control.PolicySpec, error) {
	cfg, ok, err := loadConfig(reg, dims)
	if err != nil || !ok {
		return nil, err
	}
	return toPolicies(cfg.Managed), nil
}

func toPolicies(rules []config.Managed) []control.PolicySpec {
	var out []control.PolicySpec
	for _, m := range rules {
		p := control.PolicySpec{Name: m.Name, Selector: m.Selector, OnDrift: m.OnDrift}
		if m.HasControl {
			p.Nice = m.Control.Nice
			p.Stop = m.Control.Stop
		}
		out = append(out, p)
	}
	return out
}

func daemonValidator(reg *metrics.Registry, dims *query.Dimensions) config.Validator {
	return config.Validator{
		HasMetric:    func(s string) bool { return reg.Has(s) },
		HasDimension: func(s string) bool { return dims.Has(s) },
		IsRateField: func(s string) bool {
			d, ok := reg.Get(s)
			return ok && d.IsRate()
		},
		EntityFields: queryspec.AllowedEntityFields(reg),
		KnownProfile: metrics.IsKnownProfile,
	}
}
