package control

import "testing"

func TestReconcile_AppliesToNewMatchesAndCapturesOriginal(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(10, 0, idFor(10, 100))
	nice := 12
	m.SetPolicies([]PolicySpec{{Name: "browsers", Selector: `comm == "chrome"`, Nice: &nice, OnDrift: "report"}})

	// First reconcile binds pid 10 and applies nice=12, capturing original 0.
	m.Reconcile(func(sel string) ([]Instance, error) {
		return []Instance{{ID: idFor(10, 100), PID: 10}}, nil
	})
	if fc.Nice[10] != 12 {
		t.Fatalf("policy nice not applied: %d", fc.Nice[10])
	}
	b := m.Find("browsers").Bindings[0]
	if b.Nice == nil || b.Nice.Original != 0 || b.Nice.Desired != 12 {
		t.Fatalf("original/desired wrong: %+v", b.Nice)
	}
}

func TestReconcile_DriftReport(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(10, 0, idFor(10, 100))
	nice := 12
	m.SetPolicies([]PolicySpec{{Name: "b", Selector: "x", Nice: &nice, OnDrift: "report"}})
	resolve := func(string) ([]Instance, error) { return []Instance{{ID: idFor(10, 100), PID: 10}}, nil }
	m.Reconcile(resolve)
	// External change.
	fc.Nice[10] = 3
	m.Reconcile(resolve)
	if fc.Nice[10] != 3 {
		t.Fatal("report mode must not overwrite external drift")
	}
	if m.Find("b").Bindings[0].Nice.Status != StatusDrifted {
		t.Fatalf("expected drifted status, got %s", m.Find("b").Bindings[0].Nice.Status)
	}
}

func TestReconcile_DriftReapply(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(10, 0, idFor(10, 100))
	nice := 12
	m.SetPolicies([]PolicySpec{{Name: "b", Selector: "x", Nice: &nice, OnDrift: "reapply"}})
	resolve := func(string) ([]Instance, error) { return []Instance{{ID: idFor(10, 100), PID: 10}}, nil }
	m.Reconcile(resolve)
	fc.Nice[10] = 3
	m.Reconcile(resolve)
	if fc.Nice[10] != 12 {
		t.Fatalf("reapply should restore desired, got %d", fc.Nice[10])
	}
}

func TestSetPolicies_PreservesRuntimeTargets(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(10, 0, idFor(10, 100))
	m.Manage("manual", ModeFollow, "", []Instance{{ID: idFor(10, 100), PID: 10}})
	m.SetPolicies([]PolicySpec{{Name: "p", Selector: "x"}})
	if m.Find("manual") == nil {
		t.Fatal("runtime target must survive SetPolicies")
	}
	if m.Find("p") == nil {
		t.Fatal("policy target should be installed")
	}
	// Replacing policies again keeps the manual one.
	m.SetPolicies([]PolicySpec{{Name: "q", Selector: "y"}})
	if m.Find("manual") == nil || m.Find("p") != nil || m.Find("q") == nil {
		t.Fatal("SetPolicies should replace config targets only")
	}
}
