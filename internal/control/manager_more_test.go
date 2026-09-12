package control

import (
	"testing"

	"github.com/netikras/procfit/internal/ports"
)

func TestOps_TargetNotFound(t *testing.T) {
	m, _, _ := newMgr(t)
	if _, err := m.SetNice("nope", 1, false); err == nil {
		t.Error("SetNice on missing target should error")
	}
	if _, err := m.Restore("nope", false); err == nil {
		t.Error("Restore on missing target should error")
	}
	if _, err := m.SetStop("nope", true); err == nil {
		t.Error("SetStop on missing target should error")
	}
	if err := m.Unmanage("nope"); err == nil {
		t.Error("Unmanage on missing target should error")
	}
	if err := m.Rebind("nope", nil); err == nil {
		t.Error("Rebind on missing target should error")
	}
}

func TestSignal_ProtectedAndStale(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(2, 0, idFor(2, 20))
	fc.Add(50, 0, idFor(50, 500))
	// pid 1 protected; pid 50 stale (identity changed); pid 2 ok.
	fc.Add(1, 0, idFor(1, 1))
	fc.Identity[50] = idFor(50, 999) // reused
	out := m.Signal([]Instance{
		{ID: idFor(1, 1), PID: 1},
		{ID: idFor(50, 500), PID: 50},
		{ID: idFor(2, 20), PID: 2},
	}, ports.SigHUP)
	if out.Counts[StatusSkipped] != 1 || out.Counts[StatusStale] != 1 || out.Counts[StatusApplied] != 1 {
		t.Fatalf("signal outcome wrong: %+v", out.Counts)
	}
}

func TestSetNice_DryRunDoesNotMutate(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(7, 0, idFor(7, 70))
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(7, 70), PID: 7}})
	res, _ := m.SetNice("t", 10, true)
	if res.Counts[StatusUnchanged] != 1 {
		t.Fatalf("dry-run should be unchanged, got %+v", res.Counts)
	}
	if fc.Nice[7] != 0 {
		t.Fatal("dry-run must not mutate")
	}
}

func TestObserveNice_Vanish(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(7, 0, idFor(7, 70))
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(7, 70), PID: 7}})
	// Capture original, then make GetNice fail during observe by removing nice
	// but keeping identity so revalidate passes and SetNice succeeds, then remove.
	// Simpler: apply once (works), then remove and restore to hit vanish path.
	_, _ = m.SetNice("t", 5, false)
	fc.Remove(7)
	res, _ := m.Restore("t", false)
	if res.Counts[StatusStale] == 0 && res.Counts[StatusVanished] == 0 {
		t.Fatalf("restore on vanished pid should be stale/vanished, got %+v", res.Counts)
	}
}
