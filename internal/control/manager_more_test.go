package control

import (
	"testing"

	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/testutil"
)

// sigCount totals every signal the fake controller has delivered, so a dry-run
// test can assert zero deliveries regardless of pid.
func sigCount(fc *testutil.FakeController) int {
	n := 0
	for _, s := range fc.Signals {
		n += len(s)
	}
	return n
}

// TestControl_DryRunNeverMutates is the PM-0510 guard: every mutating control op
// (stop/continue, signal, restore) must be a no-op under dry-run — no signal
// delivered, no nice changed, no audit event, no intent recorded (RFC §20).
func TestControl_DryRunNeverMutates(t *testing.T) {
	newT := func(t *testing.T) (*Manager, *testutil.FakeController, *capRecorder) {
		t.Helper()
		m, fc, _ := newMgr(t)
		rec := &capRecorder{}
		m.SetRecorder(rec)
		fc.Add(7, 3, idFor(7, 70)) // nice starts at 3
		m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(7, 70), PID: 7}})
		return m, fc, rec
	}
	assertPreview := func(t *testing.T, res *ApplyResult, fc *testutil.FakeController, rec *capRecorder) {
		t.Helper()
		if res.Counts[StatusUnchanged] != 1 {
			t.Fatalf("dry-run should report unchanged, got %+v", res.Counts)
		}
		if got := sigCount(fc); got != 0 {
			t.Fatalf("dry-run delivered %d signal(s); must deliver none", got)
		}
		if fc.Nice[7] != 3 {
			t.Fatalf("dry-run mutated nice to %d; must not touch process", fc.Nice[7])
		}
		if len(rec.events) != 0 {
			t.Fatalf("dry-run recorded audit events: %+v", rec.events)
		}
	}

	t.Run("stop", func(t *testing.T) {
		m, fc, rec := newT(t)
		res, _ := m.SetStop("t", true, true)
		assertPreview(t, res, fc, rec)
		if m.Find("t").DesiredStop != nil {
			t.Fatal("dry-run must not record stop intent on the target")
		}
	})
	t.Run("signal", func(t *testing.T) {
		m, fc, rec := newT(t)
		res := m.Signal([]Instance{{ID: idFor(7, 70), PID: 7}}, ports.SigTerm, true)
		assertPreview(t, res, fc, rec)
	})
	t.Run("restore", func(t *testing.T) {
		m, fc, rec := newT(t)
		// Apply a real nice + stop first (so restore has something to undo)...
		if _, err := m.SetNice("t", 15, false); err != nil {
			t.Fatal(err)
		}
		if _, err := m.SetStop("t", true, false); err != nil {
			t.Fatal(err)
		}
		// ...then reset audit + baseline and assert the dry-run restore is inert.
		rec.events = nil
		fc.Signals = map[int][]ports.Signal{}
		nice := fc.Nice[7]
		res, _ := m.Restore("t", false, true)
		if got := sigCount(fc); got != 0 {
			t.Fatalf("dry-run restore delivered %d signal(s)", got)
		}
		if fc.Nice[7] != nice {
			t.Fatalf("dry-run restore changed nice %d→%d", nice, fc.Nice[7])
		}
		if len(rec.events) != 0 {
			t.Fatalf("dry-run restore recorded audit events: %+v", rec.events)
		}
		if res.Counts[StatusRestored] != 0 {
			t.Fatalf("dry-run restore should not report restored, got %+v", res.Counts)
		}
		// Intent must survive a preview so the real restore still has work to do.
		tg := m.Find("t")
		if tg.DesiredNice == nil || tg.DesiredStop == nil {
			t.Fatal("dry-run restore must not clear the target's control intent")
		}
	})
}

func TestOps_TargetNotFound(t *testing.T) {
	m, _, _ := newMgr(t)
	if _, err := m.SetNice("nope", 1, false); err == nil {
		t.Error("SetNice on missing target should error")
	}
	if _, err := m.Restore("nope", false, false); err == nil {
		t.Error("Restore on missing target should error")
	}
	if _, err := m.SetStop("nope", true, false); err == nil {
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
	}, ports.SigHUP, false)
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
	res, _ := m.Restore("t", false, false)
	if res.Counts[StatusStale] == 0 && res.Counts[StatusVanished] == 0 {
		t.Fatalf("restore on vanished pid should be stale/vanished, got %+v", res.Counts)
	}
}
