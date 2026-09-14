package tui

import (
	"strings"
	"testing"

	"github.com/netikras/procfit/internal/queryspec"
)

func TestModel_SlimStatusBar(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(120, 24)
	res, cols := sampleResult(3)
	m.SetResult(res, cols)

	bar := m.statusBar()
	// The steady-state bar carries only the essentials + pointers to help/quit.
	for _, want := range []string{"procfit", "gen=", "rows=", "interval=", "[↑↓ enter]nav", "[?]help", "[q]uit"} {
		if !strings.Contains(bar, want) {
			t.Fatalf("slim bar missing %q: %q", want, bar)
		}
	}
	// The full keymap must NOT be inlined anymore (it lives in `?` help).
	for _, gone := range []string{"[g]roup", "[t]leaf", "[i]nspect", "[s]ort", "fold-all"} {
		if strings.Contains(bar, gone) {
			t.Fatalf("slim bar should not inline %q: %q", gone, bar)
		}
	}
}

func TestModel_HelpOverlay(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(100, 40)
	res, cols := sampleResult(3)
	m.SetResult(res, cols)

	// `?` opens the overlay; it lists nav + view keys and how to close.
	m.Update(KeyEvent{Rune: '?'})
	if !m.help {
		t.Fatal("`?` should open the help overlay")
	}
	joined := strings.Join(m.Frame(), "\n")
	for _, want := range []string{"HELP", "NAVIGATION", "cycle the grouping preset", "fold all groups", "quit"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("help overlay missing %q:\n%s", want, joined)
		}
	}
	// `?` again toggles it shut; q/Esc also close.
	m.Update(KeyEvent{Rune: '?'})
	if m.help {
		t.Fatal("`?` should toggle the help overlay closed")
	}
	m.Update(KeyEvent{Rune: '?'})
	m.Update(KeyEvent{Name: "esc"})
	if m.help {
		t.Fatal("Esc should close the help overlay")
	}
}

func TestModel_HelpAdaptsToAvailability(t *testing.T) {
	// Without a controller or managed source, control/panel sections are absent.
	m := NewModel(queryspec.Flags{})
	m.SetSize(100, 40)
	res, cols := sampleResult(1)
	m.SetResult(res, cols)
	m.Update(KeyEvent{Rune: '?'})
	if j := strings.Join(m.Frame(), "\n"); strings.Contains(j, "CONTROL") || strings.Contains(j, "PANELS") {
		t.Fatalf("help should hide control/panel sections when unavailable:\n%s", j)
	}

	// With both wired, the control + panel sections appear.
	m2 := NewModel(queryspec.Flags{})
	m2.SetSize(100, 40)
	m2.SetResult(res, cols)
	m2.SetControl(func(ControlRequest) (string, error) { return "", nil })
	m2.SetManaged(true, func([]int) (string, error) { return "", nil })
	m2.Update(KeyEvent{Rune: '?'})
	j := strings.Join(m2.Frame(), "\n")
	if !strings.Contains(j, "CONTROL") || !strings.Contains(j, "SIGSTOP") || !strings.Contains(j, "PANELS") {
		t.Fatalf("help should show control + panel sections when available:\n%s", j)
	}
}

func TestModel_HelpScrollsOnShortTerminal(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 8) // deliberately short: the keymap won't fit
	res, cols := sampleResult(1)
	m.SetResult(res, cols)
	m.SetControl(func(ControlRequest) (string, error) { return "", nil })
	m.Update(KeyEvent{Rune: '?'})

	top := strings.Join(m.Frame(), "\n")
	if !strings.Contains(top, "scroll") {
		t.Fatalf("short terminal help should advertise scrolling:\n%s", top)
	}
	// Paging down reveals later sections not visible at the top.
	m.Update(KeyEvent{Name: "pgdn"})
	if m.helpScroll == 0 {
		t.Fatal("PgDn should advance the help scroll")
	}
	// Scroll is clamped: repeated PgDn cannot run past the end.
	for i := 0; i < 20; i++ {
		m.Update(KeyEvent{Name: "pgdn"})
	}
	if maxTop := len(m.helpLines()) - m.bodyHeight(); m.helpScroll != maxTop {
		t.Fatalf("help scroll should clamp to %d, got %d", maxTop, m.helpScroll)
	}
}
