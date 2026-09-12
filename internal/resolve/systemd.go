package resolve

import (
	"strings"

	"github.com/netikras/procfit/internal/model"
)

// SystemdResolver derives a systemd unit name from a cgroup v2 path heuristic
// (RFC §14.2). It only fills SystemdUnit when empty and leaves the cgroup path
// (the native identity) unchanged.
type SystemdResolver struct{}

// NewSystemdResolver builds the resolver.
func NewSystemdResolver() *SystemdResolver { return &SystemdResolver{} }

// ID identifies the resolver.
func (r *SystemdResolver) ID() string { return "systemd" }

// Resolve sets p.SystemdUnit from the cgroup path if a unit is discernible.
func (r *SystemdResolver) Resolve(p *model.Process) {
	if p.SystemdUnit != "" || p.CgroupPath == "" {
		return
	}
	if unit := unitFromCgroup(p.CgroupPath); unit != "" {
		p.SystemdUnit = unit
	}
}

// unitFromCgroup returns the last path component that names a systemd unit
// (…/foo.service, …/foo.scope, …/foo.slice). Pure and testable.
func unitFromCgroup(path string) string {
	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		seg := parts[i]
		if strings.HasSuffix(seg, ".service") || strings.HasSuffix(seg, ".scope") {
			return seg
		}
	}
	// Fall back to a trailing .slice if no service/scope is present.
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.HasSuffix(parts[i], ".slice") {
			return parts[i]
		}
	}
	return ""
}
