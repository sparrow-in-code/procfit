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

	res, err := m.SetFreeze("t", true)
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
	_, _ = m.SetFreeze("t", false)
	if fc.Frozen["/system.slice/app.service"] {
		t.Fatal("cgroup should be thawed")
	}
}

func TestSetFreeze_NoCgroupUnavailable(t *testing.T) {
	m, fc, _ := newMgr(t)
	fc.Add(10, 0, idFor(10, 100)) // no cgroup configured
	m.Manage("t", ModeFollow, "", []Instance{{ID: idFor(10, 100), PID: 10}})
	res, _ := m.SetFreeze("t", true)
	if res.Counts[StatusUnavailable] != 1 {
		t.Fatalf("missing cgroup should be unavailable, got %+v", res.Counts)
	}
}
