package daemon

import (
	"github.com/netikras/procfit/internal/config"
	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
)

// loadPolicies discovers and loads config, converting [[managed]] rules into
// control policy specs. An absent config yields no policies (not an error).
func loadPolicies(reg *metrics.Registry, dims *query.Dimensions) ([]control.PolicySpec, error) {
	d := config.Discovery{ConfigDir: config.DefaultConfigDir()}
	path, err := d.Discover()
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	cfg, err := config.LoadFile(path, daemonValidator(reg, dims))
	if err != nil {
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
