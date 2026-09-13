package tui

import (
	"fmt"

	"github.com/netikras/procfit/internal/meta"
)

// Panel selects which view the explorer shows.
type Panel int

const (
	PanelBrowser Panel = iota // the process/group tree
	PanelManaged              // the managed-targets side plane
)

// ManagedRow is one managed target as shown in the managed panel.
type ManagedRow struct {
	Name     string
	Mode     string
	Active   bool
	Members  int
	Nice     string // "" when nice is not controlled
	Stopped  bool
	Frozen   bool
	LastSeen string
}

// ManagedFunc returns the current managed targets (queried fresh on demand).
type ManagedFunc func() []ManagedRow

// ManagedActionKind is a managed-panel action on the selected target.
type ManagedActionKind int

const (
	ManagedRestore ManagedActionKind = iota
	ManagedUnmanage
)

// ManagedActionFunc restores or unmanages a target by name, returning a summary.
type ManagedActionFunc func(name string, kind ManagedActionKind) (string, error)

// togglePanel switches between the browser and the managed panel, refreshing the
// managed list on entry. No-op when the managed backend is absent.
func (m *Model) togglePanel() {
	if m.managedFn == nil {
		m.status = "managed panel unavailable"
		return
	}
	if m.panel == PanelBrowser {
		m.panel = PanelManaged
		m.mCursor = 0
		m.refreshManaged()
		m.status = "managed targets"
	} else {
		m.panel = PanelBrowser
		m.status = ""
	}
}

func (m *Model) refreshManaged() {
	if m.managedFn == nil {
		return
	}
	m.managed = m.managedFn()
	if m.mCursor >= len(m.managed) {
		m.mCursor = maxInt(0, len(m.managed)-1)
	}
}

func (m *Model) managedKey(ev KeyEvent) {
	switch {
	case ev.Name == "up" || ev.Rune == 'k':
		m.mCursor = maxInt(0, m.mCursor-1)
	case ev.Name == "down" || ev.Rune == 'j':
		m.mCursor = minInt(maxInt(0, len(m.managed)-1), m.mCursor+1)
	case ev.Rune == 'R':
		m.managedDo(ManagedRestore)
	case ev.Rune == 'd':
		m.managedDo(ManagedUnmanage)
	case ev.Rune == 'r':
		m.refreshManaged()
		m.status = "refreshed"
	case ev.Rune == 'q', ev.Name == "ctrl-c":
		m.quit = true
	}
}

func (m *Model) managedDo(kind ManagedActionKind) {
	if m.managedAct == nil || m.mCursor < 0 || m.mCursor >= len(m.managed) {
		return
	}
	name := m.managed[m.mCursor].Name
	res, err := m.managedAct(name, kind)
	if err != nil {
		m.status = "managed: " + err.Error()
	} else {
		m.status = res
	}
	m.refreshManaged()
}

// managedFrame renders the managed panel: a status bar, header, then targets.
func (m *Model) managedFrame() []string {
	lines := []string{m.managedStatus(), m.managedHeader()}
	body := m.bodyHeight()
	for i := 0; i < body; i++ {
		if i >= len(m.managed) {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, m.managedLine(i))
	}
	return lines
}

func (m *Model) managedStatus() string {
	return truncate(fmt.Sprintf("%s  MANAGED (%d)  [↑↓]move [R]estore [d]rop [r]efresh [Tab]browser [q]uit  %s",
		meta.Name, len(m.managed), m.status), m.width)
}

func (m *Model) managedHeader() string {
	return truncate("  TARGET                MODE      STATE     P     NICE  STOP  FRZ  LAST-SEEN", m.width)
}

func (m *Model) managedLine(i int) string {
	t := m.managed[i]
	cursor := "  "
	if i == m.mCursor {
		cursor = "> "
	}
	state := "INACTIVE"
	if t.Active {
		state = "ACTIVE"
	}
	nice := t.Nice
	if nice == "" {
		nice = "-"
	}
	return truncate(fmt.Sprintf("%s%-20s %-9s %-9s %-5d %-5s %-5s %-4s %s",
		cursor, t.Name, t.Mode, state, t.Members, nice, yesNo(t.Stopped), yesNo(t.Frozen), t.LastSeen), m.width)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "-"
}
