// Package query is the renderer-independent observation engine: it selects
// entities, groups them by an ordered list of dimensions, aggregates metrics,
// applies optional leaves and a having filter, sorts, and projects columns (RFC
// §6.1, §12). No renderer may reimplement any of this.
package query

import (
	"fmt"
	"strconv"

	"github.com/netikras/procfit/internal/model"
)

// Dimension is one grouping axis. It extracts a stable key and a display label
// from a process. Availability lets grouping bucket "unknown" distinctly rather
// than as a fabricated value.
type Dimension struct {
	ID    string
	Label string
	// Desc is a one-line description for the field catalog (`procfit metrics`).
	Desc string
	// extract returns (stableKey, displayLabel, available).
	extract func(p *model.Process) (string, string, bool)
}

// Key returns the grouping key for a process under this dimension.
func (d Dimension) Key(p *model.Process) (key, label string, ok bool) {
	return d.extract(p)
}

// desc sets a dimension's catalog description (chainable at registration).
func (d Dimension) desc(s string) Dimension { d.Desc = s; return d }

// Dimensions is a registry of grouping dimensions.
type Dimensions struct {
	byID  map[string]Dimension
	order []string
}

// Get returns a dimension by id.
func (d *Dimensions) Get(id string) (Dimension, bool) {
	dim, ok := d.byID[id]
	return dim, ok
}

// Has reports whether a dimension id is known.
func (d *Dimensions) Has(id string) bool {
	_, ok := d.byID[id]
	return ok
}

// IDs returns the registered dimension ids in registration order (the single
// source of truth for help text, so it never drifts from what actually resolves).
func (d *Dimensions) IDs() []string {
	out := make([]string, len(d.order))
	copy(out, d.order)
	return out
}

func nsDim(id, label string, t model.NamespaceType) Dimension {
	return Dimension{ID: id, Label: label, extract: func(p *model.Process) (string, string, bool) {
		ino, ok := p.Namespaces.Get(t)
		if !ok {
			return "", "", false
		}
		return strconv.FormatUint(ino, 10), fmt.Sprintf("%s:%d", t, ino), true
	}}
}

func strField(id, label string, get func(*model.Process) string) Dimension {
	return Dimension{ID: id, Label: label, extract: func(p *model.Process) (string, string, bool) {
		v := get(p)
		if v == "" {
			return "", "", false
		}
		return v, v, true
	}}
}

func intField(id, label string, get func(*model.Process) int) Dimension {
	return Dimension{ID: id, Label: label, extract: func(p *model.Process) (string, string, bool) {
		v := get(p)
		s := strconv.Itoa(v)
		return s, s, true
	}}
}

// NewDimensions builds the default dimension registry (subset of RFC §12.1;
// process/thread are leaf modes, not group dimensions).
func NewDimensions() *Dimensions {
	dims := []Dimension{
		Dimension{ID: "host", Label: "HOST", extract: func(*model.Process) (string, string, bool) { return "host", "HOST", true }}.desc("the whole host (a single group)"),
		// Fields shared with a display column inherit that column's description in
		// the catalog; dimension-only fields describe themselves here.
		strField("comm", "COMM", func(p *model.Process) string { return p.Comm }),
		strField("name", "NAME", func(p *model.Process) string { return p.DisplayName() }),
		strField("user", "USER", func(p *model.Process) string { return p.User }),
		strField("exe", "EXE", func(p *model.Process) string { return p.Exe }).desc("executable path"),
		strField("app", "APP", func(p *model.Process) string { return p.AppID }).desc("application id (desktop/systemd app)"),
		strField("cgroup", "CGROUP", func(p *model.Process) string { return p.CgroupPath }).desc("control-group path (cgroup v2)"),
		// Grouping by state (R/S/D/…) buckets the load contributors; by wchan
		// clusters every task blocked in the same kernel function — instant root
		// cause for D-state stalls (jbd2/NFS/reclaim/…).
		strField("pstate", "PSTATE", func(p *model.Process) string { return string(p.State.Code) }),
		strField("wchan", "WCHAN", func(p *model.Process) string { return p.Wchan }),
		strField("systemd-unit", "UNIT", func(p *model.Process) string { return p.SystemdUnit }).desc("systemd unit"),
		strField("container", "CONTAINER", func(p *model.Process) string { return p.ContainerID }).desc("container id"),
		strField("pod", "POD", func(p *model.Process) string { return p.PodUID }).desc("pod uid"),
		intField("pid", "PID", func(p *model.Process) int { return p.PID }),
		intField("ppid", "PPID", func(p *model.Process) int { return p.PPID }),
		intField("session", "SID", func(p *model.Process) int { return p.SID }).desc("session id (SID)"),
		intField("process-group", "PGID", func(p *model.Process) int { return p.PGID }).desc("process-group id (PGID)"),
		{ID: "uid", Label: "UID", extract: func(p *model.Process) (string, string, bool) {
			s := strconv.FormatUint(uint64(p.UID), 10)
			return s, s, true
		}},
		nsDim("pidns", "PIDNS", model.NSPID).desc("PID namespace inode"),
		nsDim("netns", "NETNS", model.NSNet).desc("network namespace inode"),
		nsDim("mntns", "MNTNS", model.NSMount).desc("mount namespace inode"),
		nsDim("userns", "USERNS", model.NSUser).desc("user namespace inode"),
		nsDim("cgroupns", "CGROUPNS", model.NSCgroup).desc("cgroup namespace inode"),
		Dimension{ID: "namespace-set", Label: "NS-SET", extract: func(p *model.Process) (string, string, bool) {
			k := p.Namespaces.Key()
			if k == "" {
				return "", "", false
			}
			return k, k, true
		}}.desc("combined namespace membership key"),
	}
	reg := &Dimensions{byID: make(map[string]Dimension, len(dims)), order: make([]string, 0, len(dims))}
	for _, d := range dims {
		reg.byID[d.ID] = d
		reg.order = append(reg.order, d.ID)
	}
	return reg
}
