package control

import "testing"

func TestSetFreeze_DistinctCgroupsOnce(t *testing.T) {
	m, fc, _ := newMgr(t)
	// Two procs in the same cgroup, one in another.
	fc.Add(10, 0, idFor(10, 100))
	fc.Add(11, 0, idFor(11, 110))
	fc.Add(12, 0, idFor(12, 120))
	fc.Cgroups[10] = "/system.slice/app.service"
	fc.Cgroups[11] = "/system.slice/app.service"
	fc.Cgroups[12] = "/system.slice/other.service"
	m.Manage("t", ModeFollow, "", []Instance{
		{ID: idFor(10, 100), PID: 10}, {ID: idFor(11, 110), PID: 11}, {ID: idFor(12, 120), PID: 12},
	})

	res, err := m.SetFreeze("t", true, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !fc.Frozen["/system.slice/app.service"] || !fc.Frozen["/system.slice/other.service"] {
		t.Fatalf("both cgroups should be frozen: %+v", fc.Frozen)
	}
	// The 2nd pid in app.service should be Unchanged (cgroup already handled).
	if res.Counts[StatusApplied] != 2 || res.Counts[StatusUnchanged] != 1 {
		t.Fatalf("expected 2 applied + 1 unchanged, got %+v", res.Counts)
	}
	if m.Find("t").DesiredFreeze == nil || !*m.Find("t").DesiredFreeze {
		t.Fatal("desired freeze intent not recorded")
	}

	// Thaw.
	_, _ = m.SetFreeze("t", false, false, false)
	if fc.Frozen["/system.slice/app.service"] {
		t.Fatal("cgroup should be thawed")
	}
}

func TestSetFreeze_RefusesOwnSession(t *testing.T) {
	m, fc, _ := newMgr(t)
	// procfit (self pid 99999) runs inside the user session...
	fc.Cgroups[99999] = "/user.slice/user-1000.slice/session-2.scope"
	fc.Add(10, 0, idFor(10, 100))
	fc.Cgroups[10] = "/user.slice/user-1000.slice" // ...and the target is its ancestor.
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(10, 100), PID: 10}})

	// Without --force: refused, cgroup left untouched (no self-lockout).
	res, _ := m.SetFreeze("t", true, false, false)
	if res.Counts[StatusSkipped] != 1 || fc.Frozen["/user.slice/user-1000.slice"] {
		t.Fatalf("freezing own session must be refused: counts=%+v frozen=%v", res.Counts, fc.Frozen)
	}
	// With --force: honoured.
	res, _ = m.SetFreeze("t", true, true, false)
	if res.Counts[StatusApplied] != 1 || !fc.Frozen["/user.slice/user-1000.slice"] {
		t.Fatalf("--force should override the guard: counts=%+v frozen=%v", res.Counts, fc.Frozen)
	}
}

func TestRestore_ResumesStopAndThaws(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(10, 0, idFor(10, 100))
	fc.Cgroups[10] = "/system.slice/app.service"
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(10, 100), PID: 10}})

	if _, err := m.SetStop("t", true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetFreeze("t", true, false, false); err != nil {
		t.Fatal(err)
	}
	if !fc.Frozen["/system.slice/app.service"] {
		t.Fatal("target should be frozen before restore")
	}

	res, err := m.Restore("t", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if fc.Frozen["/system.slice/app.service"] {
		t.Fatal("restore must thaw the cgroup")
	}
	if res.Counts[StatusRestored] == 0 {
		t.Fatalf("restore should report a resumed process: %+v", res.Counts)
	}
	if tgt := m.Find("t"); tgt.DesiredStop != nil || tgt.DesiredFreeze != nil {
		t.Fatal("restore must clear stop/freeze intents")
	}
}

func TestRestoreNice_NiceOnly(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(10, 5, idFor(10, 100)) // starts at nice 5
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(10, 100), PID: 10}})
	if _, err := m.SetNice("t", 15, false); err != nil {
		t.Fatal(err)
	}
	res, err := m.RestoreNice("t", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts[StatusRestored] != 1 {
		t.Fatalf("RestoreNice should restore, got %+v", res.Counts)
	}
	if b := m.Find("t").Bindings[0].Nice; b == nil || b.Desired != b.Original {
		t.Fatalf("nice should be back to original: %+v", b)
	}
	if m.Find("t").DesiredNice != nil {
		t.Fatal("RestoreNice should clear the nice intent")
	}
}

func TestManage_PersistsBindingName(t *testing.T) {
	m, _, _ := newMgr(t)
	m.Manage("t", ModeSnapshot, "", []Instance{{ID: idFor(10, 100), PID: 10, Name: "idea"}})
	if got := m.Find("t").Bindings[0].Name; got != "idea" {
		t.Fatalf("binding should persist the name, got %q", got)
	}
	// A rebind without a name keeps the last-known name (so orphans stay labelled).
	m.Manage("t", ModeSnapshot, "", []Instance{{ID: idFor(10, 100), PID: 10}})
	if got := m.Find("t").Bindings[0].Name; got != "idea" {
		t.Fatalf("rebind should keep the last-known name, got %q", got)
	}
}

func TestSetFreeze_NoCgroupUnavailable(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(10, 0, idFor(10, 100)) // no cgroup configured
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(10, 100), PID: 10}})
	res, _ := m.SetFreeze("t", true, false, false)
	if res.Counts[StatusUnavailable] != 1 {
		t.Fatalf("missing cgroup should be unavailable, got %+v", res.Counts)
	}
}
