package metrics

import (
	"fmt"

	"github.com/netikras/procfit/internal/model"
)

// Override is a single metric add/remove instruction applied after a profile is
// expanded (RFC §8.2). Bare "--metric cpu" is an add.
type Override struct {
	Add bool
	ID  string
}

// ResolveSelection expands a profile then applies overrides left-to-right,
// returning canonical metric ids in a stable order with duplicates removed.
// Unknown metric names are errors (RFC §8.2). This is shared by CLI flag
// handling and config metric blocks so both behave identically.
func (r *Registry) ResolveSelection(profile ProfileName, overrides []Override) ([]model.MetricID, error) {
	base, err := r.Resolve(profile)
	if err != nil {
		return nil, err
	}
	// Ordered set preserving first-seen order.
	order := make([]model.MetricID, 0, len(base))
	present := make(map[model.MetricID]bool)
	add := func(id model.MetricID) {
		if !present[id] {
			present[id] = true
			order = append(order, id)
		}
	}
	remove := func(id model.MetricID) {
		if present[id] {
			present[id] = false
			for i, x := range order {
				if x == id {
					order = append(order[:i], order[i+1:]...)
					break
				}
			}
		}
	}
	for _, id := range base {
		add(id)
	}
	for _, ov := range overrides {
		id, ok := r.ResolveID(ov.ID)
		if !ok {
			return nil, fmt.Errorf("unknown metric %q", ov.ID)
		}
		if ov.Add {
			add(id)
		} else {
			remove(id)
		}
	}
	return order, nil
}
