package procfs

import (
	"context"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/model"
)

func writeSys(t *testing.T, root, rel, content string) {
	t.Helper()
	mustWrite(t, root+"/"+rel, content)
}

func powercapTree(t *testing.T, root string) {
	t.Helper()
	writeSys(t, root, "class/powercap/intel-rapl:0/name", "package-0\n")
	writeSys(t, root, "class/powercap/intel-rapl:0/energy_uj", "1000000\n")
	writeSys(t, root, "class/powercap/intel-rapl:0:0/name", "core\n")
	writeSys(t, root, "class/powercap/intel-rapl:0:0/energy_uj", "500000\n")
}

func TestPowerCollector_PowercapWatts(t *testing.T) {
	root := t.TempDir()
	powercapTree(t, root)

	c := NewPowerCollector(root)
	c.window = 100 * time.Millisecond
	c.sleep = func(time.Duration) {
		// +2.0 J on pkg (→20W) and +1.0 J on core (→10W) over 0.1s.
		writeSys(t, root, "class/powercap/intel-rapl:0/energy_uj", "3000000\n")
		writeSys(t, root, "class/powercap/intel-rapl:0:0/energy_uj", "1500000\n")
	}
	procs := []model.Process{{PID: 1}, {PID: 2}}
	c.Collect(context.Background(), procs)

	if b := c.Backends(); len(b) != 1 || b[0] != "powercap" {
		t.Fatalf("backends = %v, want [powercap]", b)
	}
	for _, p := range procs {
		if v := p.Metric("power-pkg"); !v.Present() || v.V < 19.9 || v.V > 20.1 {
			t.Fatalf("power-pkg = %+v, want ~20W", v)
		}
		if v := p.Metric("power-core"); !v.Present() || v.V < 9.9 || v.V > 10.1 {
			t.Fatalf("power-core = %+v, want ~10W", v)
		}
		// A domain the host does not expose renders unavailable, never zero.
		if v := p.Metric("power-dram"); v.Present() {
			t.Fatalf("power-dram must be unavailable, got %+v", v)
		}
	}
}

func TestPowerCollector_CounterWrap(t *testing.T) {
	root := t.TempDir()
	powercapTree(t, root)
	c := NewPowerCollector(root)
	c.window = 100 * time.Millisecond
	c.sleep = func(time.Duration) {
		// Counter wraps (2nd < 1st) -> unavailable, not a negative/huge value.
		writeSys(t, root, "class/powercap/intel-rapl:0/energy_uj", "10\n")
	}
	procs := []model.Process{{PID: 1}}
	c.Collect(context.Background(), procs)
	if v := procs[0].Metric("power-pkg"); v.Present() {
		t.Fatalf("counter wrap must render unavailable, got %+v", v)
	}
}

func TestBatterySource_PowerNow(t *testing.T) {
	root := t.TempDir()
	writeSys(t, root, "class/power_supply/BAT0/type", "Battery\n")
	writeSys(t, root, "class/power_supply/BAT0/power_now", "15000000\n") // 15 W

	c := NewPowerCollector(root)
	c.sleep = func(time.Duration) { t.Fatal("instantaneous battery power needs no window") }
	procs := []model.Process{{PID: 1}}
	c.Collect(context.Background(), procs)

	if b := c.Backends(); len(b) != 1 || b[0] != "battery" {
		t.Fatalf("backends = %v, want [battery]", b)
	}
	if v := procs[0].Metric("power-system"); !v.Present() || v.V < 14.9 || v.V > 15.1 {
		t.Fatalf("power-system = %+v, want ~15W", v)
	}
}

// TestPowerCollector_MergesBackends is the key fix: a laptop with BOTH RAPL and a
// battery must report RAPL's pkg/core AND the battery's whole-system draw — the
// single-backend selection used to hide power-system whenever RAPL existed.
func TestPowerCollector_MergesBackends(t *testing.T) {
	root := t.TempDir()
	powercapTree(t, root)
	writeSys(t, root, "class/power_supply/BAT0/type", "Battery\n")
	writeSys(t, root, "class/power_supply/BAT0/power_now", "12000000\n") // 12 W system

	c := NewPowerCollector(root)
	c.window = 100 * time.Millisecond
	c.sleep = func(time.Duration) {
		writeSys(t, root, "class/powercap/intel-rapl:0/energy_uj", "3000000\n")   // +2J → 20W
		writeSys(t, root, "class/powercap/intel-rapl:0:0/energy_uj", "1500000\n") // +1J → 10W
	}
	procs := []model.Process{{PID: 1}}
	c.Collect(context.Background(), procs)

	if b := c.Backends(); len(b) != 2 || b[0] != "powercap" || b[1] != "battery" {
		t.Fatalf("backends = %v, want [powercap battery]", b)
	}
	if v := procs[0].Metric("power-pkg"); !v.Present() || v.V < 19.9 || v.V > 20.1 {
		t.Fatalf("power-pkg = %+v, want ~20W (RAPL)", v)
	}
	if v := procs[0].Metric("power-system"); !v.Present() || v.V < 11.9 || v.V > 12.1 {
		t.Fatalf("power-system = %+v, want ~12W (battery, not hidden by RAPL)", v)
	}
}

func TestNewPowerSources_Discovery(t *testing.T) {
	// Battery only.
	batRoot := t.TempDir()
	writeSys(t, batRoot, "class/power_supply/BAT0/type", "Battery\n")
	writeSys(t, batRoot, "class/power_supply/BAT0/power_now", "9000000\n")
	if s := NewPowerSources(batRoot); len(s) != 1 || s[0].ID() != "battery" {
		t.Fatalf("battery-only host → %v, want [battery]", s)
	}
	// Both present → both returned, powercap first.
	bothRoot := t.TempDir()
	powercapTree(t, bothRoot)
	writeSys(t, bothRoot, "class/power_supply/BAT0/type", "Battery\n")
	writeSys(t, bothRoot, "class/power_supply/BAT0/power_now", "9000000\n")
	if s := NewPowerSources(bothRoot); len(s) != 2 || s[0].ID() != "powercap" || s[1].ID() != "battery" {
		t.Fatalf("both present → %v, want [powercap battery]", s)
	}
	// Nothing present.
	if s := NewPowerSources(t.TempDir()); len(s) != 0 {
		t.Fatalf("no power source → %v, want empty", s)
	}
}
