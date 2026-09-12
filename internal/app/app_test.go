package app

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/testutil"
)

func stat(pid int, comm string, u, s, rss uint64) ports.ProcStat {
	return ports.ProcStat{
		ID:         model.ProcessInstanceID{BootID: "boot-test", PID: pid, StartTime: uint64(pid)},
		PID:        pid,
		Comm:       comm,
		State:      model.ProcessState{Code: model.StateRunning},
		NiceAvail:  model.Available,
		NumThreads: 1,
		StartTicks: uint64(pid),
		UTimeTicks: u,
		STimeTicks: s,
		RSSBytes:   rss,
		IOAvail:    model.Available,
	}
}

// fakeAssembly installs a fake-backed assembly and returns a restore func.
func fakeAssembly(t *testing.T, gens ...[]ports.ProcStat) (*testutil.FakeClock, func()) {
	t.Helper()
	clk := testutil.NewFakeClock(time.Unix(1_600_000_100, 0))
	src := testutil.NewFakeSource(gens...)
	prev := newAssemblyFn
	newAssemblyFn = func() (*assembly, error) {
		a := newAssemblyWith(src, clk)
		a.wait = func(d time.Duration) <-chan time.Time {
			clk.Advance(d)
			ch := make(chan time.Time, 1)
			ch <- clk.Now()
			return ch
		}
		return a, nil
	}
	return clk, func() { newAssemblyFn = prev }
}

func runCmd(env Env, args ...string) int { return run(env, args) }

func TestPS_TableWarmupThenRates(t *testing.T) {
	g1 := []ports.ProcStat{stat(1, "worker", 100, 0, 4096)}
	g2 := []ports.ProcStat{stat(1, "worker", 200, 0, 4096)} // +100 ticks over 1s @hz100 => 100%
	_, restore := fakeAssembly(t, g1, g2)
	defer restore()

	var out, errb bytes.Buffer
	code := runCmd(Env{Stdout: &out, Stderr: &errb}, "ps", "--group-by", "none", "--leaf", "process", "--columns", "target,cpu")
	if code != ExitOK {
		t.Fatalf("exit %d, stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "worker") {
		t.Fatalf("missing process:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "100.0") {
		t.Fatalf("expected cpu ~100 after warmup:\n%s", out.String())
	}
}

func TestPS_InstantRatesUnavailable(t *testing.T) {
	g1 := []ports.ProcStat{stat(1, "worker", 100, 0, 4096)}
	_, restore := fakeAssembly(t, g1, g1)
	defer restore()

	var out, errb bytes.Buffer
	code := runCmd(Env{Stdout: &out, Stderr: &errb}, "ps", "--instant", "--group-by", "none", "--leaf", "process", "--columns", "target,cpu")
	if code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	// cpu must be '-' (warming), never 0.0, on an instant single sample.
	line := lastLine(out.String())
	if strings.Contains(line, "0.0") {
		t.Fatalf("instant cpu must not be zero: %q", line)
	}
	if !strings.Contains(line, "-") {
		t.Fatalf("instant cpu should be '-': %q", line)
	}
}

func TestPS_JSONFormat(t *testing.T) {
	g := []ports.ProcStat{stat(1, "worker", 100, 0, 4096)}
	_, restore := fakeAssembly(t, g, g)
	defer restore()

	var out, errb bytes.Buffer
	code := runCmd(Env{Stdout: &out, Stderr: &errb}, "ps", "--format", "json", "--group-by", "none", "--leaf", "process")
	if code != ExitOK {
		t.Fatalf("exit %d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), `"schema_version": 1`) {
		t.Fatalf("json envelope missing:\n%s", out.String())
	}
}

func TestStat_CountEmitsBatches(t *testing.T) {
	g1 := []ports.ProcStat{stat(1, "worker", 100, 0, 4096)}
	g2 := []ports.ProcStat{stat(1, "worker", 150, 0, 4096)}
	g3 := []ports.ProcStat{stat(1, "worker", 220, 0, 4096)}
	_, restore := fakeAssembly(t, g1, g2, g3)
	defer restore()

	var out, errb bytes.Buffer
	code := runCmd(Env{Stdout: &out, Stderr: &errb}, "stat", "1s", "--count", "3", "--group-by", "comm", "--leaf", "none", "--columns", "target,cpu")
	if code != ExitOK {
		t.Fatalf("exit %d stderr=%s", code, errb.String())
	}
	if n := strings.Count(out.String(), "== "); n != 3 {
		t.Fatalf("want 3 timestamped batches, got %d:\n%s", n, out.String())
	}
}

func TestRun_Routing(t *testing.T) {
	var out, errb bytes.Buffer
	if code := runCmd(Env{Stdout: &out, Stderr: &errb}, "version"); code != ExitOK {
		t.Fatalf("version exit %d", code)
	}
	if !strings.Contains(out.String(), "procfit") {
		t.Fatal("version output missing name")
	}
	errb.Reset()
	if code := runCmd(Env{Stdout: &out, Stderr: &errb}, "bogus"); code != ExitUsage {
		t.Fatalf("unknown command should be ExitUsage, got %d", code)
	}
	errb.Reset()
	// No args and no TTY => usage error.
	if code := runCmd(Env{Stdout: &out, Stderr: &errb, IsTTY: false}); code != ExitUsage {
		t.Fatalf("no-TTY bare invocation should be ExitUsage, got %d", code)
	}
}

func TestPS_SelectUnimplemented(t *testing.T) {
	_, restore := fakeAssembly(t, []ports.ProcStat{stat(1, "a", 1, 0, 1)})
	defer restore()
	var out, errb bytes.Buffer
	code := runCmd(Env{Stdout: &out, Stderr: &errb}, "ps", "--select", "uid == 0")
	if code != ExitUsage {
		t.Fatalf("select should currently be ExitUsage, got %d", code)
	}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[len(lines)-1]
}
