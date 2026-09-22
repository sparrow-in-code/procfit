package procfs

import (
	"context"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/model"
)

func TestParseSchedField(t *testing.T) {
	dump := "worker (100, #threads: 1)\n" +
		"se.exec_start : 123456.789\n" +
		"nr_switches                                  :   4200\n" +
		"nr_wakeups                                   :   987\n"
	if n, ok := parseSchedField(dump, "nr_wakeups"); !ok || n != 987 {
		t.Fatalf("nr_wakeups = %d ok=%v, want 987", n, ok)
	}
	if n, ok := parseSchedField(dump, "nr_switches"); !ok || n != 4200 {
		t.Fatalf("nr_switches = %d ok=%v, want 4200", n, ok)
	}
	if _, ok := parseSchedField(dump, "nr_missing"); ok {
		t.Fatal("absent field must not parse (CONFIG_SCHEDSTATS off)")
	}
}

func TestSchedWakeupsCollector_Window(t *testing.T) {
	root := t.TempDir()
	// pid 100 accrues 20 wakeups over a 100ms window -> 200/s.
	writeProcFile(t, root, 100, "sched", "p (100, #threads: 1)\nnr_wakeups : 1000\n")
	// pid 200: no sched file -> unavailable, never zero.

	c := NewSchedWakeupsCollector(root)
	c.window = 100 * time.Millisecond
	c.sleep = func(time.Duration) {
		writeProcFile(t, root, 100, "sched", "p (100, #threads: 1)\nnr_wakeups : 1020\n")
	}
	procs := []model.Process{{PID: 100}, {PID: 200}}
	c.Collect(context.Background(), procs)

	if v := procs[0].Metric("wakeups"); !v.Present() || v.V < 199 || v.V > 201 {
		t.Fatalf("pid100 wakeups = %+v, want ~200/s", v)
	}
	if v := procs[0].Metric("wakeups"); v.Source != "sched" {
		t.Fatalf("pid100 wakeups source = %q, want sched", v.Source)
	}
	if v := procs[1].Metric("wakeups"); v.Present() {
		t.Fatalf("pid200 (no sched) must be unavailable, got %+v", v)
	}
}

func TestSchedWakeupsCollector_EBPFPreferred(t *testing.T) {
	root := t.TempDir()
	writeProcFile(t, root, 100, "sched", "p (100, #threads: 1)\nnr_wakeups : 1000\n")

	c := NewSchedWakeupsCollector(root)
	c.window = 100 * time.Millisecond
	windowRan := false
	c.sleep = func(time.Duration) { windowRan = true }

	// A process whose wakeups the eBPF source already filled must be left alone,
	// and (all pending resolved) the fallback must not even run its window.
	procs := []model.Process{{PID: 100}}
	procs[0].SetMetric("wakeups", model.NewValue(42.0, model.Sampled, "ebpf"))
	c.Collect(context.Background(), procs)

	if v := procs[0].Metric("wakeups"); v.Source != "ebpf" || v.V != 42 {
		t.Fatalf("eBPF value must win, got %+v", v)
	}
	if windowRan {
		t.Fatal("no pending processes: the fallback must skip its measurement window")
	}
}
