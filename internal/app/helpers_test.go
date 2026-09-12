package app

import (
	"testing"

	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/ports"
)

func TestExitForResult(t *testing.T) {
	mk := func(m map[control.FieldStatus]int) *control.ApplyResult {
		r := &control.ApplyResult{Counts: m}
		for s, n := range m {
			for i := 0; i < n; i++ {
				r.Results = append(r.Results, control.BindingResult{Status: s})
			}
		}
		return r
	}
	cases := []struct {
		name string
		in   *control.ApplyResult
		want int
	}{
		{"nil", nil, ExitOK},
		{"empty", &control.ApplyResult{Counts: map[control.FieldStatus]int{}}, ExitOK},
		{"all applied", mk(map[control.FieldStatus]int{control.StatusApplied: 2}), ExitOK},
		{"all drifted", mk(map[control.FieldStatus]int{control.StatusDrifted: 1}), ExitConflict},
		{"partial drift", mk(map[control.FieldStatus]int{control.StatusApplied: 1, control.StatusDrifted: 1}), ExitPartial},
		{"all denied", mk(map[control.FieldStatus]int{control.StatusDenied: 1}), ExitPermission},
		{"partial denied", mk(map[control.FieldStatus]int{control.StatusApplied: 1, control.StatusDenied: 1}), ExitPartial},
		{"failed", mk(map[control.FieldStatus]int{control.StatusFailed: 1}), ExitPartial},
	}
	for _, c := range cases {
		if got := exitForResult(c.in); got != c.want {
			t.Errorf("%s: exitForResult = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestSmallHelpers(t *testing.T) {
	if maxExit(1, 5) != 5 || maxExit(7, 2) != 7 {
		t.Error("maxExit wrong")
	}
	if defaultMode("pid:1") != control.ModeSnapshot {
		t.Error("pid: should default to snapshot")
	}
	if defaultMode("managed:x") != control.ModeFollow {
		t.Error("managed: should default to follow")
	}
	if targetName("managed:foo") != "foo" || targetName("pid:9") != "pid:9" {
		t.Error("targetName wrong")
	}
	if selectorOf("selector:cpu>1") != "cpu>1" || selectorOf("pid:1") != "" {
		t.Error("selectorOf wrong")
	}
	if !destructive(ports.SigTerm) || !destructive(ports.SigKill) || destructive(ports.SigHUP) {
		t.Error("destructive wrong")
	}
}

func TestResolveStateDir(t *testing.T) {
	if dir, eph := resolveStateDir("/explicit"); dir != "/explicit" || eph {
		t.Error("explicit dir should win and not be ephemeral")
	}
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	if dir, eph := resolveStateDir(""); dir != "/run/user/1000/procfit" || eph {
		t.Errorf("xdg dir wrong: %q eph=%v", dir, eph)
	}
	t.Setenv("XDG_RUNTIME_DIR", "")
	if _, eph := resolveStateDir(""); !eph {
		t.Error("no XDG_RUNTIME_DIR should be ephemeral")
	}
}

func TestParseOverrides(t *testing.T) {
	got, _ := parseOverrides([]string{"cpu", "+rss", "-vsz", ""})
	if len(got) != 3 {
		t.Fatalf("want 3 overrides, got %d", len(got))
	}
	if !got[0].Add || got[0].ID != "cpu" || !got[1].Add || got[2].Add {
		t.Fatalf("override parse wrong: %+v", got)
	}
}
