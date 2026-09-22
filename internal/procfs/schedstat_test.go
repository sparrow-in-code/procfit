package procfs

import (
	"context"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/model"
)

func TestParseSchedstatRunDelay(t *testing.T) {
	if ns, ok := parseSchedstatRunDelay("123456 789 42\n"); !ok || ns != 789 {
		t.Fatalf("run_delay = %d ok=%v, want 789", ns, ok)
	}
	if _, ok := parseSchedstatRunDelay("only-one-field"); ok {
		t.Fatal("a short schedstat line must not parse")
	}
}

func TestSchedstatCollector_Window(t *testing.T) {
	root := t.TempDir()
	// pid 100 accrues 50ms of run-queue wait over a 100ms window -> 50%.
	writeProcFile(t, root, 100, "schedstat", "1000000 1000000 5\n")
	// pid 200: no schedstat file (CONFIG_SCHEDSTATS off) -> unavailable.

	c := NewSchedstatCollector(root)
	c.window = 100 * time.Millisecond
	c.sleep = func(time.Duration) {
		writeProcFile(t, root, 100, "schedstat", "1000000 51000000 6\n") // +50ms run_delay
	}
	procs := []model.Process{{PID: 100}, {PID: 200}}
	c.Collect(context.Background(), procs)

	if v := procs[0].Metric("runq-delay"); !v.Present() || v.V < 49.9 || v.V > 50.1 {
		t.Fatalf("pid100 runq-delay = %+v, want ~50%%", v)
	}
	if v := procs[1].Metric("runq-delay"); v.Present() {
		t.Fatalf("pid200 (no schedstat) must be unavailable, got %+v", v)
	}
}
