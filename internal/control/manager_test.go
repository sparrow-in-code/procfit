package control

import (
	"testing"
	"time"

	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/testutil"
)

func idFor(pid int, start uint64) model.ProcessInstanceID {
	return model.ProcessInstanceID{BootID: "boot", PID: pid, StartTime: start}
}

func newMgr(t *testing.T) (*Manager, *testutil.FakeController, *testutil.FakeClock) {
	t.Helper()
	fc := testutil.NewFakeController()
	clk := testutil.NewFakeClock(time.Unix(1000, 0))
	m := NewManager(fc, clk, NewSafeguards(99999, 88888), "boot")
	return m, fc, clk
}

func TestManage_MembershipOnly_NoControl(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(1234, 0, idFor(1234, 100))
	tgt := m.Manage("Chrome", ModeFollow, `comm == "chrome"`, []Instance{{ID: idFor(1234, 100), PID: 1234}})
	if !tgt.Active || len(tgt.Bindings) != 1 {
		t.Fatal("target should be active with one binding")
	}
	if tgt.Bindings[0].Nice != nil {
		t.Fatal("membership must not apply control (RFC §6.6)")
	}
	if fc.Nice[1234] != 0 {
		t.Fatal("nice must be unchanged by managing")
	}
}

func TestSetNice_CapturesOriginalOnce_AndObserves(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(1234, 0, idFor(1234, 100))
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(1234, 100), PID: 1234}})

	res, err := m.SetNice("t", 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts[StatusApplied] != 1 {
		t.Fatalf("expected applied, got %+v", res.Counts)
	}
	b := m.Find("t").Bindings[0]
	if b.Nice.Original != 0 || b.Nice.Desired != 10 || b.Nice.Observed != 10 {
		t.Fatalf("nice field wrong: %+v", b.Nice)
	}
	if fc.Nice[1234] != 10 {
		t.Fatal("controller nice not set")
	}
	// Change desired again; original must NOT be overwritten (RFC §15.2).
	_, _ = m.SetNice("t", 15, false)
	b = m.Find("t").Bindings[0]
	if b.Nice.Original != 0 {
		t.Fatalf("original overwritten: %d", b.Nice.Original)
	}
	if b.Nice.Desired != 15 || fc.Nice[1234] != 15 {
		t.Fatal("desired not updated")
	}
}

func TestSetNice_StalePIDRefused(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(1234, 0, idFor(1234, 100))
	m.Manage("t", ModeSnapshot, "", []Instance{{ID: idFor(1234, 100), PID: 1234}})
	// PID reused: same pid, different starttime.
	fc.Identity[1234] = idFor(1234, 999)
	res, _ := m.SetNice("t", 10, false)
	if res.Counts[StatusStale] != 1 {
		t.Fatalf("stale pid must be refused, got %+v", res.Counts)
	}
	if fc.Nice[1234] != 0 {
		t.Fatal("must not mutate a reused pid (RFC §6.3/§21.2)")
	}
}

func TestExternalDrift_DetectedAndRestoreRefuses(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(1234, 0, idFor(1234, 100))
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(1234, 100), PID: 1234}})
	_, _ = m.SetNice("t", 10, false)

	// External actor changes nice.
	fc.Nice[1234] = 5
	// Restore without force refuses to overwrite drift (exit 7 territory).
	res, _ := m.Restore("t", false)
	if res.Counts[StatusDrifted] != 1 {
		t.Fatalf("restore should detect drift, got %+v", res.Counts)
	}
	if fc.Nice[1234] != 5 {
		t.Fatal("restore must not overwrite drifted state without force")
	}
	// Forced restore returns to original.
	res, _ = m.Restore("t", true)
	if res.Counts[StatusRestored] != 1 || fc.Nice[1234] != 0 {
		t.Fatalf("forced restore failed: counts=%+v nice=%d", res.Counts, fc.Nice[1234])
	}
}

func TestRestore_Normal(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(7, 2, idFor(7, 70))
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(7, 70), PID: 7}})
	_, _ = m.SetNice("t", 12, false)
	res, _ := m.Restore("t", false)
	if res.Counts[StatusRestored] != 1 || fc.Nice[7] != 2 {
		t.Fatalf("restore to original failed: %+v nice=%d", res.Counts, fc.Nice[7])
	}
}

func TestSetNice_PermissionDenied(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(1234, 0, idFor(1234, 100))
	fc.SetNiceErr[1234] = &permErr{}
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(1234, 100), PID: 1234}})
	res, _ := m.SetNice("t", 10, false)
	if res.Counts[StatusDenied] != 1 {
		t.Fatalf("expected permission_denied, got %+v", res.Counts)
	}
}

type permErr struct{}

func (permErr) Error() string { return "operation not permitted" }

func TestSafeguards_ProtectPID1AndSelf(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(1, 0, idFor(1, 1))
	m.Manage("t", ModeSnapshot, "", []Instance{{ID: idFor(1, 1), PID: 1}})
	res, _ := m.SetNice("t", 10, false)
	if res.Counts[StatusSkipped] != 1 {
		t.Fatalf("PID 1 must be protected, got %+v", res.Counts)
	}
}

func TestStopIntent_SeparateFromObserved_AndOwnership(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(50, 0, idFor(50, 500))
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(50, 500), PID: 50}})

	// Continuing a task procfit did not stop is refused (RFC §15.4).
	res, _ := m.SetStop("t", false)
	if res.Counts[StatusSkipped] != 1 {
		t.Fatalf("continue on non-owned stop should be skipped, got %+v", res.Counts)
	}
	// Stop, then continue is allowed.
	res, _ = m.SetStop("t", true)
	if res.Counts[StatusApplied] != 1 {
		t.Fatalf("stop should apply, got %+v", res.Counts)
	}
	last := fc.Signals[50][len(fc.Signals[50])-1]
	if last != ports.SigStop {
		t.Fatalf("expected SIGSTOP, got %v", last)
	}
	res, _ = m.SetStop("t", false)
	if res.Counts[StatusApplied] != 1 || fc.Signals[50][len(fc.Signals[50])-1] != ports.SigCont {
		t.Fatalf("continue after owned stop should apply SIGCONT")
	}
}

func TestInactiveRetention_And_SnapshotNoRebind(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(60, 0, idFor(60, 600))
	m.Manage("follow", ModeFollow, "", []Instance{{ID: idFor(60, 600), PID: 60}})
	m.Manage("snap", ModeSnapshot, "", []Instance{{ID: idFor(60, 600), PID: 60}})

	// Process vanishes: rebind follow with empty set.
	if err := m.Rebind("follow", nil); err != nil {
		t.Fatal(err)
	}
	ft := m.Find("follow")
	if ft == nil || ft.Active || len(ft.Bindings) != 0 {
		t.Fatal("follow target should remain but be INACTIVE with no bindings (RFC §16.6)")
	}
	// Follow rebinds when it reappears.
	_ = m.Rebind("follow", []Instance{{ID: idFor(61, 610), PID: 61}})
	if !m.Find("follow").Active {
		t.Fatal("follow should rebind to a new match")
	}
	// Snapshot never rebinds.
	_ = m.Rebind("snap", []Instance{{ID: idFor(61, 610), PID: 61}})
	if len(m.Find("snap").Bindings) != 1 || m.Find("snap").Bindings[0].PID != 60 {
		t.Fatal("snapshot target must not rebind (RFC §16.6)")
	}
}

func TestLoadState_BootMismatchDiscardsBindings(t *testing.T) {
	m, _, _ := newMgr(t)
	prior := &State{
		SchemaVersion: SchemaVersion, BootID: "OLD-BOOT",
		Targets: []Target{{ID: "runtime:x", Name: "x", Origin: OriginRuntime, BindingMode: ModeFollow, Active: true,
			Bindings: []Binding{{ID: idFor(9, 90), PID: 9}}}},
	}
	m.LoadState(prior)
	tgt := m.Find("x")
	if tgt == nil {
		t.Fatal("logical target should survive a boot change (RFC §16.5)")
	}
	if len(tgt.Bindings) != 0 || tgt.Active {
		t.Fatal("concrete bindings must be discarded on boot mismatch")
	}
	if m.State().BootID != "boot" {
		t.Fatal("state boot id should be updated to current boot")
	}
}

func TestUnmanage_LeavesKernelUntouched(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(5, 0, idFor(5, 50))
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(5, 50), PID: 5}})
	_, _ = m.SetNice("t", 10, false)
	if err := m.Unmanage("t"); err != nil {
		t.Fatal(err)
	}
	if m.Find("t") != nil {
		t.Fatal("target should be removed")
	}
	if fc.Nice[5] != 10 {
		t.Fatal("unmanage must not change kernel state (RFC §15.8)")
	}
}
