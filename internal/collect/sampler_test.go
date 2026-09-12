package collect

import (
	"context"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/testutil"
)

func stat(pid int, start uint64, utime, stime, rss uint64) ports.ProcStat {
	return ports.ProcStat{
		ID:         model.ProcessInstanceID{BootID: "boot-test", PID: pid, StartTime: start},
		PID:        pid,
		Comm:       "worker",
		State:      model.ProcessState{Code: model.StateRunning},
		Nice:       0,
		NiceAvail:  model.Available,
		NumThreads: 1,
		StartTicks: start,
		UTimeTicks: utime,
		STimeTicks: stime,
		RSSBytes:   rss,
		IOAvail:    model.Available,
	}
}

func TestSampler_FirstSampleWarmsUp(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_600_000_100, 0))
	src := testutil.NewFakeSource([]ports.ProcStat{stat(10, 50, 100, 20, 4096)})
	s := NewSampler(src, clk)

	snap, err := s.Sample(context.Background(), []model.MetricID{"cpu", "rss"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Processes) != 1 {
		t.Fatalf("want 1 process, got %d", len(snap.Processes))
	}
	cpu := snap.Processes[0].Metric("cpu")
	if cpu.Present() {
		t.Fatal("first cpu sample must be warming_up, not a value")
	}
	if cpu.Availability != model.WarmingUp {
		t.Fatalf("cpu availability = %q, want warming_up", cpu.Availability)
	}
	// Gauge is available on the first sample.
	rss := snap.Processes[0].Metric("rss")
	if v, ok := rss.Get(); !ok || v != 4096 {
		t.Fatalf("rss = (%v,%v), want (4096,true)", v, ok)
	}
}

func TestSampler_CPURate(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_600_000_100, 0))
	// hz=100. Over 1s, 50 ticks of cpu => 0.5s cpu => 50% of one CPU.
	g1 := []ports.ProcStat{stat(10, 50, 100, 20, 4096)}
	g2 := []ports.ProcStat{stat(10, 50, 130, 40, 8192)} // +30 utime +20 stime = +50 ticks
	src := testutil.NewFakeSource(g1, g2)
	s := NewSampler(src, clk)

	if _, err := s.Sample(context.Background(), []model.MetricID{"cpu", "cpu-normalized"}); err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Second)
	snap, err := s.Sample(context.Background(), []model.MetricID{"cpu", "cpu-normalized"})
	if err != nil {
		t.Fatal(err)
	}
	cpu := snap.Processes[0].Metric("cpu")
	v, ok := cpu.Get()
	if !ok {
		t.Fatal("cpu should be available on second sample")
	}
	if v < 49.9 || v > 50.1 {
		t.Fatalf("cpu = %v, want ~50", v)
	}
	// cpus=4 => normalized ~12.5
	norm, _ := snap.Processes[0].Metric("cpu-normalized").Get()
	if norm < 12.4 || norm > 12.6 {
		t.Fatalf("cpu-normalized = %v, want ~12.5", norm)
	}
}

func TestSampler_CounterResetDiscarded(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_600_000_100, 0))
	g1 := []ports.ProcStat{stat(10, 50, 500, 0, 4096)}
	g2 := []ports.ProcStat{stat(10, 50, 10, 0, 4096)} // counter went backwards
	src := testutil.NewFakeSource(g1, g2)
	s := NewSampler(src, clk)
	_, _ = s.Sample(context.Background(), []model.MetricID{"cpu"})
	clk.Advance(time.Second)
	snap, _ := s.Sample(context.Background(), []model.MetricID{"cpu"})
	if snap.Processes[0].Metric("cpu").Present() {
		t.Fatal("counter reset must discard the delta (warming_up), not emit a value")
	}
}

func TestSampler_NewProcessWarmsUp(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_600_000_100, 0))
	g1 := []ports.ProcStat{stat(10, 50, 100, 0, 4096)}
	g2 := []ports.ProcStat{stat(10, 50, 200, 0, 4096), stat(11, 60, 100, 0, 2048)} // pid 11 is new
	src := testutil.NewFakeSource(g1, g2)
	s := NewSampler(src, clk)
	_, _ = s.Sample(context.Background(), []model.MetricID{"cpu"})
	clk.Advance(time.Second)
	snap, _ := s.Sample(context.Background(), []model.MetricID{"cpu"})
	for _, p := range snap.Processes {
		if p.PID == 11 && p.Metric("cpu").Present() {
			t.Fatal("newly appeared process must warm up before a rate exists")
		}
	}
}

func TestSampler_IOPermissionDenied(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_600_000_100, 0))
	st := stat(10, 50, 100, 0, 4096)
	st.IOAvail = model.PermissionDenied
	src := testutil.NewFakeSource([]ports.ProcStat{st}, []ports.ProcStat{st})
	s := NewSampler(src, clk)
	_, _ = s.Sample(context.Background(), []model.MetricID{"disk-rbps"})
	clk.Advance(time.Second)
	snap, _ := s.Sample(context.Background(), []model.MetricID{"disk-rbps"})
	d := snap.Processes[0].Metric("disk-rbps")
	if d.Present() || d.Availability != model.PermissionDenied {
		t.Fatalf("io denied should surface permission_denied, got %q", d.Availability)
	}
}
