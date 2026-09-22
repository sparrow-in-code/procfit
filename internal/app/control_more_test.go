package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/netikras/procfit/internal/ports"
)

func TestControl_UsageErrors(t *testing.T) {
	_, restore := fakeControl(t, pstat(1234, "worker", 0))
	defer restore()
	var out, errb bytes.Buffer
	cases := [][]string{
		{"manage"},         // missing target
		{"set"},            // missing target
		{"restore"},        // missing target
		{"unmanage"},       // missing target
		{"signal", "TERM"}, // missing target
	}
	for _, args := range cases {
		out.Reset()
		errb.Reset()
		if code := run(Env{Stdout: &out, Stderr: &errb}, args); code != ExitUsage {
			t.Errorf("%v should be usage error, got %d", args, code)
		}
	}
}

func TestControl_NotFound(t *testing.T) {
	_, restore := fakeControl(t, pstat(1234, "worker", 0))
	defer restore()
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"restore", "managed:ghost"}); code != ExitNotFound {
		t.Fatalf("restore missing target => %d, want NotFound", code)
	}
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"unmanage", "managed:ghost"}); code != ExitNotFound {
		t.Fatalf("unmanage missing target => %d", code)
	}
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"manage", "pid:99999"}); code != ExitNotFound {
		t.Fatalf("manage missing pid => %d", code)
	}
}

func TestControl_SetStopContinueAndDryRun(t *testing.T) {
	fc, restore := fakeControl(t, pstat(1234, "worker", 0))
	defer restore()
	var out, errb bytes.Buffer
	// manage follow then stop.
	run(Env{Stdout: &out, Stderr: &errb}, []string{"manage", "selector:comm == \"worker\"", "--name", "w"})
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"set", "managed:w", "--stop"}); code != ExitOK {
		t.Fatalf("set --stop exit %d stderr=%s", code, errb.String())
	}
	if last := fc.Signals[1234]; len(last) == 0 || last[len(last)-1] != ports.SigStop {
		t.Fatalf("expected SIGSTOP, got %v", fc.Signals[1234])
	}
	// continue.
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"set", "managed:w", "--continue"}); code != ExitOK {
		t.Fatalf("set --continue exit %d", code)
	}
	if last := fc.Signals[1234]; last[len(last)-1] != ports.SigCont {
		t.Fatalf("expected SIGCONT, got %v", fc.Signals[1234])
	}
	// dry-run set nice does not mutate.
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"set", "managed:w", "--nice", "9", "--set-nice", "--dry-run"}); code != ExitOK {
		t.Fatalf("dry-run exit %d", code)
	}
	if fc.Nice[1234] != 0 {
		t.Fatal("dry-run must not change nice")
	}
	// dry-run --stop delivers no signal and previews (PM-0510).
	before := len(fc.Signals[1234])
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"set", "managed:w", "--stop", "--dry-run"}); code != ExitOK {
		t.Fatalf("dry-run --stop exit %d stderr=%s", code, errb.String())
	}
	if len(fc.Signals[1234]) != before {
		t.Fatalf("dry-run --stop delivered a signal: %v", fc.Signals[1234])
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Fatalf("dry-run --stop should print a preview:\n%s", out.String())
	}
	// dry-run signal (destructive TERM, non-TTY) needs no --yes and delivers nothing.
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"signal", "TERM", "pid:1234", "--dry-run"}); code != ExitOK {
		t.Fatalf("dry-run signal exit %d stderr=%s", code, errb.String())
	}
	if len(fc.Signals[1234]) != before {
		t.Fatalf("dry-run signal delivered a signal: %v", fc.Signals[1234])
	}
	// dry-run restore does not revert a real renice.
	run(Env{Stdout: &out, Stderr: &errb}, []string{"set", "managed:w", "--nice", "7", "--set-nice"})
	if fc.Nice[1234] != 7 {
		t.Fatalf("setup renice failed: %v", fc.Nice)
	}
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"restore", "managed:w", "--dry-run"}); code != ExitOK {
		t.Fatalf("dry-run restore exit %d stderr=%s", code, errb.String())
	}
	if fc.Nice[1234] != 7 {
		t.Fatalf("dry-run restore reverted nice to %d", fc.Nice[1234])
	}
}

func TestControl_ManageGroupResolution(t *testing.T) {
	fc, restore := fakeControl(t, pstat(1234, "chrome", 0), pstat(1235, "bash", 0))
	defer restore()
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"manage", "group:comm=chrome", "--name", "c", "--nice", "5", "--set-nice"}); code != ExitOK {
		t.Fatalf("manage group exit %d stderr=%s", code, errb.String())
	}
	if fc.Nice[1234] != 5 || fc.Nice[1235] != 0 {
		t.Fatalf("group:comm=chrome should affect only chrome: %v", fc.Nice)
	}
}

func TestControl_BarePIDResolution(t *testing.T) {
	_, restore := fakeControl(t, pstat(4242, "x", 0))
	defer restore()
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"manage", "4242", "--name", "b"}); code != ExitOK {
		t.Fatalf("bare pid manage exit %d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "1 instance") {
		t.Fatalf("bare pid should resolve one instance:\n%s", out.String())
	}
}
