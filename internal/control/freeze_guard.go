package control

import (
	"path"
	"strings"
)

// FreezeSafety decides whether freezing the cgroup cg is safe, given selfCg (the
// cgroup procfit itself runs in). cgroup freeze pauses the whole cgroup subtree,
// so freezing an ancestor of our own cgroup — or a login-session/user slice —
// would freeze the caller's session (a self-inflicted lockout). Both paths are
// v2 relative paths ("/user.slice/..."). Thaw is always safe and must not be
// gated by this (recovery). Callers may override with --force.
func FreezeSafety(cg, selfCg string) (allowed bool, reason string) {
	if cg == "" || cg == "/" {
		return false, "refusing to freeze the root cgroup"
	}
	if selfCg != "" && isAncestorOrEqual(cg, selfCg) {
		return false, "would freeze procfit's own session (cgroup " + cg + ")"
	}
	if isSessionCgroup(cg) {
		return false, "targets a login session / user slice (cgroup " + cg + ")"
	}
	return true, ""
}

// isAncestorOrEqual reports whether anc is desc or an ancestor of desc.
func isAncestorOrEqual(anc, desc string) bool {
	anc = "/" + strings.Trim(anc, "/")
	desc = "/" + strings.Trim(desc, "/")
	return desc == anc || strings.HasPrefix(desc, anc+"/")
}

// isSessionCgroup reports whether cg names a login session or user slice/manager,
// i.e. a scope whose freeze would take down a whole login.
func isSessionCgroup(cg string) bool {
	base := path.Base("/" + strings.Trim(cg, "/"))
	switch {
	case strings.HasPrefix(base, "session-") && strings.HasSuffix(base, ".scope"):
		return true
	case strings.HasPrefix(base, "user@") && strings.HasSuffix(base, ".service"):
		return true
	case strings.HasPrefix(base, "user-") && strings.HasSuffix(base, ".slice"):
		return true
	default:
		return false
	}
}
