package render

import (
	"testing"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

func TestFormatMetric_Unavailable(t *testing.T) {
	d := metrics.Descriptor{Unit: metrics.UnitPercentOneCPU}
	if got := FormatMetric(d, model.Unavailable[float64](model.WarmingUp, "x")); got != "-" {
		t.Fatalf("warming_up should render '-', got %q", got)
	}
	if got := FormatMetric(d, model.Unavailable[float64](model.PermissionDenied, "x")); got != "?" {
		t.Fatalf("permission_denied should render '?', got %q", got)
	}
	mixed := model.Value[float64]{Availability: model.Available, Quality: "mixed"}
	if got := FormatMetric(d, mixed); got != "mixed" {
		t.Fatalf("mixed should render 'mixed', got %q", got)
	}
}

func TestFormatMetricRaw(t *testing.T) {
	cases := []struct {
		unit metrics.Unit
		val  float64
		want string
	}{
		{metrics.UnitBytes, 40480391168, "40480391168"}, // raw bytes, not 38G
		{metrics.UnitBytesPerSec, 4096, "4096"},
		{metrics.UnitCount, 731, "731"},
		{metrics.UnitPerSec, 1200, "1200"},
		{metrics.UnitInteger, 10, "10"},
		{metrics.UnitPercentOneCPU, 32.0366, "32.0"}, // percent stays 1 decimal, not full float
		{metrics.UnitDuration, 5391.6, "5391"},
	}
	for _, c := range cases {
		d := metrics.Descriptor{Unit: c.unit}
		if got := FormatMetricRaw(d, model.NewValue(c.val, model.Exact, "x")); got != c.want {
			t.Errorf("raw unit %s val %v => %q, want %q", c.unit, c.val, got, c.want)
		}
	}
	// Unavailable still shows a placeholder, never zero.
	d := metrics.Descriptor{Unit: metrics.UnitBytes}
	if got := FormatMetricRaw(d, model.Unavailable[float64](model.WarmingUp, "x")); got != "-" {
		t.Fatalf("raw unavailable => %q, want '-'", got)
	}
}

func TestResolveColumnsMode_RawVsHuman(t *testing.T) {
	reg := metrics.NewDefault()
	row := &query.Row{Kind: query.RowProcess, Label: "x"}
	row.Metrics = map[model.MetricID]model.MetricValue{"rss": model.NewValue(40480391168.0, model.Exact, "t")}
	human, _ := ResolveColumnsMode(reg, []string{"rss"}, true)
	raw, _ := ResolveColumnsMode(reg, []string{"rss"}, false)
	if human[0].Cell(row) != "38G" {
		t.Fatalf("human rss = %q, want 38G", human[0].Cell(row))
	}
	if raw[0].Cell(row) != "40480391168" {
		t.Fatalf("raw rss = %q, want raw bytes", raw[0].Cell(row))
	}
	// Machine value is always raw regardless of mode.
	if human[0].Machine(row) != "40480391168" {
		t.Fatalf("machine rss = %q", human[0].Machine(row))
	}
}

func TestFormatUnits(t *testing.T) {
	cases := []struct {
		unit metrics.Unit
		val  float64
		want string
	}{
		{metrics.UnitBytes, 512, "512"},
		{metrics.UnitBytes, 1536, "1.5K"},
		{metrics.UnitBytes, 4 * 1024 * 1024 * 1024, "4.0G"},
		{metrics.UnitBytes, 40 * 1024 * 1024 * 1024, "40G"},
		{metrics.UnitPercentOneCPU, 2.34, "2.3"},
		{metrics.UnitPerSec, 731, "731"},
		{metrics.UnitPerSec, 1200, "1.2K"},
		{metrics.UnitCount, 2_000_000, "2.0M"},
		{metrics.UnitDuration, 45, "45s"},
		{metrics.UnitDuration, 120, "2m"},
		{metrics.UnitDuration, 7200, "2h"},
		{metrics.UnitDuration, 172800, "2d"},
		{metrics.UnitInteger, 10, "10"},
	}
	for _, c := range cases {
		d := metrics.Descriptor{Unit: c.unit}
		got := FormatMetric(d, model.NewValue(c.val, model.Exact, "x"))
		if got != c.want {
			t.Errorf("unit %s val %v => %q, want %q", c.unit, c.val, got, c.want)
		}
	}
}
