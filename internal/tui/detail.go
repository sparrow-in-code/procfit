package tui

import (
	"fmt"
	"strings"

	"github.com/netikras/procfit/internal/meta"
	"github.com/netikras/procfit/internal/query"
)

// openDetail opens the inspect overlay for the selected row.
func (m *Model) openDetail() {
	if _, ok := m.currentRow(); !ok {
		return
	}
	m.detail = true
	m.status = ""
}

func (m *Model) detailKey(ev KeyEvent) {
	switch {
	case ev.Rune == 'i', ev.Rune == 'q', ev.Name == "esc", ev.Name == "ctrl-c":
		m.detail = false
	}
}

// detailFrame renders the inspect overlay for the selected row: full identity for
// a process/thread, or a summary for a group.
func (m *Model) detailFrame() []string {
	lines := []string{truncate(meta.Name+"  DETAIL — Esc/i/q to close", m.width)}
	fr, ok := m.currentRow()
	if ok {
		lines = append(lines, m.detailBody(fr.row)...)
	}
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	return lines
}

func (m *Model) detailBody(r *query.Row) []string {
	switch {
	case r.Process != nil:
		p := r.Process
		return []string{
			m.kv("name", p.DisplayName()),
			m.kv("comm", p.Comm),
			m.kv("pid", fmt.Sprintf("%d   ppid %d", p.PID, p.PPID)),
			m.kv("user", fmt.Sprintf("%s (uid %d)", p.User, p.UID)),
			m.kv("state", string(p.State.Code)),
			m.kv("exe", p.Exe),
			m.kv("cgroup", p.CgroupPath),
			m.kv("cmdline", strings.Join(p.Cmdline, " ")),
		}
	case r.Thread != nil:
		t := r.Thread
		return []string{
			m.kv("thread", t.Comm),
			m.kv("tid", fmt.Sprintf("%d   (owner pid %d)", t.ID.TID, t.ID.Process.PID)),
			m.kv("state", string(t.State.Code)),
		}
	default:
		return []string{
			m.kv("group", r.Label),
			m.kv("procs/threads", fmt.Sprintf("%d / %d", r.Procs, r.Threads)),
			m.kv("children", fmt.Sprintf("%d", r.Children)),
		}
	}
}

func (m *Model) kv(k, v string) string {
	return truncate(fmt.Sprintf("  %-9s %s", k+":", v), m.width)
}
