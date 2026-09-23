package tui

import (
	"github.com/netikras/procfit/internal/meta"
)

// openHelp opens the full-screen help overlay (the `?` key). The status bar only
// carries the essentials, so the complete keymap lives here (vim/tmux style).
func (m *Model) openHelp() {
	m.help = true
	m.helpScroll = 0
	m.status = ""
}

// helpKey handles input while the help overlay is open: it closes on ?/Esc/q and
// scrolls otherwise, so the keymap stays readable on a short terminal.
func (m *Model) helpKey(ev KeyEvent) {
	switch {
	case ev.Rune == '?', ev.Rune == 'q', ev.Name == "esc", ev.Name == "ctrl-c":
		m.help = false
	case ev.Name == "down", ev.Rune == 'j':
		m.helpScroll++
	case ev.Name == "up", ev.Rune == 'k':
		m.helpScroll--
	case ev.Name == "pgdn":
		m.helpScroll += m.bodyHeight()
	case ev.Name == "pgup":
		m.helpScroll -= m.bodyHeight()
	}
	m.clampHelpScroll()
}

// helpEntry is one keymap row; a blank Keys with a Desc is a section heading, and
// a fully blank entry is a spacer.
type helpEntry struct{ keys, desc string }

// helpEntries returns the keymap, tailored to what is actually available (control
// keys only when a controller is wired; panel keys only when a managed source is).
func (m *Model) helpEntries() []helpEntry {
	e := []helpEntry{
		{"", "NAVIGATION"},
		{"↑/↓  k/j", "move the cursor"},
		{"PgUp/PgDn", "page up / down"},
		{"Enter", "fold / unfold the group under the cursor"},
		{"← / →", "collapse-or-parent / expand-or-child"},
		{"c / C", "fold all groups / unfold all groups"},
		{"", ""},
		{"", "VIEW"},
		{"g", "cycle grouping — presets + any displayed dimension column (wchan, state, user, …)"},
		{"t", "cycle the leaf mode (process/thread/none)"},
		{"s / S", "cycle sort column / toggle asc↔desc"},
		{"/  or  f", "filter — one word=target substring; +space=column filter w/ operator autosuggest (Tab/Enter autofill)"},
		{"+", "column picker — type to filter, Enter toggles a column on/off, Esc closes"},
		{"-", "quick-drop the rightmost column"},
		{"u", "toggle human units (K/M/G) vs raw"},
		{"i", "inspect the selected row (identity + history)"},
		{"[ / ]", "faster / slower refresh interval"},
		{"p / Space", "pause / resume auto-refresh"},
		{"r", "refresh once"},
	}
	if m.control != nil {
		e = append(e,
			helpEntry{"", ""},
			helpEntry{"", "CONTROL  (lowercase applies, UPPERCASE lifts)"},
			helpEntry{"n / N", "renice / restore original nice"},
			helpEntry{"x / X", "stop (SIGSTOP) / continue (SIGCONT)"},
			helpEntry{"z / Z", "freeze / thaw the whole cgroup subtree"},
			helpEntry{"", "each previews first — y confirms, v = full forecast"},
		)
	}
	if m.hasMgr {
		e = append(e,
			helpEntry{"", ""},
			helpEntry{"", "PANELS"},
			helpEntry{"Tab", "switch between the browser and managed panel"},
			helpEntry{"d", "managed panel: drop a target (stop tracking)"},
		)
	}
	return append(e,
		helpEntry{"", ""},
		helpEntry{"", "GENERAL"},
		helpEntry{"?", "toggle this help"},
		helpEntry{"q / Ctrl-C", "quit (prints the equivalent procfit ps command)"},
	)
}

// helpLines renders the keymap to display lines.
func (m *Model) helpLines() []string {
	entries := m.helpEntries()
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		switch {
		case e.keys == "" && e.desc == "":
			out = append(out, "")
		case e.keys == "": // section heading (or an indented note)
			out = append(out, "  "+e.desc)
		default:
			out = append(out, truncate("    "+pad(e.keys, 12, false)+"  "+e.desc, m.width))
		}
	}
	return out
}

// clampHelpScroll keeps the scroll offset within the rendered content so paging
// on a short terminal cannot scroll past the ends.
func (m *Model) clampHelpScroll() {
	maxTop := len(m.helpLines()) - m.bodyHeight()
	if m.helpScroll > maxTop {
		m.helpScroll = maxTop
	}
	if m.helpScroll < 0 {
		m.helpScroll = 0
	}
}

// helpFrame renders the help overlay: a title row, then the (scrolled) keymap.
func (m *Model) helpFrame() []string {
	title := meta.Name + "  HELP — ?/Esc/q to close"
	if len(m.helpLines()) > m.bodyHeight() {
		title += "   (↑/↓ scroll)"
	}
	lines := []string{truncate(title, m.width)}
	content := m.helpLines()
	for i := 0; i < m.bodyHeight(); i++ {
		idx := m.helpScroll + i
		if idx < len(content) {
			lines = append(lines, content[idx])
		} else {
			lines = append(lines, "")
		}
	}
	return lines
}
