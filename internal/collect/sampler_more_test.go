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
