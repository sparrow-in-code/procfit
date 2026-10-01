package collect

import (
	"context"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/testutil"
)

func TestSampler_DiskRateAndGauges(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_600_003_600, 0)) // 3600s after boot 1_600_000_000
	base := stat(10, 100, 0, 0, 4096)
	base.IO = ports.ProcIO{ReadBytes: 1000}
	base.NumThreads = 3
	base.Nice = 5
	g2 := stat(10, 100, 0, 0, 4096)
	g2.IO = ports.ProcIO{ReadBytes: 3000} // +2000 bytes over 1s
	g2.NumThreads = 3
	g2.Nice = 5
	src := testutil.NewFakeSource([]ports.ProcStat{base}, []ports.ProcStat{g2})
	s := NewSampler(src, clk)

	want := []model.MetricID{"disk-rbps", "threads", "pnice", "age"}
	_, _ = s.Sample(context.Background(), want)
	clk.Advance(time.Second)
	snap, _ := s.Sample(context.Background(), want)
	p := snap.Processes[0]

	d, ok := p.Metric("disk-rbps").Get()
	if !ok || d != 2000 {
		t.Fatalf("disk-rbps = (%v,%v), want 2000", d, ok)
	}
	if th, _ := p.Metric("threads").Get(); th != 3 {
		t.Fatalf("threads = %v, want 3", th)
	}
	if n, _ := p.Metric("pnice").Get(); n != 5 {
		t.Fatalf("pnice = %v, want 5", n)
	}
	age, ok := p.Metric("age").Get()
	if !ok || age <= 0 {
		t.Fatalf("age should be positive, got %v ok=%v", age, ok)
	}
}

func TestSampler_PniceUnavailable(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_600_000_100, 0))
	st := stat(10, 50, 0, 0, 4096)
	st.NiceAvail = model.PermissionDenied
	src := testutil.NewFakeSource([]ports.ProcStat{st})
	s := NewSampler(src, clk)
	snap, _ := s.Sample(context.Background(), []model.MetricID{"pnice"})
	v := snap.Processes[0].Metric("pnice")
	if v.Present() || v.Availability != model.PermissionDenied {
		t.Fatalf("pnice should be permission_denied, got %q", v.Availability)
	}
}

func TestSampler_AgeUnsupportedWithoutBootTime(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_600_000_100, 0))
	src := testutil.NewFakeSource([]ports.ProcStat{stat(10, 50, 0, 0, 4096)})
	src.BootUnix = 0
	s := NewSampler(src, clk)
	snap, _ := s.Sample(context.Background(), []model.MetricID{"age"})
	if a := snap.Processes[0].Metric("age"); a.Availability != model.Unsupported {
		t.Fatalf("age without boot time should be unsupported, got %q", a.Availability)
	}
}

func TestSampler_CPUTimeAndAge(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_600_000_100, 0))
	// utime+stime = 120 ticks @hz100 => 1.2s cpu-time; start 50 ticks, boot
	// 1_600_000_000 => ~100s age. Both are gauges: available on the first sample.
	src := testutil.NewFakeSource([]ports.ProcStat{stat(10, 50, 100, 20, 4096)})
	s := NewSampler(src, clk)
	snap, err := s.Sample(context.Background(), []model.MetricID{"cpu-time", "age"})
	if err != nil {
		t.Fatal(err)
	}
	p := snap.Processes[0]
	if v, ok := p.Metric("cpu-time").Get(); !ok || v < 1.19 || v > 1.21 {
		t.Fatalf("cpu-time = (%v,%v), want ~1.2s on first sample", v, ok)
	}
	if v, ok := p.Metric("age").Get(); !ok || v < 99 || v > 101 {
		t.Fatalf("age = (%v,%v), want ~100s", v, ok)
	}
}

func TestSampler_ThreadCPUTimeAndAge(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_600_000_100, 0))
	st := stat(10, 50, 0, 0, 4096)
	st.Threads = []ports.ThreadStat{
		{TID: 11, StartTicks: 50, Comm: "worker", UTimeTicks: 30, STimeTicks: 10}, // 40 ticks => 0.4s
	}
	src := testutil.NewFakeSource([]ports.ProcStat{st})
	s := NewSampler(src, clk)
	snap, err := s.Sample(context.Background(), []model.MetricID{"cpu-time", "age"})
	if err != nil {
		t.Fatal(err)
	}
	th := snap.Processes[0].Threads[0]
	if v, ok := th.Metrics["cpu-time"].Get(); !ok || v < 0.39 || v > 0.41 {
		t.Fatalf("thread cpu-time = (%v,%v), want ~0.4s (per-thread gauge)", v, ok)
	}
	if v, ok := th.Metrics["age"].Get(); !ok || v < 99 || v > 101 {
		t.Fatalf("thread age = (%v,%v), want ~100s from its own start time", v, ok)
	}
}

func TestSampler_UnknownMetricDisabled(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_600_000_100, 0))
	src := testutil.NewFakeSource([]ports.ProcStat{stat(10, 50, 0, 0, 4096)})
	s := NewSampler(src, clk)
	snap, _ := s.Sample(context.Background(), []model.MetricID{"does-not-exist"})
	if v := snap.Processes[0].Metric("does-not-exist"); v.Availability != model.Disabled {
		t.Fatalf("unknown metric should be disabled, got %q", v.Availability)
	}
}

func TestSampler_ListError(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(0, 0))
	src := testutil.NewFakeSource()
	src.ListErr = context.DeadlineExceeded
	s := NewSampler(src, clk)
	if _, err := s.Sample(context.Background(), nil); err == nil {
		t.Fatal("expected error from source")
	}
}
