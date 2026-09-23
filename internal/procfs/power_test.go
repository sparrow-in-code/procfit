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

	if b := c.Backend(); b != "powercap" {
		t.Fatalf("backend = %q, want powercap", b)
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

	if c.Backend() != "battery" {
		t.Fatalf("backend = %q, want battery", c.Backend())
	}
	if v := procs[0].Metric("power-system"); !v.Present() || v.V < 14.9 || v.V > 15.1 {
		t.Fatalf("power-system = %+v, want ~15W", v)
	}
}

func TestNewPowerSource_SelectionOrder(t *testing.T) {
	// Battery only.
	batRoot := t.TempDir()
	writeSys(t, batRoot, "class/power_supply/BAT0/type", "Battery\n")
	writeSys(t, batRoot, "class/power_supply/BAT0/power_now", "9000000\n")
	if s := NewPowerSource(batRoot); s == nil || s.ID() != "battery" {
		t.Fatalf("battery-only host should select battery, got %v", s)
	}
	// powercap (readable) wins over a present battery.
	bothRoot := t.TempDir()
	powercapTree(t, bothRoot)
	writeSys(t, bothRoot, "class/power_supply/BAT0/type", "Battery\n")
	writeSys(t, bothRoot, "class/power_supply/BAT0/power_now", "9000000\n")
	if s := NewPowerSource(bothRoot); s == nil || s.ID() != "powercap" {
		t.Fatalf("powercap must win over battery, got %v", s)
	}
	// Nothing available.
	if s := NewPowerSource(t.TempDir()); s != nil {
		t.Fatalf("no power source should select nil, got %v", s)
	}
}
