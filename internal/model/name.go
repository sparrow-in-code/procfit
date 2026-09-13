package model

import (
	"path"
	"strings"
)

// displayNameCap is a generous safety bound so a pathological (e.g. multi-KB)
// argv[0] cannot blow up memory or column widths. The on-screen TARGET width is
// controlled separately by --target-width (and the terminal width in the TUI),
// so this only guards against the truly degenerate case.
const displayNameCap = 256

// DisplayName returns a fuller, human-friendly name for a process, working
// around the kernel's 15-character `comm` truncation. It prefers argv[0]
// (basename when it looks like a path), then the exe basename, then comm.
func (p *Process) DisplayName() string {
	if len(p.Cmdline) > 0 && p.Cmdline[0] != "" {
		return cap64(baseName(p.Cmdline[0]))
	}
	if p.Exe != "" {
		return cap64(baseName(p.Exe))
	}
	return p.Comm
}

func baseName(s string) string {
	// An absolute path may carry trailing args in the same string: Chrome (and a
	// few others) rewrite /proc/PID/cmdline into one space-joined string, so
	// argv[0] becomes "/nix/.../chrome --type=renderer ...". Take the executable
	// token, then its basename, so the name is "chrome" (and all chrome processes
	// group together) rather than the whole differing command line.
	if strings.HasPrefix(s, "/") {
		first := s
		if i := strings.IndexByte(s, ' '); i >= 0 {
			first = s[:i]
		}
		return path.Base(first)
	}
	// A relative path with no spaces is still a path; a spaced title with no
	// leading separator (e.g. "sshd: user [priv]") is kept verbatim.
	if strings.Contains(s, "/") && !strings.Contains(s, " ") {
		return path.Base(s)
	}
	return s
}

func cap64(s string) string {
	if len(s) > displayNameCap {
		return s[:displayNameCap]
	}
	return s
}
