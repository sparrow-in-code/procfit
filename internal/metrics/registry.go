package metrics

import (
	"fmt"
	"sort"

	"github.com/netikras/procfit/internal/model"
)

// Registry is a set of metric descriptors indexed by canonical id and alias. It
// is populated once at construction and treated as immutable thereafter
// (DEVELOPMENT.md §2.4). Consumers receive a *Registry by injection rather than
// reaching for a global, which keeps them testable (DIP).
type Registry struct {
	byID    map[model.MetricID]Descriptor
	byAlias map[string]model.MetricID
	order   []model.MetricID
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byID:    make(map[model.MetricID]Descriptor),
		byAlias: make(map[string]model.MetricID),
	}
}

// Register adds a descriptor. It errors on duplicate ids or aliases so
// mis-registration fails loudly at startup rather than silently shadowing.
func (r *Registry) Register(d Descriptor) error {
	if d.ID == "" {
		return fmt.Errorf("metric descriptor has empty id")
	}
	if _, ok := r.byID[d.ID]; ok {
		return fmt.Errorf("duplicate metric id %q", d.ID)
	}
	if _, ok := r.byAlias[string(d.ID)]; ok {
		return fmt.Errorf("metric id %q collides with an existing alias", d.ID)
	}
	r.byID[d.ID] = d
	r.byAlias[string(d.ID)] = d.ID
	for _, a := range d.Aliases {
		if _, ok := r.byAlias[a]; ok {
			return fmt.Errorf("duplicate metric alias %q (from %q)", a, d.ID)
		}
		r.byAlias[a] = d.ID
	}
	r.order = append(r.order, d.ID)
	return nil
}

// MustRegister panics on error; for use with static built-in descriptors.
func (r *Registry) MustRegister(d Descriptor) {
	if err := r.Register(d); err != nil {
		panic(err)
	}
}

// ResolveID maps a canonical id or alias to a canonical id.
func (r *Registry) ResolveID(name string) (model.MetricID, bool) {
	id, ok := r.byAlias[name]
	return id, ok
}

// Get returns the descriptor for a canonical id or alias.
func (r *Registry) Get(name string) (Descriptor, bool) {
	id, ok := r.byAlias[name]
	if !ok {
		return Descriptor{}, false
	}
	return r.byID[id], true
}

// Has reports whether a canonical id or alias is known.
func (r *Registry) Has(name string) bool {
	_, ok := r.byAlias[name]
	return ok
}

// All returns descriptors in registration order.
func (r *Registry) All() []Descriptor {
	out := make([]Descriptor, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.byID[id])
	}
	return out
}

// IDs returns the canonical ids sorted, for deterministic output.
func (r *Registry) IDs() []model.MetricID {
	out := make([]model.MetricID, len(r.order))
	copy(out, r.order)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// NewDefault builds a registry populated with the built-in metrics.
func NewDefault() *Registry {
	r := NewRegistry()
	for _, group := range [][]Descriptor{builtinLight, builtinMem, builtinLoad, builtinFD, builtinGPU, builtinEvent, builtinPerf, builtinPower} {
		for _, d := range group {
			r.MustRegister(d)
		}
	}
	return r
}
