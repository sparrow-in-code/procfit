package model

import "testing"

func TestProcessInstanceID_ReusedPIDIsNotSame(t *testing.T) {
	a := ProcessInstanceID{BootID: "b", PIDNSInode: 1, PID: 1234, StartTime: 1000}
	// Same PID, different start time = a reused PID = a different instance.
	b := ProcessInstanceID{BootID: "b", PIDNSInode: 1, PID: 1234, StartTime: 2000}
	if a.SameProcess(b) {
		t.Fatal("reused PID (different starttime) must not compare equal (RFC §6.3)")
	}
	if a.Key() == b.Key() {
		t.Fatal("keys must differ for different instances")
	}
}

func TestProcessInstanceID_SameProcess(t *testing.T) {
	a := ProcessInstanceID{BootID: "b", PIDNSInode: 1, PID: 1234, StartTime: 1000}
	b := a
	if !a.SameProcess(b) {
		t.Fatal("identical ids should be the same process")
	}
}

func TestProcessInstanceID_DifferentBoot(t *testing.T) {
	a := ProcessInstanceID{BootID: "boot-1", PIDNSInode: 1, PID: 1, StartTime: 1}
	b := ProcessInstanceID{BootID: "boot-2", PIDNSInode: 1, PID: 1, StartTime: 1}
	if a.SameProcess(b) {
		t.Fatal("same pid/starttime across boots must not be the same instance")
	}
}

func TestProcessInstanceID_UnknownScopeTolerated(t *testing.T) {
	// When boot/ns are unknown on one side, identity falls back to pid+starttime.
	a := ProcessInstanceID{PID: 5, StartTime: 9}
	b := ProcessInstanceID{BootID: "b", PIDNSInode: 3, PID: 5, StartTime: 9}
	if !a.SameProcess(b) {
		t.Fatal("unknown boot/ns should not block a pid+starttime match")
	}
}

func TestProcessInstanceID_Zero(t *testing.T) {
	if !(ProcessInstanceID{}).Zero() {
		t.Fatal("empty id should be Zero")
	}
	if (ProcessInstanceID{PID: 1}).Zero() {
		t.Fatal("non-empty id should not be Zero")
	}
}

func TestThreadInstanceID(t *testing.T) {
	p := ProcessInstanceID{BootID: "b", PID: 10, StartTime: 100}
	a := ThreadInstanceID{Process: p, TID: 11, StartTime: 100}
	b := ThreadInstanceID{Process: p, TID: 11, StartTime: 100}
	if !a.SameThread(b) {
		t.Fatal("identical thread ids should match")
	}
	c := ThreadInstanceID{Process: p, TID: 12, StartTime: 100}
	if a.SameThread(c) {
		t.Fatal("different TID should not match")
	}
	if a.Key() == c.Key() {
		t.Fatal("keys should differ")
	}
	if a.Zero() {
		t.Fatal("non-empty thread id should not be Zero")
	}
}
