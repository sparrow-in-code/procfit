package model

import (
	"path"
	"strings"
)

// displayNameCap bounds a derived display name so a pathological argv[0] cannot
// blow up column widths.
const displayNameCap = 64

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
	// Only treat it as a path when it contains a separator and no spaces (some
	// daemons set argv[0] to a spaced title like "sshd: user [priv]").
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
