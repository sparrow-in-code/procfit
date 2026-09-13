package app

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/state"
	"github.com/netikras/procfit/internal/testutil"
	"github.com/netikras/procfit/internal/tui"
)

// fakeControl installs a control assembly backed by a fake controller/source and
// a real (temp) state store, shared across command invocations so persistence
// works. Returns the shared controller and a restore func.
func fakeControl(t *testing.T, procs ...ports.ProcStat) (*testutil.FakeController, func()) {
	t.Helper()
	dir := t.TempDir()
	fc := testutil.NewFakeController()
	src := testutil.NewFakeSource(procs)
	for _, p := range procs {
		fc.Add(p.PID, p.Nice, p.ID)
	}
	clk := testutil.NewFakeClock(time.Unix(2000, 0))
	prev := newControlAsmFn
	newControlAsmFn = func(_ string) (*ctlAsm, error) {
		store, err := state.Open(dir)
		if err != nil {
			return nil, err
		}
		mgr := control.NewManager(fc, clk, control.NewSafeguards(999999, 888888), "boot-test")
		if st, ok, err := store.Load(); err == nil && ok {
			mgr.LoadState(st)
		}
		return &ctlAsm{src: src, ctrl: fc, clk: clk, mgr: mgr, store: store}, nil
	}
	return fc, func() { newControlAsmFn = prev }
}

func pstat(pid int, comm string, nice int) ports.ProcStat {
	return ports.ProcStat{
		ID:  model.ProcessInstanceID{BootID: "boot-test", PID: pid, StartTime: uint64(pid)},
		PID: pid, Comm: comm, Nice: nice, NiceAvail: model.Available, IOAvail: model.Available,
	}
}

func TestTUIControl_PreviewAndApply(t *testing.T) {
	_, restore := fakeControl(t, pstat(1234, "worker", 0))
	defer restore()
	ctl := tuiControl()

	// nice: real manager dry-run preview, then apply.
	prev, err := ctl(tui.ControlRequest{PIDs: []int{1234}, Label: "worker", Kind: tui.CtrlNice, Nice: 10, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prev, "nice→10") {
		t.Fatalf("nice preview wrong: %q", prev)
	}
	res, err := ctl(tui.ControlRequest{PIDs: []int{1234}, Label: "worker", Kind: tui.CtrlNice, Nice: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "applied") {
		t.Fatalf("nice apply wrong: %q", res)
	}

	// stop: preview is synthesized (no manager mutation), then apply acts.
	prev, _ = ctl(tui.ControlRequest{PIDs: []int{1234}, Label: "worker", Kind: tui.CtrlStop, DryRun: true})
	if !strings.Contains(prev, "would stop") {
		t.Fatalf("stop preview wrong: %q", prev)
	}
	res, err = ctl(tui.ControlRequest{PIDs: []int{1234}, Label: "worker", Kind: tui.CtrlStop})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "pid 1234") {
		t.Fatalf("stop apply wrong: %q", res)
	}
}

func TestTUIControl_Group(t *testing.T) {
	_, restore := fakeControl(t, pstat(100, "a", 0), pstat(200, "b", 0))
	defer restore()
	ctl := tuiControl()

	// A group action over multiple pids aggregates status counts.
	prev, err := ctl(tui.ControlRequest{PIDs: []int{100, 200}, Label: "grp", Kind: tui.CtrlNice, Nice: 5, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prev, "2 procs") {
		t.Fatalf("group nice preview should count procs: %q", prev)
	}
	res, err := ctl(tui.ControlRequest{PIDs: []int{100, 200}, Label: "grp", Kind: tui.CtrlNice, Nice: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "2 procs") || !strings.Contains(res, "applied=2") {
		t.Fatalf("group nice apply should report 2 applied: %q", res)
	}
}

func TestTUIManaged_ListRestoreUnmanage(t *testing.T) {
	_, restore := fakeControl(t, pstat(100, "a", 0))
	defer restore()

	// Create a managed target by renicing via the TUI control path.
	if _, err := tuiControl()(tui.ControlRequest{PIDs: []int{100}, Label: "a", Kind: tui.CtrlNice, Nice: 5}); err != nil {
		t.Fatal(err)
	}
	rows := tuiManaged()()
	if len(rows) != 1 || rows[0].Nice != "5" {
		t.Fatalf("managed list should show the target with nice=5, got %+v", rows)
	}
	name := rows[0].Name
	act := tuiManagedAction()
	if _, err := act(name, tui.ManagedRestore); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if _, err := act(name, tui.ManagedUnmanage); err != nil {
		t.Fatalf("unmanage: %v", err)
	}
	if got := tuiManaged()(); len(got) != 0 {
		t.Fatalf("target should be gone after unmanage, got %+v", got)
	}
}

func TestControl_ManageSetRestoreFlow(t *testing.T) {
	_, restore := fakeControl(t, pstat(1234, "worker", 0))
	defer restore()

	var out, errb bytes.Buffer
	// Manage + set nice 10.
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"manage", "pid:1234", "--name", "w", "--nice", "10", "--set-nice"}); code != ExitOK {
		t.Fatalf("manage exit %d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "applied") {
		t.Fatalf("expected applied:\n%s", out.String())
	}
	// managed lists it.
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"managed"}); code != ExitOK {
		t.Fatalf("managed exit %d", code)
	}
	if !strings.Contains(out.String(), "w") || !strings.Contains(out.String(), "ACTIVE") {
		t.Fatalf("managed output wrong:\n%s", out.String())
	}
	// restore back to original 0.
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"restore", "managed:w"}); code != ExitOK {
		t.Fatalf("restore exit %d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "restored") {
		t.Fatalf("expected restored:\n%s", out.String())
	}
}

func TestControl_RestoreRefusesDrift(t *testing.T) {
	fc, restore := fakeControl(t, pstat(1234, "worker", 0))
	defer restore()
	var out, errb bytes.Buffer
	run(Env{Stdout: &out, Stderr: &errb}, []string{"manage", "pid:1234", "--name", "w", "--nice", "10", "--set-nice"})
	// External drift.
	fc.Nice[1234] = 3
	out.Reset()
	code := run(Env{Stdout: &out, Stderr: &errb}, []string{"restore", "managed:w"})
	if code != ExitConflict {
		t.Fatalf("drifted restore should exit 7, got %d\n%s", code, out.String())
	}
}

func TestControl_Unmanage(t *testing.T) {
	_, restore := fakeControl(t, pstat(1234, "worker", 0))
	defer restore()
	var out, errb bytes.Buffer
	run(Env{Stdout: &out, Stderr: &errb}, []string{"manage", "pid:1234", "--name", "w"})
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"unmanage", "managed:w"}); code != ExitOK {
		t.Fatalf("unmanage exit %d", code)
	}
	out.Reset()
	run(Env{Stdout: &out, Stderr: &errb}, []string{"managed"})
	if !strings.Contains(out.String(), "no managed targets") {
		t.Fatalf("target should be gone:\n%s", out.String())
	}
}

func TestControl_SignalRequiresConfirmationNonTTY(t *testing.T) {
	_, restore := fakeControl(t, pstat(1234, "worker", 0))
	defer restore()
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb, IsTTY: false}, []string{"signal", "TERM", "pid:1234"}); code != ExitUsage {
		t.Fatalf("destructive signal without --yes on non-TTY should be usage error, got %d", code)
	}
	// With --yes it proceeds.
	out.Reset()
	if code := run(Env{Stdout: &out, Stderr: &errb, IsTTY: false}, []string{"signal", "TERM", "pid:1234", "--yes"}); code != ExitOK {
		t.Fatalf("signal with --yes exit %d stderr=%s", code, errb.String())
	}
}

func TestControl_SignalSelector(t *testing.T) {
	fc, restore := fakeControl(t, pstat(1234, "chrome", 0), pstat(1235, "bash", 0))
	defer restore()
	var out, errb bytes.Buffer
	if code := run(Env{Stdout: &out, Stderr: &errb}, []string{"signal", "HUP", "selector:comm == \"chrome\""}); code != ExitOK {
		t.Fatalf("signal selector exit %d stderr=%s", code, errb.String())
	}
	if len(fc.Signals[1234]) != 1 || len(fc.Signals[1235]) != 0 {
		t.Fatalf("only chrome should get the signal: %+v", fc.Signals)
	}
}
