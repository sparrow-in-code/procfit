package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestManaged_Empty(t *testing.T) {
	_, restore := fakeControl(t, pstat(1, "x", 0))
	defer restore()
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"managed"}); code != ExitOK {
		t.Fatalf("managed exit %d", code)
	}
	if !strings.Contains(out.String(), "no managed targets") {
		t.Fatalf("expected empty listing: %s", out.String())
	}
}

func TestControl_Freeze(t *testing.T) {
	fc, restore := fakeControl(t, pstat(1234, "worker", 0))
	defer restore()
	fc.Cgroups[1234] = "/system.slice/worker.service"
	var out, errb bytes.Buffer
	// manage then freeze.
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"manage", "pid:1234", "--name", "w"}); code != ExitOK {
		t.Fatalf("manage exit %d stderr=%s", code, errb.String())
	}
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"set", "managed:w", "--freeze"}); code != ExitOK {
		t.Fatalf("freeze exit %d stderr=%s", code, errb.String())
	}
	if !fc.Frozen["/system.slice/worker.service"] {
		t.Fatalf("cgroup should be frozen: %+v", fc.Frozen)
	}
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"set", "managed:w", "--thaw"}); code != ExitOK {
		t.Fatalf("thaw exit %d", code)
	}
	if fc.Frozen["/system.slice/worker.service"] {
		t.Fatal("cgroup should be thawed")
	}
}

func TestControl_SignalByManaged(t *testing.T) {
	fc, restore := fakeControl(t, pstat(1234, "worker", 0))
	defer restore()
	var out, errb bytes.Buffer
	run(Env{Stdout: &out, Stderr: &errb}, []string{"manage", "pid:1234", "--name", "w"})
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"signal", "HUP", "managed:w"}); code != ExitOK {
		t.Fatalf("signal managed exit %d stderr=%s", code, errb.String())
	}
	if len(fc.Signals[1234]) != 1 {
		t.Fatalf("expected one signal, got %v", fc.Signals[1234])
	}
}

func TestControl_ManageWithNiceAndStop(t *testing.T) {
	fc, restore := fakeControl(t, pstat(1234, "worker", 0))
	defer restore()
	var out, errb bytes.Buffer
	code := run(Env{Stdout: &out, Stderr: &errb}, []string{"manage", "pid:1234", "--name", "w", "--nice", "7", "--set-nice", "--stop"})
	if code != ExitOK {
		t.Fatalf("manage w/ control exit %d stderr=%s", code, errb.String())
	}
	if fc.Nice[1234] != 7 {
		t.Fatalf("nice not applied: %d", fc.Nice[1234])
	}
	if len(fc.Signals[1234]) == 0 {
		t.Fatal("stop should have signaled")
	}
}
